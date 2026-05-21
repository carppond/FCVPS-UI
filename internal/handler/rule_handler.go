package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// builtInRuleTemplates is the static catalog returned by
// GET /api/rules/templates. Per Tech Lead §1.5 these are not persisted —
// the frontend simply applies a chosen template's content into the form.
//
// v1.1 起扩充到 18 个，按 Category 分组（region / app / block / common），
// 每个模板的 Content 都用 mihomo / Clash 风格的 rules 文本（DOMAIN-SUFFIX /
// GEOIP / RULE-SET），引用的 RULE-SET 名字与 GET /api/rule-sets/presets
// 提供的预设 id 对齐 —— 用户启用一个模板时，前端可以自动建议引入对应规则集。
//
// 设计取舍：模板里写的是规则**片段**，不强制 MATCH 兜底，避免和"漏网之鱼"
// 模板冲突。最终装配 Clash YAML 时由 substore.ApplyToYAML 做 prepend /
// append 合并；同时也要求用户至少启用一个 FINAL 兜底模板（fallback-fish）。
var builtInRuleTemplates = []types.RuleTemplate{
	// ----------------------------------------------------------------------
	// region: 地区分组（每个含 RULE-SET 引用 + GEOIP 兜底）
	// ----------------------------------------------------------------------
	{
		ID: "region-hk", Name: "香港节点", Emoji: "🇭🇰", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "hk"},
		Description: "把命中香港 GEOIP 的流量打到 🇭🇰 香港节点 组。",
		Content: `GEOIP,HK,🇭🇰 香港节点
`,
	},
	{
		ID: "region-jp", Name: "日本节点", Emoji: "🇯🇵", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "jp"},
		Description: "把命中日本 GEOIP 的流量打到 🇯🇵 日本节点 组。",
		Content: `GEOIP,JP,🇯🇵 日本节点
`,
	},
	{
		ID: "region-us", Name: "美国节点", Emoji: "🇺🇸", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "america", "us"},
		Description: "把命中美国 GEOIP 的流量打到 🇺🇸 美国节点 组。",
		Content: `GEOIP,US,🇺🇸 美国节点
`,
	},
	{
		ID: "region-sg", Name: "新加坡节点", Emoji: "🇸🇬", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "sg"},
		Description: "把命中新加坡 GEOIP 的流量打到 🇸🇬 新加坡节点 组。",
		Content: `GEOIP,SG,🇸🇬 新加坡节点
`,
	},
	{
		ID: "region-tw", Name: "台湾节点", Emoji: "🇹🇼", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "tw"},
		Description: "把命中台湾 GEOIP 的流量打到 🇹🇼 台湾节点 组。",
		Content: `GEOIP,TW,🇹🇼 台湾节点
`,
	},
	{
		ID: "region-kr", Name: "韩国节点", Emoji: "🇰🇷", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "kr"},
		Description: "把命中韩国 GEOIP 的流量打到 🇰🇷 韩国节点 组。",
		Content: `GEOIP,KR,🇰🇷 韩国节点
`,
	},
	// ----------------------------------------------------------------------
	// app: 应用分组（DOMAIN-SUFFIX / RULE-SET 引用）
	// ----------------------------------------------------------------------
	{
		ID: "app-ai", Name: "AI 服务", Emoji: "🤖", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "ai", "openai", "anthropic"},
		Description: "OpenAI / Anthropic / Gemini / Copilot 域名走 🤖 AI 服务 组。",
		Content: `DOMAIN-SUFFIX,openai.com,🤖 AI 服务
DOMAIN-SUFFIX,anthropic.com,🤖 AI 服务
DOMAIN-SUFFIX,claude.ai,🤖 AI 服务
DOMAIN-SUFFIX,googleai.com,🤖 AI 服务
DOMAIN-SUFFIX,gemini.google.com,🤖 AI 服务
DOMAIN-SUFFIX,githubcopilot.com,🤖 AI 服务
RULE-SET,openai,🤖 AI 服务
RULE-SET,anthropic,🤖 AI 服务
`,
	},
	{
		ID: "app-streaming", Name: "流媒体", Emoji: "📺", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "streaming", "netflix", "youtube"},
		Description: "Netflix / Disney+ / YouTube / Spotify 域名走 📺 流媒体 组。",
		Content: `DOMAIN-SUFFIX,netflix.com,📺 流媒体
DOMAIN-SUFFIX,nflxext.com,📺 流媒体
DOMAIN-SUFFIX,disneyplus.com,📺 流媒体
DOMAIN-SUFFIX,youtube.com,📺 流媒体
DOMAIN-SUFFIX,googlevideo.com,📺 流媒体
DOMAIN-SUFFIX,spotify.com,📺 流媒体
RULE-SET,netflix,📺 流媒体
RULE-SET,disney,📺 流媒体
RULE-SET,youtube,📺 流媒体
RULE-SET,spotify,📺 流媒体
`,
	},
	{
		ID: "app-google", Name: "Google 服务", Emoji: "📢", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "google"},
		Description: "Google 全家桶域名走 📢 Google 组。",
		Content: `DOMAIN-SUFFIX,google.com,📢 Google
DOMAIN-SUFFIX,gstatic.com,📢 Google
DOMAIN-SUFFIX,googleapis.com,📢 Google
DOMAIN-SUFFIX,googleusercontent.com,📢 Google
RULE-SET,google,📢 Google
`,
	},
	{
		ID: "app-microsoft", Name: "微软服务", Emoji: "Ⓜ️", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "microsoft", "azure"},
		Description: "Office / Azure / Bing / Github 域名走 Ⓜ️ 微软 组。",
		Content: `DOMAIN-SUFFIX,microsoft.com,Ⓜ️ 微软
DOMAIN-SUFFIX,office.com,Ⓜ️ 微软
DOMAIN-SUFFIX,office365.com,Ⓜ️ 微软
DOMAIN-SUFFIX,azure.com,Ⓜ️ 微软
DOMAIN-SUFFIX,bing.com,Ⓜ️ 微软
RULE-SET,microsoft,Ⓜ️ 微软
`,
	},
	{
		ID: "app-apple", Name: "苹果服务", Emoji: "🍎", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "apple", "icloud"},
		Description: "iCloud / App Store / Apple Music 等域名走 🍎 苹果 组。",
		Content: `DOMAIN-SUFFIX,apple.com,🍎 苹果
DOMAIN-SUFFIX,icloud.com,🍎 苹果
DOMAIN-SUFFIX,mzstatic.com,🍎 苹果
DOMAIN-SUFFIX,itunes.apple.com,🍎 苹果
RULE-SET,apple,🍎 苹果
`,
	},
	{
		ID: "app-telegram", Name: "Telegram", Emoji: "✈️", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "telegram", "im"},
		Description: "Telegram 域名走 ✈️ Telegram 组。",
		Content: `DOMAIN-SUFFIX,telegram.org,✈️ Telegram
DOMAIN-SUFFIX,t.me,✈️ Telegram
DOMAIN-SUFFIX,telegram.me,✈️ Telegram
DOMAIN-SUFFIX,telesco.pe,✈️ Telegram
RULE-SET,telegram,✈️ Telegram
`,
	},
	{
		ID: "app-gaming", Name: "游戏平台", Emoji: "🎮", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "gaming", "steam", "epic"},
		Description: "Steam / Epic / PlayStation / Xbox 域名走 🎮 游戏 组。",
		Content: `DOMAIN-SUFFIX,steampowered.com,🎮 游戏
DOMAIN-SUFFIX,steamcommunity.com,🎮 游戏
DOMAIN-SUFFIX,epicgames.com,🎮 游戏
DOMAIN-SUFFIX,playstation.net,🎮 游戏
DOMAIN-SUFFIX,xboxlive.com,🎮 游戏
RULE-SET,steam,🎮 游戏
`,
	},
	// ----------------------------------------------------------------------
	// block: 拦截类
	// ----------------------------------------------------------------------
	{
		ID: "block-ads", Name: "广告拦截", Emoji: "🚫", Category: "block",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"block", "ads"},
		Description: "聚合广告 + 常见 ad-network 域名 REJECT，命中即拦截。",
		Content: `DOMAIN-KEYWORD,googlesyndication,REJECT
DOMAIN-KEYWORD,doubleclick,REJECT
DOMAIN-SUFFIX,googleadservices.com,REJECT
RULE-SET,category-ads-all,REJECT
`,
	},
	{
		ID: "block-privacy", Name: "隐私保护", Emoji: "🛡️", Category: "block",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"block", "privacy", "tracker"},
		Description: "追踪 / 分析 / 钓鱼网站域名 REJECT。",
		Content: `RULE-SET,category-tracker,REJECT
RULE-SET,phishing,REJECT
`,
	},
	// ----------------------------------------------------------------------
	// common: 通用 / 兜底
	// ----------------------------------------------------------------------
	{
		ID: "cn-direct-foreign-proxy", Name: "国内直连 + 国外代理", Emoji: "🇨🇳", Category: "common",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"common", "default", "recommended"},
		Description: "中国大陆 IP / 域名直连，其余流量走代理。推荐 default 选项。",
		Content: `DOMAIN-SUFFIX,cn,DIRECT
DOMAIN-KEYWORD,baidu,DIRECT
DOMAIN-KEYWORD,taobao,DIRECT
DOMAIN-KEYWORD,jd,DIRECT
DOMAIN-KEYWORD,qq,DIRECT
DOMAIN-KEYWORD,weibo,DIRECT
RULE-SET,cn-domain,DIRECT
GEOIP,CN,DIRECT
RULE-SET,geolocation-!cn,🚀 节点选择
MATCH,🚀 节点选择
`,
	},
	{
		ID: "global-proxy", Name: "全局代理", Emoji: "🌍", Category: "common",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeReplace,
		Tags:        []string{"common", "global"},
		Description: "所有流量都走代理出口，仅本地 / 私网保留直连。",
		Content: `DOMAIN-SUFFIX,localhost,DIRECT
IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
MATCH,🚀 节点选择
`,
	},
	{
		ID: "fallback-fish", Name: "漏网之鱼", Emoji: "🐟", Category: "common",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"common", "fallback", "final"},
		Description: "MATCH 兜底规则：未命中任何规则的流量走 🐟 漏网之鱼 组。",
		Content: `MATCH,🐟 漏网之鱼
`,
	},
}

