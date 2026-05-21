package substore

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
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
//   - Render the current node set as a Clash YAML document via the same
//     producer the standard /output endpoint uses.
type SubstoreCompatService struct {
	repo     *storage.SubscriptionRepo
	nodeRepo NodeFetcher
	sync     *SyncService
	now      func() time.Time
}

// NodeFetcher is the read-side counterpart to NodeRepo used by the compat
// path; the real implementation lands with T-11. Keeping it as an interface
// here lets tests inject a stub without instantiating the full node repo.
type NodeFetcher interface {
	ListForRender(ctx context.Context, subscriptionID string) ([]*ParsedNode, error)
}

// SubstoreCompatConfig wires the service.
type SubstoreCompatConfig struct {
	Repo     *storage.SubscriptionRepo
	NodeRepo NodeFetcher
	Sync     *SyncService
	Now      func() time.Time
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
	return &SubstoreCompatService{
		repo:     cfg.Repo,
		nodeRepo: cfg.NodeRepo,
		sync:     cfg.Sync,
		now:      now,
	}, nil
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
	producer := (&ProducerFactory{}).Get(target)
	body, contentType, err := producer.Produce(nodes, ClashProducerOpts{})
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
