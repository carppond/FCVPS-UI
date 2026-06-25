package substore

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util/safehttp"
)

// Default operational parameters for SyncService.
const (
	// DefaultUserAgent is used when subscription.UA is empty.
	DefaultUserAgent = "clash.meta"
	// DefaultHTTPTimeout caps every fetch attempt.
	DefaultHTTPTimeout = 30 * time.Second
	// MaxResponseBytes caps the upstream body size to defend against
	// memory blow-ups (1 MiB is generous for the average subscription).
	MaxResponseBytes = 8 * 1024 * 1024
)

// NodeUpsertInput is the shape SyncService produces for each parsed node. The
// downstream NodeRepo decides how to translate it to a persistence row; the
// minimal interface here keeps the sync service decoupled from T-11.
type NodeUpsertInput struct {
	SubscriptionID string
	RawURI         string
	Protocol       string
	Server         string
	Port           int
	Tag            string
	ParsedConfig   *ParsedNode
	Position       int
}

// UpsertResult summarises how many rows were added / kept / removed by a
// single sync. NodeRepo implementations populate it so SyncService can return
// it in types.SyncResult.
type UpsertResult struct {
	Added   int
	Updated int
	Removed int
	Total   int
}

// NodeRepo is the minimum surface SyncService needs from the node persistence
// layer. The full repo lands in T-11; until then a stub in the same package
// (used by tests) keeps the service compilable and unit-testable.
type NodeRepo interface {
	UpsertBatch(ctx context.Context, subscriptionID string, nodes []NodeUpsertInput) (UpsertResult, error)
}

// ScriptHookRunner is invoked at the two extension points described in
// docs/03-architecture.md §3.4 (M-SCRIPT). Implementations live in T-13; the
// service tolerates a nil runner (hooks become no-ops).
type ScriptHookRunner interface {
	// RunPostFetch lets a user script mutate the raw subscription body
	// after fetch but before parsing.
	RunPostFetch(ctx context.Context, userID string, body []byte) ([]byte, error)
	// RunPreSaveNodes lets a user script mutate the parsed node slice
	// before they are written to the database.
	RunPreSaveNodes(ctx context.Context, userID string, nodes []*ParsedNode) ([]*ParsedNode, error)
}

// NotifyHook is invoked when a sync fails so the notification module (T-22)
// can deliver the alert. Implementations may be nil during T-8.
type NotifyHook interface {
	EmitSubscriptionSyncFailed(ctx context.Context, sub *storage.SubscriptionRecord, errMsg string)
}

// EventBus is the minimal SSE event surface; SyncService publishes one
// "subscription_sync" event per completed sync. The real implementation lands
// in T-22; nil is tolerated.
type EventBus interface {
	Publish(ctx context.Context, kind string, payload any)
}

// SyncService orchestrates the end-to-end sync of one subscription.
//
// Flow:
//
//  1. Resolve the upstream body (HTTP fetch for type=url, raw_content for
//     upload, no-op for manual).
//  2. Run post_fetch script hook (optional).
//  3. Parse the body — YAML proxies / base64 / plaintext URI list.
//  4. Run pre_save_nodes script hook (optional).
//  5. Upsert via NodeRepo.
//  6. Update sync status + raw_content + publish SSE event.
//
// Errors at any step are persisted via UpdateSyncState(status=error) and
// surfaced through NotifyHook.
type SyncService struct {
	repo       *storage.SubscriptionRepo
	nodeRepo   NodeRepo
	httpClient *http.Client
	// insecureClient mirrors httpClient but skips TLS verification — used only
	// for subscriptions with allow_insecure=true (trusted upstreams whose cert
	// is self-signed/expired). Lazily nil-safe via getClient.
	insecureClient *http.Client
	hooks          ScriptHookRunner
	notify         NotifyHook
	events         EventBus
	syncLog        SyncLogRecorder
	logger         *slog.Logger
	now            func() time.Time
}

// SyncLogRecorder appends a sync-history row. Implemented by
// storage.SubscriptionSyncLogRepo; optional (nil → history disabled).
type SyncLogRecorder interface {
	Record(ctx context.Context, rec storage.SubscriptionSyncLogRecord) error
}