// RuleHandler hosts /api/rules/* endpoints. The handler couples three small
// collaborators:
//
//   - rules   : CRUD repo
//   - subs    : subscription repo (preview endpoint needs the cached YAML)
//   - nodes   : node fetcher (re-renders Clash YAML before injecting rules)
//
// nodes may be nil — the preview endpoint then falls back to a minimal "no
// proxies" template so the UI can still see a valid rule pipeline.
type RuleHandler struct {
	rules    *storage.CustomRuleRepo
	subs     *storage.SubscriptionRepo
	nodeRepo substore.NodeFetcher
	logger   *slog.Logger
}

// NewRuleHandler wires the handler. subs / nodeRepo may be nil; preview
// degrades gracefully in that case.
func NewRuleHandler(rules *storage.CustomRuleRepo, subs *storage.SubscriptionRepo, nodes substore.NodeFetcher, logger *slog.Logger) *RuleHandler {
	return &RuleHandler{rules: rules, subs: subs, nodeRepo: nodes, logger: logger}
}

// List implements GET /api/rules. Optional ?type=<dns|rules|rule-providers>
// + ?keyword=<name>. Returns the full enabled+disabled set; the frontend
// renders the list with a toggle column.
func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.rules.List(r.Context(), user.ID, storage.CustomRuleListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Type:     r.URL.Query().Get("type"),
		Keyword:  r.URL.Query().Get("keyword"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.CustomRule, len(recs))
	for i := range recs {
		items[i] = ruleRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.CustomRule]]{
		Data: types.PagedResponse[types.CustomRule]{
			Items: items, Total: total, Page: page.Page, PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Create implements POST /api/rules.
func (h *RuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateRuleRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if !isValidRuleType(req.Type) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid type", nil, traceID)
		return
	}
	if !isValidRuleMode(req.Mode) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid mode", nil, traceID)
		return
	}
	if err := validateRuleContent(req.Type, req.Content); err != nil {
		util.RespondError(w, types.ErrValidationYAMLParse, err.Error(), nil, traceID)
		return
	}
	rec := storage.CustomRuleRecord{
		ID:      util.UUIDv7(),
		UserID:  user.ID,
		Name:    req.Name,
		Type:    string(req.Type),
		Mode:    string(req.Mode),
		Content: req.Content,
		Enabled: req.Enabled,
		Sort:    req.Sort,
	}
	if rec.Sort == 0 {
		rec.Sort = h.nextSort(r.Context(), user.ID)
	}
	created, err := h.rules.Create(r.Context(), rec)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.CustomRule]{
		Data: ruleRecordToDTO(created), RequestID: traceID,
	})
}

