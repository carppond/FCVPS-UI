package substore

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// SubstoreCompatService implements the read path consumed by the sub-store
// v2 compatible endpoint GET /download/:name?token=<share_token>.
//
// The service is deliberately minimal: there is no user session, so failure
// modes are folded into a single "not found" sentinel and the handler reports
// HTTP 404 across the board (docs/05-tech-lead-plan.md §1.3 — silent mode).
//
// Responsibilities:
//   - Validate token against the subscription's share_token in constant time.
//   - Optionally trigger an immediate sync if the cached nodes are stale.
//   - Render the current node set + the user's rule / proxy-group /
//     rule-set configuration as a Clash YAML document via the producer
//     factory.
type SubstoreCompatService struct {
	repo        *storage.SubscriptionRepo
	nodeRepo    NodeFetcher
	sync        *SyncService
	now         func() time.Time
	logger      *slog.Logger
	ruleRepo    customRuleLister
	groupRepo   proxyGroupLister
	ruleSetRepo ruleSetLister
}

// NodeFetcher is the read-side counterpart to NodeRepo used by the compat
// path; the real implementation lands with T-11. Keeping it as an interface
// here lets tests inject a stub without instantiating the full node repo.
type NodeFetcher interface {
	ListForRender(ctx context.Context, subscriptionID string) ([]*ParsedNode, error)
}

// customRuleLister narrows storage.CustomRuleRepo to just the read used by
// the compat path. The narrow interface keeps the unit tests in this package
// independent of the storage layer.
type customRuleLister interface {
	ListEnabled(ctx context.Context, userID string) ([]storage.CustomRuleRecord, error)
}

// proxyGroupLister narrows storage.ProxyGroupRepo. We want every group the
// user owns (sort_order ASC), not just enabled ones — proxy groups have no
// "enabled" column.
type proxyGroupLister interface {
	List(ctx context.Context, userID string, opts storage.ProxyGroupListOptions) ([]storage.ProxyGroupCategoryRecord, int64, error)
}

// ruleSetLister narrows storage.RuleSetProviderRepo to just ListEnabled.
type ruleSetLister interface {
	ListEnabled(ctx context.Context, userID string) ([]storage.RuleSetProviderRecord, error)
}

// SubstoreCompatConfig wires the service. RuleRepo / GroupRepo / RuleSetRepo
// are optional — when omitted, the compat service still renders nodes but the
// rendered Clash YAML will fall back to the producer's built-in defaults
// (single "🚀 节点选择" group + single MATCH rule). The intent is to keep older
// call sites and unit tests compiling without forcing every test to wire the
// full storage stack.
type SubstoreCompatConfig struct {
	Repo        *storage.SubscriptionRepo
	NodeRepo    NodeFetcher
	Sync        *SyncService
	Now         func() time.Time
	Logger      *slog.Logger
	RuleRepo    *storage.CustomRuleRepo
	GroupRepo   *storage.ProxyGroupRepo
	RuleSetRepo *storage.RuleSetProviderRepo
}

// NewSubstoreCompatService constructs the service.
func NewSubstoreCompatService(cfg SubstoreCompatConfig) (*SubstoreCompatService, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("substore compat: nil repo")
	}
	if cfg.NodeRepo == nil {
		return nil, fmt.Errorf("substore compat: nil node fetcher")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	svc := &SubstoreCompatService{
		repo:     cfg.Repo,
		nodeRepo: cfg.NodeRepo,
		sync:     cfg.Sync,
		now:      now,
		logger:   cfg.Logger,
	}
	// Each repo is captured behind a narrow interface so the rendering hot
	// path stays testable without spinning up the full storage layer.
	if cfg.RuleRepo != nil {
		svc.ruleRepo = cfg.RuleRepo
	}
	if cfg.GroupRepo != nil {
		svc.groupRepo = cfg.GroupRepo
	}
	if cfg.RuleSetRepo != nil {
		svc.ruleSetRepo = cfg.RuleSetRepo
	}
	return svc, nil
}

// ErrCompatNotFound is the single error surface for ServeDownload — token
// failures and "subscription not found" intentionally collapse to the same
// error so 404 leaks no information to a probing client.
var ErrCompatNotFound = errors.New("substore: compat resource not found")

// DownloadResult bundles the rendered body with a few headers the handler
// needs to write. ContentType is producer-specific (text/yaml for Clash,
// application/json for sing-box, text/plain for URI list / Surge).
//
// YAMLType is preserved as a legacy alias for ContentType so existing
// callers and tests continue to compile; new code should read ContentType.
type DownloadResult struct {
	Body        []byte
	TotalNodes  int
	ContentType string
	YAMLType    string // deprecated alias of ContentType, kept for back-compat
}