// SyncServiceConfig bundles SyncService dependencies.
type SyncServiceConfig struct {
	Repo     *storage.SubscriptionRepo
	NodeRepo NodeRepo
	// HTTPClient fetches over verified TLS. Inject the project's safehttp
	// client so subscription URLs cannot reach internal/loopback addresses.
	HTTPClient *http.Client
	// InsecureHTTPClient is used only for subscriptions with AllowInsecure=true.
	// It MUST still go through the safehttp dialer (SSRF guard) — only TLS
	// verification is skipped. nil falls back to a safehttp transport with
	// private networks blocked (fail-safe).
	InsecureHTTPClient *http.Client
	Hooks              ScriptHookRunner
	Notify             NotifyHook
	Events             EventBus
	SyncLog            SyncLogRecorder
	Logger             *slog.Logger
	Now                func() time.Time
}

// NewSyncService wires the service. nil HTTPClient defaults to a fresh
// http.Client with the project-wide timeout; nil Now defaults to time.Now.
func NewSyncService(cfg SyncServiceConfig) (*SyncService, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("sync service: nil repo")
	}
	if cfg.NodeRepo == nil {
		return nil, fmt.Errorf("sync service: nil node repo")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	insecureClient := cfg.InsecureHTTPClient
	if insecureClient == nil {
		// Fail-safe default: still SSRF-guarded (private nets blocked),
		// only TLS verification skipped. Callers should inject one built
		// from the deployment's allow_private_networks setting.
		t := safehttp.NewTransport(safehttp.Config{AllowPrivate: false})
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in per subscription
		insecureClient = &http.Client{Timeout: client.Timeout, Transport: t}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &SyncService{
		repo:           cfg.Repo,
		nodeRepo:       cfg.NodeRepo,
		httpClient:     client,
		insecureClient: insecureClient,
		hooks:          cfg.Hooks,
		notify:         cfg.Notify,
		events:         cfg.Events,
		syncLog:        cfg.SyncLog,
		logger:         cfg.Logger,
		now:            now,
	}, nil
}

// SyncOne runs the full sync flow for a single subscription. The result is
// always returned (even on partial failure) so callers can audit the attempt.
func (s *SyncService) SyncOne(ctx context.Context, sub *storage.SubscriptionRecord) (*types.SyncResult, error) {
	if sub == nil {
		return nil, fmt.Errorf("sync one: nil subscription")
	}
	startedAt := s.now()
	body, info, err := s.resolveBody(ctx, sub)
	if err != nil {
		s.failSync(ctx, sub, err)
		return nil, err
	}
	if s.hooks != nil {
		hooked, hookErr := s.hooks.RunPostFetch(ctx, sub.UserID, body)
		if hookErr != nil {
			s.failSync(ctx, sub, fmt.Errorf("post_fetch hook: %w", hookErr))
			return nil, hookErr
		}
		if hooked != nil {
			body = hooked
		}
	}
	parsed, parseErrs := parseSubscriptionBody(body)
	if len(parsed) == 0 && len(parseErrs) > 0 {
		// Treat zero-success as a hard failure so the operator notices.
		err := fmt.Errorf("parse subscription: %d errors, no nodes parsed (first: %v)", len(parseErrs), parseErrs[0])
		s.failSync(ctx, sub, err)
		return nil, err
	}
	if s.hooks != nil {
		hooked, hookErr := s.hooks.RunPreSaveNodes(ctx, sub.UserID, parsed)
		if hookErr != nil {
			s.failSync(ctx, sub, fmt.Errorf("pre_save_nodes hook: %w", hookErr))
			return nil, hookErr
		}
		if hooked != nil {
			parsed = hooked
		}
	}
	inputs := nodeInputsFromParsed(sub.ID, parsed)
	upsertResult, err := s.nodeRepo.UpsertBatch(ctx, sub.ID, inputs)
	if err != nil {
		s.failSync(ctx, sub, fmt.Errorf("upsert batch: %w", err))
		return nil, err
	}
	if len(body) > 0 {
		// Best-effort persist of the latest raw body; failure is logged but
		// does not fail the sync (nodes are already in place).
		if rawErr := s.repo.UpdateRawContent(ctx, sub.ID, body); rawErr != nil && s.logger != nil {
			s.logger.Warn(
				"subscription update raw_content failed",
				slog.String("subscription_id", sub.ID),
				slog.String("err", rawErr.Error()),
			)
		}
	}
	if err := s.repo.UpdateSyncState(ctx, sub.ID, string(types.SyncStatusOK), s.now(), ""); err != nil {
		// Sync ran but state metadata failed: still surface to caller.
		return nil, fmt.Errorf("update sync state: %w", err)
	}
	s.recordSyncLog(ctx, sub, string(types.SyncStatusOK), upsertResult.Total, "")
	// Best-effort: refresh traffic/expiry from the upstream Subscription-Userinfo
	// header (so used/limit match the source panel, e.g. 3X-UI).
	if info != nil {
		if mErr := s.repo.UpdateTrafficMeta(ctx, sub.ID, info.used, info.total, info.expireMs); mErr != nil && s.logger != nil {
			s.logger.Warn(
				"subscription update traffic meta failed",
				slog.String("subscription_id", sub.ID),
				slog.String("err", mErr.Error()),
			)
		}
	}
	result := &types.SyncResult{
		SubscriptionID: sub.ID,
		NodeCount:      int32(upsertResult.Total),
		AddedCount:     int32(upsertResult.Added),
		RemovedCount:   int32(upsertResult.Removed),
		SyncedAt:       startedAt.UnixMilli(),
	}
	if s.events != nil {
		s.events.Publish(ctx, "subscription_sync", result)
	}
	return result, nil
}

// SyncAll iterates the user's subscriptions and triggers SyncOne for each. It
// continues on per-row errors and returns the first one observed (alongside a
// best-effort log of the rest). Subscriptions of type=manual are skipped.
func (s *SyncService) SyncAll(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("sync all: empty user id")
	}
	subs, _, err := s.repo.List(ctx, userID, storage.SubscriptionListOptions{PageSize: 100})
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}
	var firstErr error
	for i := range subs {
		sub := subs[i]
		if sub.Type == string(types.SubTypeManual) {
			continue
		}
		if _, err := s.SyncOne(ctx, &sub); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if s.logger != nil {
				s.logger.Warn(
					"sync subscription failed",
					slog.String("subscription_id", sub.ID),
					slog.String("err", err.Error()),
				)
			}
		}
	}
	return firstErr
}