// Get implements GET /api/rules/{id}. Cross-user requests resolve to 404.
func (h *RuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.rules.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.CustomRule]{
		Data: ruleRecordToDTO(rec), RequestID: traceID,
	})
}

// Update implements PUT/PATCH /api/rules/{id}. Empty string fields are
// preserved; Enabled requires a non-nil pointer to be applied.
func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateRuleRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Mode != "" && !isValidRuleMode(req.Mode) {
		util.RespondError(w, types.ErrValidationSchemaMismatch, "invalid mode", nil, traceID)
		return
	}
	if req.Content != "" {
		existing, err := h.rules.GetByID(r.Context(), id, user.ID)
		if err != nil {
			h.respondStorageErr(w, traceID, err)
			return
		}
		if err := validateRuleContent(types.RuleType(existing.Type), req.Content); err != nil {
			util.RespondError(w, types.ErrValidationYAMLParse, err.Error(), nil, traceID)
			return
		}
	}
	rec := storage.CustomRuleRecord{
		ID: id, UserID: user.ID,
		Name: req.Name, Mode: string(req.Mode), Content: req.Content,
	}
	enabledUpdate := req.Enabled != nil
	if enabledUpdate {
		rec.Enabled = *req.Enabled
	}
	if err := h.rules.Update(r.Context(), rec, enabledUpdate, false); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	updated, err := h.rules.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.CustomRule]{
		Data: ruleRecordToDTO(updated), RequestID: traceID,
	})
}

