package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// ProxyGroupHandler 持有 /api/proxy-groups/* 端点。CRUD + Reorder + Presets。
type ProxyGroupHandler struct {
	repo   *storage.ProxyGroupRepo
	logger *slog.Logger
}

// NewProxyGroupHandler 装配 handler。logger 可以是 nil（错误打日志会被吞掉）。
func NewProxyGroupHandler(repo *storage.ProxyGroupRepo, logger *slog.Logger) *ProxyGroupHandler {
	return &ProxyGroupHandler{repo: repo, logger: logger}
}

// List implements GET /api/proxy-groups. 默认按 sort_order ASC 返回。
// 支持 ?type= / ?keyword= 过滤 + 分页。
func (h *ProxyGroupHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.repo.List(r.Context(), user.ID, storage.ProxyGroupListOptions{
		Page: page.Page, PageSize: page.PageSize,
		Type:    r.URL.Query().Get("type"),
		Keyword: r.URL.Query().Get("keyword"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.ProxyGroupCategory, len(recs))
	for i := range recs {
		items[i] = proxyGroupRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.ProxyGroupCategory]]{
		Data: types.PagedResponse[types.ProxyGroupCategory]{
			Items: items, Total: total, Page: page.Page, PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Get implements GET /api/proxy-groups/{id}.
func (h *ProxyGroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.ProxyGroupCategory]{
		Data: proxyGroupRecordToDTO(rec), RequestID: traceID,
	})
}

// Create implements POST /api/proxy-groups.
func (h *ProxyGroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateProxyGroupRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if !isValidProxyGroupType(req.Type) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid type", nil, traceID)
		return
	}
	rec := storage.ProxyGroupCategoryRecord{
		ID:            util.UUIDv7(),
		UserID:        user.ID,
		Name:          req.Name,
		Type:          string(req.Type),
		Icon:          req.Icon,
		SortOrder:     req.SortOrder,
		TestURL:       req.TestURL,
		TestInterval:  req.TestInterval,
		Filter:        req.Filter,
		IncludeAll:    req.IncludeAll,
		MemberProxies: marshalStringSlice(req.MemberProxies),
		MemberGroups:  marshalStringSlice(req.MemberGroups),
	}
	created, err := h.repo.Create(r.Context(), rec)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.ProxyGroupCategory]{
		Data: proxyGroupRecordToDTO(created), RequestID: traceID,
	})
}

// Update implements PUT/PATCH /api/proxy-groups/{id}.
func (h *ProxyGroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateProxyGroupRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Type != "" && !isValidProxyGroupType(req.Type) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid type", nil, traceID)
		return
	}
	upd := storage.ProxyGroupUpdate{
		Name:         req.Name,
		Type:         string(req.Type),
		Icon:         req.Icon,
		SortOrder:    req.SortOrder,
		TestURL:      req.TestURL,
		TestInterval: req.TestInterval,
		Filter:       req.Filter,
		IncludeAll:   req.IncludeAll,
	}
	if req.MemberProxies != nil {
		json := marshalStringSlice(*req.MemberProxies)
		upd.MemberProxies = &json
	}
	if req.MemberGroups != nil {
		json := marshalStringSlice(*req.MemberGroups)
		upd.MemberGroups = &json
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
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.ProxyGroupCategory]{
		Data: proxyGroupRecordToDTO(updated), RequestID: traceID,
	})
}

// Delete implements DELETE /api/proxy-groups/{id}.
func (h *ProxyGroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// Reorder implements POST /api/proxy-groups/reorder.
//
// Body 是一个有序 id 列表 `{ "ids": ["..."] }`；每个 id 的位置即新的 sort_order
// （以 100 为步长展开，方便后续手工插入）。未在 body 里出现的组保持原 sort。
func (h *ProxyGroupHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.ProxyGroupReorderRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if len(req.IDs) == 0 {
		util.RespondError(w, types.ErrValidationRequiredField, "ids required", nil, traceID)
		return
	}
	entries := make([]storage.ProxyGroupReorderEntry, len(req.IDs))
	for i, id := range req.IDs {
		entries[i] = storage.ProxyGroupReorderEntry{
			ID: id, SortOrder: int32((i + 1) * 100),
		}
	}
	updated, err := h.repo.Reorder(r.Context(), user.ID, entries)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]int]{
		Data: map[string]int{"updated": updated}, RequestID: traceID,
	})
}