// userinfoMeta holds the traffic/expiry figures parsed from the upstream
// `Subscription-Userinfo` response header (the de-facto standard used by 3X-UI,
// sub-store, etc.). nil when the header is absent/unparseable.
type userinfoMeta struct {
	used     int64 // upload + download
	total    int64 // 0 = unlimited/unknown
	expireMs int64 // 0 = no expiry in header
}

// parseSubscriptionUserinfo parses a header like
// "upload=455727941; download=6174315083; total=1073741824000; expire=1719792000".
// expire is unix seconds → converted to ms to match the subscription contract.
func parseSubscriptionUserinfo(h string) *userinfoMeta {
	if strings.TrimSpace(h) == "" {
		return nil
	}
	var up, down, total, expire int64
	got := false
	for _, part := range strings.Split(h, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "upload":
			up, got = n, true
		case "download":
			down, got = n, true
		case "total":
			total, got = n, true
		case "expire":
			expire, got = n, true
		}
	}
	if !got {
		return nil
	}
	m := &userinfoMeta{used: up + down, total: total}
	if expire > 0 {
		m.expireMs = expire * 1000
	}
	return m
}

// resolveBody chooses the right source per subscription type. The returned
// userinfoMeta is non-nil only for url subscriptions whose upstream emitted a
// Subscription-Userinfo header.
func (s *SyncService) resolveBody(ctx context.Context, sub *storage.SubscriptionRecord) ([]byte, *userinfoMeta, error) {
	switch sub.Type {
	case string(types.SubTypeURL):
		if sub.SourceURL == "" {
			return nil, nil, fmt.Errorf("subscription %s: type=url with empty source_url", sub.ID)
		}
		return s.fetchURL(ctx, sub.SourceURL, sub.UA, sub.AllowInsecure)
	case string(types.SubTypeUpload):
		if len(sub.RawContent) == 0 {
			return nil, nil, fmt.Errorf("subscription %s: type=upload with empty raw_content", sub.ID)
		}
		return sub.RawContent, nil, nil
	case string(types.SubTypeManual):
		// Manual subscriptions sync via per-node POSTs (T-11). No-op here.
		return nil, nil, fmt.Errorf("subscription %s: type=manual cannot be synced", sub.ID)
	}
	return nil, nil, fmt.Errorf("subscription %s: unknown type %q", sub.ID, sub.Type)
}