// ServeDownload looks up the subscription by token and renders its current
// node set in the format requested by target. target falls back to "clash"
// when empty so the legacy /download/:name?token=... call shape stays valid.
//
// If the subscription's last_synced_at is older than sync_interval, an
// inline best-effort SyncOne is run before rendering. Sync errors do not
// fail the request — the previous nodes are still served (PRD M-SUB.5
// "无网时返回上次成功 cache").
func (s *SubstoreCompatService) ServeDownload(ctx context.Context, name, token, target string) (*DownloadResult, error) {
	if name == "" || token == "" {
		return nil, ErrCompatNotFound
	}
	sub, err := s.repo.GetByShareToken(ctx, token)
	if err != nil {
		if errors.Is(err, storage.ErrSubscriptionNotFound) {
			return nil, ErrCompatNotFound
		}
		return nil, fmt.Errorf("lookup subscription: %w", err)
	}
	// Defence in depth: the path segment must also match the subscription
	// name. Without this an attacker who steals a token for sub A could
	// download it via /download/<unrelated-name>.
	if subtle.ConstantTimeCompare([]byte(sub.Name), []byte(name)) != 1 {
		return nil, ErrCompatNotFound
	}
	if subtle.ConstantTimeCompare([]byte(sub.ShareToken), []byte(token)) != 1 {
		return nil, ErrCompatNotFound
	}

	if s.sync != nil && s.isStale(sub) && sub.Type != string(types.SubTypeManual) {
		// Best-effort refresh; even on failure we still try to render the
		// previously cached nodes.
		_, _ = s.sync.SyncOne(ctx, sub)
	}

	nodes, err := s.nodeRepo.ListForRender(ctx, sub.ID)
	if err != nil {
		return nil, fmt.Errorf("list nodes for render: %w", err)
	}

	// Hydrate the user's rules / proxy-groups / rule-sets. Failures here are
	// downgraded to warnings — we still want to emit *something* so the
	// client receives a usable subscription even if a single tier of the
	// pipeline is down.
	customRules := s.listCustomRules(ctx, sub.UserID)
	proxyGroups := s.listProxyGroups(ctx, sub.UserID)
	ruleSets := s.listRuleSets(ctx, sub.UserID)

	input := &ClashRenderInput{
		Nodes:       nodes,
		CustomRules: customRules,
		ProxyGroups: proxyGroups,
		RuleSets:    ruleSets,
	}
	// Singbox / Surge / URI list producers do not (yet) emit routing rules;
	// emit a one-shot warning at info level so the operator knows the cause
	// when those targets render without the user's rule config.
	if s.logger != nil {
		switch target {
		case "singbox", "sing-box", "surge", "surge-mac", "surgemac", "surge-ios":
			s.logger.Info("substore compat: routing rules not yet emitted for non-clash target",
				slog.String("target", target),
				slog.Int("custom_rules", len(customRules)),
				slog.Int("proxy_groups", len(proxyGroups)),
				slog.Int("rule_sets", len(ruleSets)),
			)
		}
	}

	producer := (&ProducerFactory{}).Get(target)
	body, contentType, err := producer.Produce(input, ClashProducerOpts{})
	if err != nil {
		return nil, fmt.Errorf("render target %q: %w", target, err)
	}
	return &DownloadResult{
		Body:        body,
		TotalNodes:  len(nodes),
		ContentType: contentType,
		YAMLType:    contentType, // legacy alias
	}, nil
}

// listCustomRules fetches the enabled custom_rules rows for userID and maps
// them onto the substore-local CustomRuleRecord. Errors are swallowed (and
// optionally logged) so the render path stays best-effort: a broken table
// must not collapse the whole /download endpoint.
func (s *SubstoreCompatService) listCustomRules(ctx context.Context, userID string) []CustomRuleRecord {
	if s.ruleRepo == nil {
		return nil
	}
	recs, err := s.ruleRepo.ListEnabled(ctx, userID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("substore compat: list custom rules failed",
				slog.String("user_id", userID),
				slog.String("err", err.Error()))
		}
		return nil
	}
	out := make([]CustomRuleRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, CustomRuleRecord{
			ID: r.ID, Name: r.Name, Type: r.Type, Mode: r.Mode,
			Content: r.Content, Sort: r.Sort,
		})
	}
	return out
}

// listProxyGroups fetches every proxy_group_categories row for userID
// (sort_order ASC) and maps to the producer-local record. Same best-effort
// semantics as listCustomRules.
func (s *SubstoreCompatService) listProxyGroups(ctx context.Context, userID string) []ProxyGroupRecord {
	if s.groupRepo == nil {
		return nil
	}
	recs, _, err := s.groupRepo.List(ctx, userID, storage.ProxyGroupListOptions{})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("substore compat: list proxy groups failed",
				slog.String("user_id", userID),
				slog.String("err", err.Error()))
		}
		return nil
	}
	out := make([]ProxyGroupRecord, 0, len(recs))
	for _, g := range recs {
		out = append(out, ProxyGroupRecord{
			ID: g.ID, Name: g.Name, Type: g.Type, Icon: g.Icon,
			SortOrder:     g.SortOrder,
			TestURL:       g.TestURL,
			TestInterval:  g.TestInterval,
			Filter:        g.Filter,
			IncludeAll:    g.IncludeAll,
			MemberProxies: g.MemberProxies,
			MemberGroups:  g.MemberGroups,
		})
	}
	return out
}

// listRuleSets fetches the enabled rule_set_providers rows for userID.
func (s *SubstoreCompatService) listRuleSets(ctx context.Context, userID string) []RuleSetRecord {
	if s.ruleSetRepo == nil {
		return nil
	}
	recs, err := s.ruleSetRepo.ListEnabled(ctx, userID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("substore compat: list rule sets failed",
				slog.String("user_id", userID),
				slog.String("err", err.Error()))
		}
		return nil
	}
	out := make([]RuleSetRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, RuleSetRecord{
			ID: r.ID, Name: r.Name, Behavior: r.Behavior,
			Format: r.Format, URL: r.URL, IntervalSeconds: r.IntervalSeconds,
		})
	}
	return out
}

// isStale returns true when the subscription has never been synced or the
// last sync is older than sync_interval seconds.
func (s *SubstoreCompatService) isStale(sub *storage.SubscriptionRecord) bool {
	if sub.SyncInterval <= 0 {
		return false
	}
	if sub.LastSyncedAt <= 0 {
		return true
	}
	cutoff := s.now().Add(-time.Duration(sub.SyncInterval) * time.Second).UnixMilli()
	return sub.LastSyncedAt < cutoff
}
