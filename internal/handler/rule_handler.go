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
		Content: `GEOIP,HK,🇭🇰 香港节点,no-resolve
`,
	},
	{
		ID: "region-jp", Name: "日本节点", Emoji: "🇯🇵", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "jp"},
		Description: "把命中日本 GEOIP 的流量打到 🇯🇵 日本节点 组。",
		Content: `GEOIP,JP,🇯🇵 日本节点,no-resolve
`,
	},
	{
		ID: "region-us", Name: "美国节点", Emoji: "🇺🇸", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "america", "us"},
		Description: "把命中美国 GEOIP 的流量打到 🇺🇸 美国节点 组。",
		Content: `GEOIP,US,🇺🇸 美国节点,no-resolve
`,
	},
	{
		ID: "region-sg", Name: "新加坡节点", Emoji: "🇸🇬", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "sg"},
		Description: "把命中新加坡 GEOIP 的流量打到 🇸🇬 新加坡节点 组。",
		Content: `GEOIP,SG,🇸🇬 新加坡节点,no-resolve
`,
	},
	{
		ID: "region-tw", Name: "台湾节点", Emoji: "🇹🇼", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "tw"},
		Description: "把命中台湾 GEOIP 的流量打到 🇹🇼 台湾节点 组。",
		Content: `GEOIP,TW,🇹🇼 台湾节点,no-resolve
`,
	},
	{
		ID: "region-kr", Name: "韩国节点", Emoji: "🇰🇷", Category: "region",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"region", "asia", "kr"},
		Description: "把命中韩国 GEOIP 的流量打到 🇰🇷 韩国节点 组。",
		Content: `GEOIP,KR,🇰🇷 韩国节点,no-resolve
`,
	},
	// ----------------------------------------------------------------------
	// app: 应用分组（DOMAIN-SUFFIX / RULE-SET 引用）
	// ----------------------------------------------------------------------
	{
		ID: "app-ai", Name: "AI 服务", Emoji: "🤖", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "ai", "openai", "anthropic", "gemini", "copilot"},
		Description: "OpenAI / Anthropic / Bing / Copilot / Gemini 等 AI 服务走 🤖 AI 服务 组。",
		Content: `RULE-SET,openai,🤖 AI 服务
RULE-SET,anthropic,🤖 AI 服务
RULE-SET,bing,🤖 AI 服务
RULE-SET,copilot,🤖 AI 服务
RULE-SET,gemini,🤖 AI 服务
`,
	},
	{
		ID: "app-streaming", Name: "流媒体", Emoji: "📺", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "streaming", "netflix", "youtube", "disney", "spotify", "tiktok", "bilibili"},
		Description: "YouTube / Netflix / Disney+ / Spotify / TikTok / Bilibili / Bahamut 等流媒体走 📺 流媒体 组。",
		Content: `RULE-SET,youtube,📺 流媒体
RULE-SET,netflix,📺 流媒体
RULE-SET,disney,📺 流媒体
RULE-SET,spotify,📺 流媒体
RULE-SET,tiktok,📺 流媒体
RULE-SET,bilibili,📺 流媒体
RULE-SET,bahamut,📺 流媒体
`,
	},
	{
		ID: "app-google", Name: "Google 服务", Emoji: "📢", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "google", "youtube"},
		Description: "Google 全家桶 + YouTube 走 📢 Google 组。",
		Content: `RULE-SET,google,📢 Google
RULE-SET,youtube,📢 Google
`,
	},
	{
		ID: "app-microsoft", Name: "微软服务", Emoji: "Ⓜ️", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "microsoft", "azure", "github", "linkedin"},
		Description: "Microsoft / GitHub / LinkedIn 走 Ⓜ️ 微软 组。",
		Content: `RULE-SET,microsoft,Ⓜ️ 微软
RULE-SET,github,Ⓜ️ 微软
RULE-SET,linkedin,Ⓜ️ 微软
`,
	},
	{
		ID: "app-apple", Name: "苹果服务", Emoji: "🍎", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "apple", "icloud"},
		Description: "Apple / iCloud 走 🍎 苹果 组。",
		Content: `RULE-SET,apple,🍎 苹果
RULE-SET,icloud,🍎 苹果
`,
	},
	{
		ID: "app-telegram", Name: "Telegram", Emoji: "✈️", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "telegram", "im"},
		Description: "Telegram 走 ✈️ Telegram 组。",
		Content: `RULE-SET,telegram,✈️ Telegram
`,
	},
	{
		ID: "app-gaming", Name: "游戏平台", Emoji: "🎮", Category: "app",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"app", "gaming", "steam", "epic", "ea", "nintendo"},
		Description: "Steam / Epic / EA / Nintendo 等游戏平台走 🎮 游戏 组。",
		Content: `RULE-SET,steam,🎮 游戏
RULE-SET,epic,🎮 游戏
RULE-SET,ea,🎮 游戏
RULE-SET,nintendo,🎮 游戏
`,
	},
	// ----------------------------------------------------------------------
	// block: 拦截类
	// ----------------------------------------------------------------------
	{
		ID: "block-ads", Name: "广告拦截", Emoji: "🚫", Category: "block",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"block", "ads"},
		Description: "聚合广告 + Windows 间谍 / 遥测 REJECT，命中即拦截。",
		Content: `RULE-SET,category-ads-all,REJECT
RULE-SET,win-spy,REJECT
RULE-SET,win-extra,REJECT
`,
	},
	{
		ID: "block-privacy", Name: "隐私保护", Emoji: "🛡️", Category: "block",
		RuleType: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Tags:        []string{"block", "privacy", "tracker", "malware", "phishing"},
		Description: "广告 + 追踪 + 恶意软件 + 钓鱼网站 REJECT。",
		Content: `RULE-SET,category-ads-all,REJECT
RULE-SET,tracking,REJECT
RULE-SET,malware,REJECT
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
		Description: "中国大陆域名 / IP 直连，非中国域名走代理，MATCH 兜底到代理。推荐 default 选项。",
		Content: `RULE-SET,cn-domain,DIRECT
RULE-SET,private-domain,DIRECT
RULE-SET,cn-ip,DIRECT,no-resolve
GEOIP,CN,DIRECT,no-resolve
RULE-SET,geolocation-!cn,🚀 节点选择
MATCH,🚀 节点选择
`,
	},
	{
		ID: "global-proxy", Name: "全局代理", Emoji: "🌍", Category: "common",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeReplace,
		Tags:        []string{"common", "global"},
		Description: "所有流量都走代理出口，仅私网 / 中国大陆域名保留直连。",
		Content: `RULE-SET,private-domain,DIRECT
RULE-SET,cn-domain,DIRECT
MATCH,🚀 节点选择
`,
	},
	{
		ID: "fallback-fish", Name: "漏网之鱼", Emoji: "🐟", Category: "common",
		RuleType: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Tags:        []string{"common", "fallback", "final"},
		Description: "MATCH 兜底规则：未命中任何规则的流量走 🚀 节点选择。",
		Content: `MATCH,🚀 节点选择
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
	// Preview path: we want only the proxies block to feed into the rule
	// injector, so we do NOT pre-seed proxy-groups / rule-providers /
	// default MATCH rule here — that's what ApplyToYAML below layers on.
	return substore.ProduceClashYAML(
		&substore.ClashRenderInput{Nodes: nodes},
		substore.ClashProducerOpts{ProxiesOnly: true},
	)
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