// Delete implements DELETE /api/rules/{id}.
func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.rules.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// Reorder implements POST /api/rules/reorder + PUT /api/rules/order.
// Accepts a flat list of (id, sort) tuples; rules not in the payload keep
// their existing sort. Cross-user ids are silently skipped (the repo filters
// by user_id at the SQL level).
func (h *RuleHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.UpdateRuleOrderRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if len(req.Orders) == 0 {
		util.RespondError(w, types.ErrValidationRequiredField, "orders required", nil, traceID)
		return
	}
	entries := make([]storage.ReorderEntry, len(req.Orders))
	for i, o := range req.Orders {
		entries[i] = storage.ReorderEntry{ID: o.ID, Sort: o.Sort}
	}
	updated, err := h.rules.Reorder(r.Context(), user.ID, entries)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]int]{
		Data: map[string]int{"updated": updated}, RequestID: traceID,
	})
}

// Preview implements GET /api/rules/preview/{subID}. Renders the subscription
// to a Clash YAML doc (proxies block) and then applies every enabled rule.
// Returns the final YAML as a text/yaml body inside a JSON wrapper so the
// frontend can show diffs / line counts before/after.
func (h *RuleHandler) Preview(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	subID := r.PathValue("subID")
	if subID == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "subID required", nil, traceID)
		return
	}
	base, err := h.renderBaseYAML(r.Context(), subID, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	enabled, err := h.rules.ListEnabled(r.Context(), user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	rules := make([]*substore.CustomRule, len(enabled))
	for i := range enabled {
		rules[i] = &substore.CustomRule{
			ID: enabled[i].ID, Name: enabled[i].Name,
			Type: enabled[i].Type, Mode: enabled[i].Mode,
			Content: enabled[i].Content, Sort: enabled[i].Sort,
		}
	}
	out, err := substore.ApplyToYAML(base, rules)
	if err != nil {
		util.RespondError(w, types.ErrValidationYAMLParse, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]any]{
		Data: map[string]any{
			"base_yaml":  string(base),
			"final_yaml": string(out),
			"rule_count": len(rules),
		},
		RequestID: traceID,
	})
}

