package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// rulesetSyncTimeout 是单次 HEAD 同步校验的硬上限。规则集 URL 一般是
// GitHub raw / CDN，5s 已足够；超时按"同步失败"处理。
const rulesetSyncTimeout = 5 * time.Second

// metaRulesDatBase 是 MetaCubeX/meta-rules-dat 通过 gh-proxy 反代的根地址。
// 预设清单的 URL 全部以这个前缀开头，便于 v1 切换镜像（比如未来切到自建
// jsdelivr）只需要改这一处。
const metaRulesDatBase = "https://gh-proxy.com/https://github.com/MetaCubeX/meta-rules-dat/raw/refs/heads/meta/geo"

// RuleSetHandler 持有 /api/rule-sets/* 端点。
//
// 协作者：
//   - repo  : 规则集 CRUD repo
//   - now   : 时钟（默认 time.Now，方便测试注入）
//   - http  : 用于 sync 时发 HEAD 请求（可 nil；nil 时回退到 http.DefaultClient）
//
// 设计取舍：v1 不真正下载 .mrs 内容（mihomo 客户端会自己拉），sync 只 HEAD
// 一下 URL 验证可达 + 更新 last_synced_at；这样 hub 占用最小、出错面也最小。
type RuleSetHandler struct {
	repo   *storage.RuleSetProviderRepo
	http   *http.Client
	logger *slog.Logger
	now    func() time.Time
}

// NewRuleSetHandler 装配 handler。client / logger / now 都可以是 nil。
func NewRuleSetHandler(repo *storage.RuleSetProviderRepo, client *http.Client, logger *slog.Logger, now func() time.Time) *RuleSetHandler {
	if client == nil {
		client = &http.Client{Timeout: rulesetSyncTimeout}
	}
	if now == nil {
		now = time.Now
	}
	return &RuleSetHandler{repo: repo, http: client, logger: logger, now: now}
}

// List implements GET /api/rule-sets. 支持 ?keyword= 模糊搜索 + 分页。
func (h *RuleSetHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.repo.List(r.Context(), user.ID, storage.RuleSetProviderListOptions{
		Page: page.Page, PageSize: page.PageSize,
		Keyword: r.URL.Query().Get("keyword"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.RuleSetProvider, len(recs))
	for i := range recs {
		items[i] = ruleSetRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.RuleSetProvider]]{
		Data: types.PagedResponse[types.RuleSetProvider]{
			Items: items, Total: total, Page: page.Page, PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Get implements GET /api/rule-sets/{id}.
func (h *RuleSetHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.RuleSetProvider]{
		Data: ruleSetRecordToDTO(rec), RequestID: traceID,
	})
}

// Create implements POST /api/rule-sets.
func (h *RuleSetHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateRuleSetRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if !isValidRuleSetBehavior(req.Behavior) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid behavior", nil, traceID)
		return
	}
	if !isValidRuleSetFormat(req.Format) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid format", nil, traceID)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "url required", nil, traceID)
		return
	}
	if req.IntervalSeconds <= 0 {
		req.IntervalSeconds = 86400
	}
	rec := storage.RuleSetProviderRecord{
		ID:              util.UUIDv7(),
		UserID:          user.ID,
		Name:            req.Name,
		Behavior:        string(req.Behavior),
		Format:          string(req.Format),
		URL:             req.URL,
		IntervalSeconds: req.IntervalSeconds,
		Enabled:         req.Enabled,
	}
	created, err := h.repo.Create(r.Context(), rec)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.RuleSetProvider]{
		Data: ruleSetRecordToDTO(created), RequestID: traceID,
	})
}

// Update implements PUT/PATCH /api/rule-sets/{id}.
func (h *RuleSetHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateRuleSetRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Behavior != "" && !isValidRuleSetBehavior(req.Behavior) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid behavior", nil, traceID)
		return
	}
	if req.Format != "" && !isValidRuleSetFormat(req.Format) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid format", nil, traceID)
		return
	}
	upd := storage.RuleSetProviderUpdate{
		Name:            req.Name,
		Behavior:        string(req.Behavior),
		Format:          string(req.Format),
		URL:             req.URL,
		IntervalSeconds: req.IntervalSeconds,
		Enabled:         req.Enabled,
	}
	if err := h.repo.Update(r.Context(), id, user.ID, upd); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	updated, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.RuleSetProvider]{
		Data: ruleSetRecordToDTO(updated), RequestID: traceID,
	})
}

