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
// Three templates ship with v1 (PRD M-RULE.4):
//   - cn-direct-foreign-proxy : 国内直连 + 国外代理
//   - global-proxy            : 纯透明代理 (一切走代理)
//   - ad-block                : 广告屏蔽（rule-providers 注入）
var builtInRuleTemplates = []types.RuleTemplate{
	{
		ID:          "cn-direct-foreign-proxy",
		Name:        "国内直连 + 国外代理",
		Description: "中国大陆 IP / 域名直连，其余流量走代理。基于 GEOIP + DOMAIN-SUFFIX 常见列表。",
		Content: `DOMAIN-SUFFIX,cn,DIRECT
DOMAIN-KEYWORD,baidu,DIRECT
DOMAIN-KEYWORD,taobao,DIRECT
DOMAIN-KEYWORD,jd,DIRECT
DOMAIN-KEYWORD,qq,DIRECT
DOMAIN-KEYWORD,weibo,DIRECT
GEOIP,CN,DIRECT
MATCH,Proxy
`,
	},
	{
		ID:          "global-proxy",
		Name:        "全局代理",
		Description: "所有流量都走代理出口，仅本地保留直连。适合 OpenWrt / 透明代理网关场景。",
		Content: `DOMAIN-SUFFIX,localhost,DIRECT
IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
MATCH,Proxy
`,
	},
	{
		ID:          "ad-block",
		Name:        "广告屏蔽",
		Description: "通过 rule-providers 引入开源广告列表（ACL4SSR），并在 rules 前置 REJECT。",
		Content: `RULE-SET,reject,REJECT
RULE-SET,direct,DIRECT
RULE-SET,proxy,Proxy
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