// Templates implements GET /api/rules/templates. Static catalog.
func (h *RuleHandler) Templates(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.RuleTemplate]{
		Data: builtInRuleTemplates, RequestID: traceID,
	})
}

// renderBaseYAML produces the Clash YAML that rules will be injected into.
// When the node fetcher is wired, the subscription's current node set is
// rendered via substore.ProduceClashYAML. Otherwise an empty proxies block is
// returned so the rule pipeline still has a valid mapping to mutate.
func (h *RuleHandler) renderBaseYAML(ctx context.Context, subID, userID string) ([]byte, error) {
	if h.subs == nil {
		return defaultEmptyClashBase(), nil
	}
	if _, err := h.subs.GetByID(ctx, subID, userID); err != nil {
		return nil, err
	}
	if h.nodeRepo == nil {
		return defaultEmptyClashBase(), nil
	}
	nodes, err := h.nodeRepo.ListForRender(ctx, subID)
	if err != nil {
		return nil, err
	}
	return substore.ProduceClashYAML(nodes, substore.ClashProducerOpts{})
}

// nextSort returns the smallest sort value that places a freshly created rule
// after every existing one. We use 100-step intervals so manual insertions
// between adjacent rules remain possible.
func (h *RuleHandler) nextSort(ctx context.Context, userID string) int32 {
	recs, _, err := h.rules.List(ctx, userID, storage.CustomRuleListOptions{})
	if err != nil || len(recs) == 0 {
		return 100
	}
	max := int32(0)
	for _, r := range recs {
		if r.Sort > max {
			max = r.Sort
		}
	}
	return max + 100
}

// respondStorageErr translates rule repo / subscription repo errors into the
// canonical envelope.
func (h *RuleHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrCustomRuleNotFound):
		util.RespondError(w, types.ErrNotFoundRule, "rule not found", nil, traceID)
	case errors.Is(err, storage.ErrSubscriptionNotFound):
		util.RespondError(w, types.ErrNotFoundSubscription, "subscription not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("rule handler db failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalDatabase, "internal error", nil, traceID)
	}
}

// ruleRecordToDTO projects a storage record into the contract DTO.
func ruleRecordToDTO(rec *storage.CustomRuleRecord) types.CustomRule {
	return types.CustomRule{
		ID: rec.ID, UserID: rec.UserID, Name: rec.Name,
		Type:      types.RuleType(rec.Type),
		Mode:      types.RuleMode(rec.Mode),
		Content:   rec.Content,
		Enabled:   rec.Enabled,
		Sort:      rec.Sort,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

// isValidRuleType / isValidRuleMode mirror the CHECK constraints in
// migrations/0001_initial.sql. Validation here gives the user a 400 instead
// of an opaque DB error.
func isValidRuleType(t types.RuleType) bool {
	switch t {
	case types.RuleTypeDNS, types.RuleTypeRules, types.RuleTypeRuleProviders:
		return true
	}
	return false
}

func isValidRuleMode(m types.RuleMode) bool {
	switch m {
	case types.RuleModeReplace, types.RuleModePrepend, types.RuleModeAppend:
		return true
	}
	return false
}

// validateRuleContent runs a cheap shape check on the rule's content so the
// user gets fast feedback instead of seeing the error only when rendering a
// preview. dns / rule-providers must be YAML mappings; rules can be free-form
// text (one rule per line).
func validateRuleContent(t types.RuleType, content string) error {
	switch t {
	case types.RuleTypeDNS, types.RuleTypeRuleProviders:
		// dry-run a parse — the injector also parses but emits less friendly
		// error messages.
		_, err := substore.ApplyToYAML([]byte("dns: {}\nrules: []\nrule-providers: {}\n"),
			[]*substore.CustomRule{
				{Name: "validate", Type: string(t), Mode: "replace", Content: content},
			})
		return err
	case types.RuleTypeRules:
		// Any non-empty content is acceptable; the injector tolerates list
		// markers and blank lines.
		return nil
	}
	return nil
}

// defaultEmptyClashBase returns a minimal Clash document used when no
// subscription / node fetcher is wired. Keeps the preview endpoint usable
// even before the user has synced a subscription.
func defaultEmptyClashBase() []byte {
	return []byte("proxies: []\ndns: {}\nrules: []\nrule-providers: {}\n")
}