// fetchURL performs the HTTP GET with the supplied UA (or DefaultUserAgent) and
// parses the Subscription-Userinfo header (traffic/expiry) when present.
func (s *SyncService) fetchURL(ctx context.Context, url, ua string, insecure bool) ([]byte, *userinfoMeta, error) {
	if ua == "" {
		ua = DefaultUserAgent
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	client := s.httpClient
	if insecure && s.insecureClient != nil {
		client = s.insecureClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("http get %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > MaxResponseBytes {
		return nil, nil, fmt.Errorf("subscription body exceeds %d bytes", MaxResponseBytes)
	}
	return body, parseSubscriptionUserinfo(resp.Header.Get("Subscription-Userinfo")), nil
}

// failSync records the failure state and emits a notification. It deliberately
// swallows secondary errors (UpdateSyncState / notify) so callers see the
// primary cause.
func (s *SyncService) failSync(ctx context.Context, sub *storage.SubscriptionRecord, cause error) {
	if cause == nil {
		return
	}
	msg := cause.Error()
	if err := s.repo.UpdateSyncState(ctx, sub.ID, string(types.SyncStatusError), s.now(), msg); err != nil && s.logger != nil {
		s.logger.Warn(
			"subscription update sync_state failed",
			slog.String("subscription_id", sub.ID),
			slog.String("err", err.Error()),
		)
	}
	s.recordSyncLog(ctx, sub, string(types.SyncStatusError), 0, msg)
	if s.notify != nil {
		s.notify.EmitSubscriptionSyncFailed(ctx, sub, msg)
	}
}

// recordSyncLog appends a sync-history row (best-effort; nil recorder or write
// failure is non-fatal — history must never break a sync).
func (s *SyncService) recordSyncLog(ctx context.Context, sub *storage.SubscriptionRecord, status string, nodeCount int, errMsg string) {
	if s.syncLog == nil {
		return
	}
	if err := s.syncLog.Record(ctx, storage.SubscriptionSyncLogRecord{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		Status:         status,
		NodeCount:      nodeCount,
		Error:          errMsg,
		CreatedAt:      s.now().UnixMilli(),
	}); err != nil && s.logger != nil {
		s.logger.Warn(
			"subscription record sync log failed",
			slog.String("subscription_id", sub.ID),
			slog.String("err", err.Error()),
		)
	}
}

// parseSubscriptionBody decides which decoder applies based on heuristics:
//
//  1. If the first 200 bytes contain a "proxies:" key it is parsed as Clash YAML.
//  2. Otherwise, try base64 decode — if the result yields any URI, treat it
//     as a base64-wrapped URI list.
//  3. Fall back to treating the body as a plaintext URI list.
//
// Returns the parsed nodes plus any per-line errors from ParseBulk.
func parseSubscriptionBody(body []byte) ([]*ParsedNode, []*BulkError) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var clashErr error
	// 1. Direct Clash YAML.
	if isClashYAML(trimmed) {
		nodes, err := parseClashYAML(trimmed)
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
		clashErr = err // remember, but DON'T hard-fail — try other formats too
	}
	// 2. base64-wrapped — which may decode to a URI list OR a Clash YAML (some
	//    providers return base64(clash.yaml)). Previously the Clash-in-base64
	//    case was misdetected and the whole subscription parsed to 0 nodes.
	if decoded, ok := tryBase64Decode(trimmed); ok {
		if isClashYAML(decoded) {
			if nodes, err := parseClashYAML(decoded); err == nil && len(nodes) > 0 {
				return nodes, nil
			} else if err != nil && clashErr == nil {
				clashErr = err
			}
		}
		if nodes, errs := ParseBulk(string(decoded)); len(nodes) > 0 {
			return nodes, errs
		}
	}
	// 3. Plaintext URI list.
	if nodes, errs := ParseBulk(string(trimmed)); len(nodes) > 0 {
		return nodes, errs
	}
	// Nothing parsed — surface the Clash error if we had one, else the URI errs.
	if clashErr != nil {
		return nil, []*BulkError{{Line: 0, Err: clashErr}}
	}
	return ParseBulk(string(trimmed))
}

// isClashYAML returns true when the first 200 bytes of body contain the
// canonical Clash root key "proxies:". Heuristic only — false negatives fall
// through to base64/URI list paths.
func isClashYAML(body []byte) bool {
	head := body
	if len(head) > 4096 {
		head = head[:4096]
	}
	return bytes.Contains(head, []byte("proxies:")) ||
		bytes.Contains(head, []byte("Proxy:"))
}

// tryBase64Decode attempts std and url base64 decoding (with/without padding).
// Returns the decoded bytes when at least one variant produced parseable text.
func tryBase64Decode(body []byte) ([]byte, bool) {
	// Filter whitespace which is common in real-world payloads.
	clean := bytes.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, body)
	if len(clean) == 0 {
		return nil, false
	}
	candidates := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range candidates {
		out, err := enc.DecodeString(string(clean))
		if err == nil && (looksLikeURIList(out) || isClashYAML(out)) {
			return out, true
		}
	}
	// Tolerate padding mismatch by retrying StdEncoding with manual padding.
	padded := append([]byte(nil), clean...)
	if pad := len(padded) % 4; pad != 0 {
		padded = append(padded, bytes.Repeat([]byte("="), 4-pad)...)
		if out, err := base64.StdEncoding.DecodeString(string(padded)); err == nil && (looksLikeURIList(out) || isClashYAML(out)) {
			return out, true
		}
	}
	return nil, false
}