// Delete implements DELETE /api/rule-sets/{id}.
func (h *RuleSetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// Sync implements POST /api/rule-sets/{id}/sync. 触发立即同步：HEAD 远程 URL
// 校验可达；更新 last_synced_at + status。失败时 status="error"，错误消息存
// last_sync_error；但 endpoint 自身仍返回 200（同步失败不是 HTTP 错误）。
func (h *RuleSetHandler) Sync(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	status, syncErr := h.probeURL(r.Context(), rec.URL)
	if updateErr := h.repo.UpdateSyncStatus(r.Context(), id, user.ID, status, syncErr); updateErr != nil {
		h.respondStorageErr(w, traceID, updateErr)
		return
	}
	updated, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.RuleSetProvider]{
		Data: ruleSetRecordToDTO(updated), RequestID: traceID,
	})
}

// Presets implements GET /api/rule-sets/presets. 返回内置规则集清单，不进 db。
func (h *RuleSetHandler) Presets(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.RuleSetPreset]{
		Data: builtInRuleSetPresets, RequestID: traceID,
	})
}

// SyncAll 是后台 cron 调度器调用的入口（每日 03:00 UTC）。它遍历所有
// enabled=1 的规则集，逐个 HEAD 校验。失败的条目把错误记到 last_sync_error，
// 但循环不会中断。
//
// 调用方在 cmd/server 里以 goroutine 启动；ctx 控制总耗时。
func (h *RuleSetHandler) SyncAll(ctx context.Context, userID string) (int, int) {
	if h == nil || h.repo == nil {
		return 0, 0
	}
	recs, err := h.repo.ListEnabled(ctx, userID)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("rule set sync-all: list enabled failed",
				slog.String("err", err.Error()), slog.String("user_id", userID))
		}
		return 0, 0
	}
	ok, fail := 0, 0
	for _, rec := range recs {
		select {
		case <-ctx.Done():
			return ok, fail
		default:
		}
		status, syncErr := h.probeURL(ctx, rec.URL)
		if err := h.repo.UpdateSyncStatus(ctx, rec.ID, userID, status, syncErr); err != nil {
			fail++
			continue
		}
		if status == "ok" {
			ok++
		} else {
			fail++
		}
	}
	return ok, fail
}

// probeURL 发一个带超时的 HEAD 请求。2xx / 3xx 视为成功（部分 CDN 对 HEAD 回
// 30x 到 raw 上）；其他都按失败处理。
func (h *RuleSetHandler) probeURL(parent context.Context, url string) (status string, msg string) {
	ctx, cancel := context.WithTimeout(parent, rulesetSyncTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "error", "build request: " + err.Error()
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return "error", err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "ok", ""
	}
	return "error", resp.Status
}

// respondStorageErr 把规则集 repo 的错误投影成 HTTP 错误码。
func (h *RuleSetHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrRuleSetProviderNotFound):
		util.RespondError(w, types.ErrNotFoundRule, "rule set not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("rule set handler db failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalDatabase, "internal error", nil, traceID)
	}
}

// ruleSetRecordToDTO 把存储记录翻译成对外 DTO。
func ruleSetRecordToDTO(rec *storage.RuleSetProviderRecord) types.RuleSetProvider {
	return types.RuleSetProvider{
		ID:              rec.ID,
		UserID:          rec.UserID,
		Name:            rec.Name,
		Behavior:        types.RuleSetBehavior(rec.Behavior),
		Format:          types.RuleSetFormat(rec.Format),
		URL:             rec.URL,
		IntervalSeconds: rec.IntervalSeconds,
		Enabled:         rec.Enabled,
		LastSyncedAt:    rec.LastSyncedAt,
		LastSyncStatus:  rec.LastSyncStatus,
		LastSyncError:   rec.LastSyncError,
		CreatedAt:       rec.CreatedAt,
		UpdatedAt:       rec.UpdatedAt,
	}
}

func isValidRuleSetBehavior(b types.RuleSetBehavior) bool {
	switch b {
	case types.RuleSetBehaviorDomain, types.RuleSetBehaviorIPCIDR, types.RuleSetBehaviorClassical:
		return true
	}
	return false
}

func isValidRuleSetFormat(f types.RuleSetFormat) bool {
	switch f {
	case types.RuleSetFormatYAML, types.RuleSetFormatText, types.RuleSetFormatMRS:
		return true
	}
	return false
}