// Presets implements GET /api/proxy-groups/presets. 返回内置代理组清单。
func (h *ProxyGroupHandler) Presets(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.ProxyGroupPreset]{
		Data: builtInProxyGroupPresets, RequestID: traceID,
	})
}

// respondStorageErr 投影 proxy_group repo 的错误。
func (h *ProxyGroupHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrProxyGroupNotFound):
		util.RespondError(w, types.ErrNotFoundProxyGroup, "proxy group not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("proxy group handler db failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalDatabase, "internal error", nil, traceID)
	}
}

// proxyGroupRecordToDTO 把存储记录翻译成对外 DTO，并解析 JSON member 字段。
func proxyGroupRecordToDTO(rec *storage.ProxyGroupCategoryRecord) types.ProxyGroupCategory {
	return types.ProxyGroupCategory{
		ID:            rec.ID,
		UserID:        rec.UserID,
		Name:          rec.Name,
		Type:          types.ProxyGroupType(rec.Type),
		Icon:          rec.Icon,
		SortOrder:     rec.SortOrder,
		TestURL:       rec.TestURL,
		TestInterval:  rec.TestInterval,
		Filter:        rec.Filter,
		IncludeAll:    rec.IncludeAll,
		MemberProxies: unmarshalStringSlice(rec.MemberProxies),
		MemberGroups:  unmarshalStringSlice(rec.MemberGroups),
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
}

func isValidProxyGroupType(t types.ProxyGroupType) bool {
	switch t {
	case types.ProxyGroupSelect, types.ProxyGroupURLTest, types.ProxyGroupFallback,
		types.ProxyGroupLoadBalance, types.ProxyGroupRelay:
		return true
	}
	return false
}

// marshalStringSlice 把 []string 序列化成 JSON 文本（空切片 → "[]"）。
// nil 切片也产出 "[]"，便于存储层语义统一。
func marshalStringSlice(s []string) string {
	if s == nil {
		s = []string{}
	}
	buf, err := json.Marshal(s)
	if err != nil {
		// json.Marshal 对 []string 不会失败；防御性返回空数组。
		return "[]"
	}
	return string(buf)
}

// unmarshalStringSlice 把 JSON 文本解析回 []string。空 / 无效字符串 → []string{}。
// DTO 层保证 nil-safety —— 前端拿到的总是 [] 而非 null。
func unmarshalStringSlice(raw string) []string {
	out := []string{}
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

// builtInProxyGroupPresets 是 GET /api/proxy-groups/presets 返回的静态清单。
//
// 排版顺序：选择器 → 地区组（HK/JP/US/SG/TW/KR）→ 应用组（AI/流媒体/Google/...）
// → 全球出口（直连 / 拦截）→ 漏网之鱼。每个预设都是无 user_id 的"种子"，
// 前端把用户挑选的预设转成具体记录写到 db。
var builtInProxyGroupPresets = []types.ProxyGroupPreset{
	{
		ID: "node-select", Name: "🚀 节点选择", Type: types.ProxyGroupSelect,
		Icon: "🚀", IncludeAll: true,
		Description: "用户主选组：默认包含订阅里的所有节点。",
	},
	{
		ID: "auto-fastest", Name: "♻️ 自动选择", Type: types.ProxyGroupURLTest,
		Icon: "♻️", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Description: "url-test：自动挑延迟最低的节点。",
	},
	{
		ID: "failover", Name: "🔄 故障转移", Type: types.ProxyGroupFallback,
		Icon: "🔄", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Description: "fallback：按顺序尝试节点，第一个可用即使用。",
	},
	{
		ID: "load-balance", Name: "🔮 负载均衡", Type: types.ProxyGroupLoadBalance,
		Icon: "🔮", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Description: "load-balance：将流量分散到多个节点。",
	},
	{
		ID: "region-hk", Name: "🇭🇰 香港节点", Type: types.ProxyGroupURLTest,
		Icon: "🇭🇰", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Filter: "(?i)香港|HK|Hong",
		Description: "正则匹配香港节点，url-test 自动选优。",
	},
	{
		ID: "region-jp", Name: "🇯🇵 日本节点", Type: types.ProxyGroupURLTest,
		Icon: "🇯🇵", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Filter: "(?i)日本|JP|Japan|东京|大阪",
		Description: "正则匹配日本节点。",
	},
	{
		ID: "region-us", Name: "🇺🇸 美国节点", Type: types.ProxyGroupURLTest,
		Icon: "🇺🇸", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Filter: "(?i)美国|US|United States|洛杉矶|纽约",
		Description: "正则匹配美国节点。",
	},
	{
		ID: "region-sg", Name: "🇸🇬 新加坡节点", Type: types.ProxyGroupURLTest,
		Icon: "🇸🇬", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Filter: "(?i)新加坡|SG|狮城|Singapore",
		Description: "正则匹配新加坡节点。",
	},
	{
		ID: "region-tw", Name: "🇹🇼 台湾节点", Type: types.ProxyGroupURLTest,
		Icon: "🇹🇼", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Filter: "(?i)台湾|TW|Taiwan|台北",
		Description: "正则匹配台湾节点。",
	},
	{
		ID: "region-kr", Name: "🇰🇷 韩国节点", Type: types.ProxyGroupURLTest,
		Icon: "🇰🇷", IncludeAll: true,
		TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
		Filter: "(?i)韩国|KR|Korea|首尔",
		Description: "正则匹配韩国节点。",
	},
	{
		ID: "app-ai", Name: "🤖 AI 服务", Type: types.ProxyGroupSelect,
		Icon: "🤖",
		MemberGroups: []string{"node-select", "region-us", "region-jp"},
		Description: "OpenAI / Anthropic / Gemini 等 AI 域名出口，默认建议走非中国大陆地区。",
	},
	{
		ID: "app-streaming", Name: "📺 流媒体", Type: types.ProxyGroupSelect,
		Icon: "📺",
		MemberGroups: []string{"node-select", "region-hk", "region-sg", "region-us"},
		Description: "Netflix / Disney+ / YouTube 等流媒体出口。",
	},
	{
		ID: "app-google", Name: "📢 Google", Type: types.ProxyGroupSelect,
		Icon: "📢",
		MemberGroups: []string{"node-select", "auto-fastest"},
		Description: "Google 全家桶出口。",
	},
	{
		ID: "app-microsoft", Name: "Ⓜ️ 微软", Type: types.ProxyGroupSelect,
		Icon: "Ⓜ️",
		MemberGroups:  []string{"node-select"},
		MemberProxies: []string{"DIRECT"},
		Description:   "微软全家桶出口（默认建议直连，国内访问快）。",
	},
	{
		ID: "app-apple", Name: "🍎 苹果", Type: types.ProxyGroupSelect,
		Icon: "🍎",
		MemberGroups:  []string{"node-select"},
		MemberProxies: []string{"DIRECT"},
		Description:   "苹果全家桶出口。",
	},
	{
		ID: "app-telegram", Name: "✈️ Telegram", Type: types.ProxyGroupSelect,
		Icon: "✈️",
		MemberGroups: []string{"node-select", "region-sg"},
		Description: "Telegram 出口。",
	},
	{
		ID: "app-gaming", Name: "🎮 游戏", Type: types.ProxyGroupSelect,
		Icon: "🎮",
		MemberGroups:  []string{"node-select"},
		MemberProxies: []string{"DIRECT"},
		Description:   "Steam / Epic / PlayStation 等游戏平台出口。",
	},
	{
		ID: "global-direct", Name: "🎯 全球直连", Type: types.ProxyGroupSelect,
		Icon: "🎯",
		MemberProxies: []string{"DIRECT"},
		Description: "直连出口（常用于国内业务）。",
	},
	{
		ID: "global-block", Name: "🛑 全球拦截", Type: types.ProxyGroupSelect,
		Icon: "🛑",
		MemberProxies: []string{"REJECT", "DIRECT"},
		Description:   "拦截出口（广告 / 追踪 / 钓鱼）。",
	},
	{
		ID: "fish", Name: "🐟 漏网之鱼", Type: types.ProxyGroupSelect,
		Icon: "🐟",
		MemberGroups:  []string{"node-select"},
		MemberProxies: []string{"DIRECT"},
		Description:   "MATCH 兜底出口：未命中任何规则的流量走这里。",
	},
}