// looksLikeURIList returns true when the byte slice contains at least one
// "scheme://" token from the recognised protocol set.
func looksLikeURIList(b []byte) bool {
	s := string(b)
	for _, scheme := range []string{
		"vmess://", "vless://", "ss://", "ssr://", "trojan://",
		"hysteria://", "hysteria2://", "hy2://", "tuic://",
		"wireguard://", "anytls://", "socks5://", "naive+",
	} {
		if strings.Contains(s, scheme) {
			return true
		}
	}
	return false
}

// parseClashYAML extracts the proxies: sequence and runs each element through
// a minimal proxy-to-ParsedNode mapper. The result is a best-effort projection
// — fields we do not understand are stored verbatim in ParsedNode.Raw.
func parseClashYAML(body []byte) ([]*ParsedNode, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, errors.New("yaml: empty document")
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, errors.New("yaml: root is not a mapping")
	}
	proxies := findProxiesNode(doc)
	if proxies == nil {
		return nil, errors.New("yaml: proxies key not found")
	}
	if proxies.Kind != yaml.SequenceNode {
		return nil, errors.New("yaml: proxies is not a sequence")
	}
	out := make([]*ParsedNode, 0, len(proxies.Content))
	for _, entry := range proxies.Content {
		if entry.Kind != yaml.MappingNode {
			continue
		}
		var m map[string]interface{}
		if err := entry.Decode(&m); err != nil {
			continue
		}
		node := clashEntryToNode(m)
		if node == nil {
			continue
		}
		out = append(out, node)
	}
	return out, nil
}

// findProxiesNode searches a mapping for the "proxies" or legacy "Proxy" key.
func findProxiesNode(doc *yaml.Node) *yaml.Node {
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key := doc.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "proxies", "Proxy":
			return doc.Content[i+1]
		}
	}
	return nil
}

// clashEntryToNode projects a Clash proxy YAML map into a ParsedNode. Unknown
// keys land in Raw so the round-trip is lossless.
func clashEntryToNode(m map[string]interface{}) *ParsedNode {
	if m == nil {
		return nil
	}
	n := &ParsedNode{Raw: make(map[string]interface{})}
	if v, ok := m["name"].(string); ok {
		n.Name = v
	}
	if v, ok := m["type"].(string); ok {
		n.Protocol = v
	}
	if v, ok := m["server"].(string); ok {
		n.Server = v
	}
	switch p := m["port"].(type) {
	case int:
		n.Port = p
	case int64:
		n.Port = int(p)
	case float64:
		n.Port = int(p)
	case string:
		if v, err := parsePort(p); err == nil {
			n.Port = v
		}
	}
	if v, ok := m["uuid"].(string); ok {
		n.UUID = v
	}
	if v, ok := m["password"].(string); ok {
		n.Password = v
	}
	if v, ok := m["cipher"].(string); ok {
		n.Method = v
	}
	if v, ok := m["network"].(string); ok {
		n.Network = v
	}
	if v, ok := m["tls"].(bool); ok {
		n.TLS = v
	}
	if v, ok := m["sni"].(string); ok {
		n.SNI = v
	}
	// Normalize Clash's nested / differently-named transport+TLS fields into the
	// same flat n.* + Raw shape the URI parsers produce — otherwise the
	// uri-list and sing-box producers (which read n.SNI / n.Path / n.Host /
	// n.Reality / flat Raw keys, NOT Clash's nested maps) lose SNI, ws/grpc
	// path, and reality entirely for Clash-sourced subscriptions.
	normalizeClashNode(n, m)
	// Stash everything else in Raw to preserve unsupported fields (the Clash
	// producer promotes valid Clash keys from here back to the top level).
	for k, v := range m {
		switch k {
		case "name", "type", "server", "port", "uuid", "password",
			"cipher", "network", "tls", "sni":
			continue
		// "_raw" is OUR internal passthrough marker, never a real proxy field.
		// A source carrying it (a prior render of ours, re-synced) would nest
		// _raw→_raw deeper every sync; drop it so the corruption can't compound.
		case "_raw":
			continue
		}
		n.Raw[k] = v
	}
	if n.Protocol == "" || n.Server == "" || n.Port == 0 {
		return nil
	}
	if n.Name == "" {
		n.Name = fmt.Sprintf("%s-%s:%d", n.Protocol, n.Server, n.Port)
	}
	return n
}