// builtInRuleSetPresets 是 GET /api/rule-sets/presets 返回的静态清单。
// 全部走 MetaCubeX/meta-rules-dat 的 meta 分支 + gh-proxy 反代镜像，所以国
// 内 mihomo 客户端无需翻墙也能拉到 .mrs 二进制。
//
// 分类：
//   - region : 国家 / 地区 GEO 数据库（cn / geolocation-!cn / private）
//   - app    : 单应用域名集合（openai / google / netflix...）
//   - block  : 广告 / 追踪 / 钓鱼拦截
var builtInRuleSetPresets = []types.RuleSetPreset{
	// ---------- 地区 ----------
	{
		ID: "cn-domain", Name: "中国大陆域名", Emoji: "🇨🇳", Category: "region",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/cn.mrs", IntervalSeconds: 86400,
		Description: "中国大陆常见域名（含 *.cn TLD 与常见站点）。",
	},
	{
		ID: "cn-ip", Name: "中国大陆 IP", Emoji: "🇨🇳", Category: "region",
		Behavior: types.RuleSetBehaviorIPCIDR, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geoip/cn.mrs", IntervalSeconds: 86400,
		Description: "中国大陆 IPv4 / IPv6 段。",
	},
	{
		ID: "geolocation-!cn", Name: "国外网站", Emoji: "🌍", Category: "region",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/geolocation-!cn.mrs", IntervalSeconds: 86400,
		Description: "国外网站聚合（CN 之外）。",
	},
	{
		ID: "private", Name: "局域网", Emoji: "🏠", Category: "region",
		Behavior: types.RuleSetBehaviorIPCIDR, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geoip/private.mrs", IntervalSeconds: 86400,
		Description: "RFC1918 + 链路本地 + 多播段。",
	},
	// ---------- 应用 ----------
	{
		ID: "openai", Name: "OpenAI", Emoji: "🤖", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/openai.mrs", IntervalSeconds: 86400,
		Description: "ChatGPT / OpenAI API 域名集合。",
	},
	{
		ID: "anthropic", Name: "Anthropic", Emoji: "🤖", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/anthropic.mrs", IntervalSeconds: 86400,
		Description: "Claude / Anthropic API 域名集合。",
	},
	{
		ID: "google", Name: "Google", Emoji: "📢", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/google.mrs", IntervalSeconds: 86400,
		Description: "Google 全家桶域名（含 google.com / gstatic.com 等）。",
	},
	{
		ID: "github", Name: "GitHub", Emoji: "🐙", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/github.mrs", IntervalSeconds: 86400,
		Description: "GitHub 域名集合。",
	},
	{
		ID: "youtube", Name: "YouTube", Emoji: "📺", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/youtube.mrs", IntervalSeconds: 86400,
		Description: "YouTube + 媒体子域名集合。",
	},
	{
		ID: "netflix", Name: "Netflix", Emoji: "📺", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/netflix.mrs", IntervalSeconds: 86400,
		Description: "Netflix 流媒体域名集合。",
	},
	{
		ID: "disney", Name: "Disney+", Emoji: "📺", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/disney.mrs", IntervalSeconds: 86400,
		Description: "Disney+ 流媒体域名集合。",
	},
	{
		ID: "spotify", Name: "Spotify", Emoji: "🎵", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/spotify.mrs", IntervalSeconds: 86400,
		Description: "Spotify 音乐流媒体域名集合。",
	},
	{
		ID: "telegram", Name: "Telegram", Emoji: "✈️", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/telegram.mrs", IntervalSeconds: 86400,
		Description: "Telegram 域名集合（含 telegram.org / t.me）。",
	},
	{
		ID: "microsoft", Name: "微软", Emoji: "Ⓜ️", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/microsoft.mrs", IntervalSeconds: 86400,
		Description: "Microsoft 全家桶（Office / Azure / Bing 等）。",
	},
	{
		ID: "apple", Name: "苹果", Emoji: "🍎", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/apple.mrs", IntervalSeconds: 86400,
		Description: "Apple 全家桶（iCloud / App Store / Apple Music 等）。",
	},
	{
		ID: "steam", Name: "Steam", Emoji: "🎮", Category: "app",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/steam.mrs", IntervalSeconds: 86400,
		Description: "Steam 平台域名集合。",
	},
	// ---------- 拦截 ----------
	{
		ID: "category-ads-all", Name: "广告聚合", Emoji: "🚫", Category: "block",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/category-ads-all.mrs", IntervalSeconds: 86400,
		Description: "聚合广告域名集合（含主流 ad-network）。",
	},
	{
		ID: "category-tracker", Name: "追踪保护", Emoji: "🛡️", Category: "block",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/tracker.mrs", IntervalSeconds: 86400,
		Description: "用户追踪 / 分析平台域名集合。",
	},
	{
		ID: "phishing", Name: "钓鱼网站", Emoji: "🎣", Category: "block",
		Behavior: types.RuleSetBehaviorDomain, Format: types.RuleSetFormatMRS,
		URL: metaRulesDatBase + "/geosite/category-phishing.mrs", IntervalSeconds: 86400,
		Description: "钓鱼 / 诈骗网站域名集合。",
	},
}
