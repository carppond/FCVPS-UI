package substore

import "context"

// NoopNodeRepo is a stub implementation of NodeRepo + NodeFetcher used until
// T-11 lands the real node persistence layer. It returns empty results from
// reads and treats writes as successful no-ops so the rest of the system
// (sync service, sub-store compat path) can be wired end-to-end ahead of T-11.
//
// Production builds must replace this stub with the real repo from
// internal/storage; the indirection (interface-based DI) makes that a
// one-line swap.
type NoopNodeRepo struct{}

// UpsertBatch implements NodeRepo.
func (NoopNodeRepo) UpsertBatch(ctx context.Context, subscriptionID string, nodes []NodeUpsertInput) (UpsertResult, error) {
	return UpsertResult{Total: len(nodes), Added: len(nodes)}, nil
}

// ListForRender implements NodeFetcher.
func (NoopNodeRepo) ListForRender(ctx context.Context, subscriptionID string) ([]*ParsedNode, error) {
	return nil, nil
}