// normalizeClashNode bridges Clash-format fields into the flat representation
// the URI / sing-box producers expect. The nested originals are LEFT in the
// node's Raw map (the Clash producer's passthrough relies on them), so this
// only adds — it never removes — keeping Clash→Clash lossless.
func normalizeClashNode(n *ParsedNode, m map[string]interface{}) {
	// Clash's TLS SNI key is "servername" (vless/vmess); URI parsers use n.SNI.
	if n.SNI == "" {
		if v, ok := m["servername"].(string); ok {
			n.SNI = v
		}
	}
	// Reality: the presence of reality-opts marks a reality node. Flatten its
	// keys to pbk/sid/spx so the URI + sing-box producers (which read these
	// flat keys) can emit reality.
	if ro, ok := m["reality-opts"].(map[string]interface{}); ok {
		n.Reality = true
		if v, ok := ro["public-key"].(string); ok {
			n.Raw["pbk"] = v
		}
		if v, ok := ro["short-id"].(string); ok {
			n.Raw["sid"] = v
		}
		if v, ok := ro["spider-x"].(string); ok {
			n.Raw["spx"] = v
		}
	}
	// vless flow + uTLS fingerprint sit at the Clash node top level; map them
	// to the flat Raw keys the producers consume.
	if v, ok := m["flow"].(string); ok && v != "" {
		n.Raw["flow"] = v
	}
	if v, ok := m["client-fingerprint"].(string); ok && v != "" {
		n.Raw["fp"] = v
	}
	// WebSocket transport: pull path + Host header into n.Path / n.Host.
	if wo, ok := m["ws-opts"].(map[string]interface{}); ok {
		if p, ok := wo["path"].(string); ok && n.Path == "" {
			n.Path = p
		}
		if hdrs, ok := wo["headers"].(map[string]interface{}); ok {
			if h, ok := hdrs["Host"].(string); ok && n.Host == "" {
				n.Host = h
			} else if h, ok := hdrs["host"].(string); ok && n.Host == "" {
				n.Host = h
			}
		}
	}
	// gRPC transport: flatten the service name for the producers.
	if grpc, ok := m["grpc-opts"].(map[string]interface{}); ok {
		if sn, ok := grpc["grpc-service-name"].(string); ok && sn != "" {
			n.Raw["serviceName"] = sn
		}
	}
	// vmess alterId → flat "aid" (URI vmess JSON + clash producer read aid).
	switch a := m["alterId"].(type) {
	case int:
		n.Raw["aid"] = strconv.Itoa(a)
	case int64:
		n.Raw["aid"] = strconv.FormatInt(a, 10)
	case float64:
		n.Raw["aid"] = strconv.FormatInt(int64(a), 10)
	case string:
		if a != "" {
			n.Raw["aid"] = a
		}
	}
	// ALPN list.
	if len(n.ALPN) == 0 {
		switch a := m["alpn"].(type) {
		case []interface{}:
			for _, e := range a {
				if s, ok := e.(string); ok && s != "" {
					n.ALPN = append(n.ALPN, s)
				}
			}
		case string:
			n.ALPN = splitALPN(a)
		}
	}
}

// nodeInputsFromParsed converts the parser output into NodeRepo.UpsertBatch
// inputs. Duplicates by (server, port, protocol) inside a single batch are
// dropped so the UNIQUE INDEX in 0001_initial.sql is not breached.
func nodeInputsFromParsed(subscriptionID string, parsed []*ParsedNode) []NodeUpsertInput {
	out := make([]NodeUpsertInput, 0, len(parsed))
	seen := make(map[string]struct{}, len(parsed))
	for i, p := range parsed {
		if p == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d", p.Protocol, p.Server, p.Port)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, NodeUpsertInput{
			SubscriptionID: subscriptionID,
			RawURI:         "",
			Protocol:       p.Protocol,
			Server:         p.Server,
			Port:           p.Port,
			Tag:            p.Name,
			ParsedConfig:   p,
			Position:       i,
		})
	}
	return out
}
