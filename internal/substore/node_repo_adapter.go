package substore

import (
	"context"

	"shiguang-vps/internal/storage"
)

// NodeRepoAdapter bridges the storage.NodeRepo concrete type to the
// substore.NodeRepo + substore.NodeFetcher interfaces. The indirection avoids
// a storage → substore import cycle (storage cannot reference substore
// because substore already references storage); the adapter lives here, in
// the upper layer, where both packages are reachable.
//
// Wire it once in main.go and pass the *NodeRepoAdapter to both
// SyncServiceConfig.NodeRepo and SubstoreCompatConfig.NodeRepo.
type NodeRepoAdapter struct {
	Repo *storage.NodeRepo
}

// NewNodeRepoAdapter returns an adapter that satisfies substore.NodeRepo and
// substore.NodeFetcher backed by the supplied *storage.NodeRepo.
func NewNodeRepoAdapter(repo *storage.NodeRepo) *NodeRepoAdapter {
	return &NodeRepoAdapter{Repo: repo}
}

// UpsertBatch implements substore.NodeRepo by translating NodeUpsertInput
// (substore-shaped) into storage.NodeUpsertInput before delegating.
func (a *NodeRepoAdapter) UpsertBatch(ctx context.Context, subID string, nodes []NodeUpsertInput) (UpsertResult, error) {
	if a == nil || a.Repo == nil {
		return UpsertResult{}, nil
	}
	out := make([]storage.NodeUpsertInput, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, storage.NodeUpsertInput{
			RawURI:       n.RawURI,
			Protocol:     n.Protocol,
			Server:       n.Server,
			Port:         n.Port,
			Tag:          n.Tag,
			Position:     n.Position,
			ParsedConfig: parsedNodeToMap(n.ParsedConfig),
		})
	}
	res, err := a.Repo.UpsertBatch(ctx, subID, out)
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{
		Added:   res.Added,
		Updated: res.Updated,
		Removed: res.Removed,
		Total:   res.Total,
	}, nil
}

// ListForRender implements substore.NodeFetcher. It hydrates every node
// belonging to subID and rebuilds a ParsedNode struct from the persisted
// parsed_config_json payload so the Clash producer has the rich data it
// needs (tls / network / sni / alpn / etc).
func (a *NodeRepoAdapter) ListForRender(ctx context.Context, subID string) ([]*ParsedNode, error) {
	if a == nil || a.Repo == nil {
		return nil, nil
	}
	records, err := a.Repo.ListBySubscription(ctx, subID)
	if err != nil {
		return nil, err
	}
	out := make([]*ParsedNode, 0, len(records))
	for i := range records {
		out = append(out, recordToParsedNode(&records[i]))
	}
	return out, nil
}

// parsedNodeToMap converts a ParsedNode into a generic map so it can be
// persisted in the parsed_config_json column. Empty input returns nil so
// callers can short-circuit to "{}".
func parsedNodeToMap(p *ParsedNode) map[string]any {
	if p == nil {
		return nil
	}
	m := map[string]any{
		"name":     p.Name,
		"protocol": p.Protocol,
		"server":   p.Server,
		"port":     p.Port,
		"network":  p.Network,
		"tls":      p.TLS,
	}
	if p.UUID != "" {
		m["uuid"] = p.UUID
	}
	if p.Password != "" {
		m["password"] = p.Password
	}
	if p.Method != "" {
		m["method"] = p.Method
	}
	if p.SNI != "" {
		m["sni"] = p.SNI
	}
	if len(p.ALPN) > 0 {
		m["alpn"] = p.ALPN
	}
	if p.Path != "" {
		m["path"] = p.Path
	}
	if p.Host != "" {
		m["host"] = p.Host
	}
	if p.Reality {
		m["reality"] = true
	}
	if p.Tag != "" {
		m["tag"] = p.Tag
	}
	if len(p.Raw) > 0 {
		m["_raw"] = p.Raw
	}
	return m
}

// recordToParsedNode reverses parsedNodeToMap for the producer read path.
// Missing fields default to zero values; the Clash producer is tolerant of
// partial structs.
func recordToParsedNode(rec *storage.NodeRecord) *ParsedNode {
	if rec == nil {
		return nil
	}
	p := &ParsedNode{
		Name:     rec.Tag,
		Protocol: rec.Protocol,
		Server:   rec.Server,
		Port:     int(rec.Port),
		Tag:      rec.Tag,
	}
	cfg := rec.ParsedConfig
	if cfg == nil {
		return p
	}
	p.UUID, _ = cfg["uuid"].(string)
	p.Password, _ = cfg["password"].(string)
	p.Method, _ = cfg["method"].(string)
	p.Network, _ = cfg["network"].(string)
	p.SNI, _ = cfg["sni"].(string)
	p.Path, _ = cfg["path"].(string)
	p.Host, _ = cfg["host"].(string)
	if v, ok := cfg["tls"].(bool); ok {
		p.TLS = v
	}
	if v, ok := cfg["reality"].(bool); ok {
		p.Reality = v
	}
	if alpns, ok := cfg["alpn"].([]any); ok {
		p.ALPN = make([]string, 0, len(alpns))
		for _, a := range alpns {
			if s, ok := a.(string); ok {
				p.ALPN = append(p.ALPN, s)
			}
		}
	}
	if rawMap, ok := cfg["_raw"].(map[string]any); ok {
		p.Raw = rawMap
	}
	return p
}
