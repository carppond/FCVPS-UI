package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// NodeUpsertInput is the per-node payload accepted by NodeRepo.UpsertBatch.
// It is intentionally decoupled from the substore package to avoid an import
// cycle (substore depends on storage); the caller is responsible for
// translating substore.ParsedNode (or any other upstream representation) into
// this struct before invoking the repo.
type NodeUpsertInput struct {
	RawURI       string
	Protocol     string
	Server       string
	Port         int
	Tag          string
	Position     int
	ParsedConfig map[string]any // pre-marshalled config; nil/empty yields "{}".
}

// NodeUpsertResult summarises how many rows were inserted / refreshed /
// removed by a single UpsertBatch invocation.
type NodeUpsertResult struct {
	Added   int
	Updated int
	Removed int
	Total   int
}

// NodeRecord is the storage projection of a single nodes row. Tags decode
// from the TEXT JSON column into a []string for ergonomic callers;
// ParsedConfig holds the structured parser output (a generic map so the
// handler layer can re-marshal into the DTO without losing fields).
//
// LastLatencyMs / LastTestedAt carry the most recent TCPing measurement;
// a nil pointer means "never tested" (the column is NULL on disk).
type NodeRecord struct {
	ID             string
	SubscriptionID string
	UserID         string // joined from subscriptions for cross-table reads
	RawURI         string
	ParsedConfig   map[string]any
	Protocol       string
	Server         string
	Port           int32
	Tag            string
	Tags           []string
	IsChainProxy   bool
	ChainParentID  string
	Position       int32
	LastLatencyMs  *int32
	LastTestedAt   *int64
	CreatedAt      int64
	UpdatedAt      int64
}

// NodeListOptions narrows / paginates a ListByUser query.
type NodeListOptions struct {
	Page           int
	PageSize       int
	Search         string // matched against tag / server / raw_uri (LIKE %s%)
	Protocol       string // exact filter; "" disables
	Tag            string // membership test on tags JSON; "" disables
	SubscriptionID string // exact filter; "" disables
	Sort           string // "latency_asc" | "latency_desc" | "created_desc" (default)
}

// TCPingPersist is the per-node TCPing measurement persisted by
// BatchUpdateLatency. LatencyMs follows the same NULL/-1/N semantics as the
// NodeRecord field of the same name (callers pass -1 for unreachable).
type TCPingPersist struct {
	NodeID    string
	LatencyMs int32
	TestedAt  int64
}

// Sentinels.
var (
	// ErrNodeNotFound is the canonical not-found sentinel for the nodes table.
	ErrNodeNotFound = errors.New("storage: node not found")
)

// NodeRepo encapsulates SQL access to the nodes table.
//
// All read paths join subscriptions on subscription_id so cross-user lookups
// (id without subscription context) can still be safely scoped. The repo is
// safe for concurrent use; the underlying *sql.DB pools handle synchronisation.
type NodeRepo struct {
	db  *DB
	now func() time.Time
}

// NewNodeRepo wires a repo to db. When now is nil, time.Now is used.
func NewNodeRepo(db *DB, now func() time.Time) *NodeRepo {
	if now == nil {
		now = time.Now
	}
	return &NodeRepo{db: db, now: now}
}

// Create inserts a manual node row. The caller supplies a NodeRecord with at
// minimum SubscriptionID + Protocol + Server + Port + RawURI populated; ID and
// timestamps are filled in when zero.
//
// Manual creation collides with the (subscription_id, server, port, protocol)
// UNIQUE INDEX from 0001_initial.sql; that constraint surfaces as a wrapped
// SQL error which the handler maps to ErrConflictUsername (the closest 409
// code currently in the contract).
func (r *NodeRepo) Create(ctx context.Context, rec NodeRecord) (*NodeRecord, error) {
	if rec.SubscriptionID == "" {
		return nil, fmt.Errorf("node create: empty subscription_id")
	}
	if rec.Protocol == "" || rec.Server == "" || rec.Port == 0 {
		return nil, fmt.Errorf("node create: required field missing")
	}
	if rec.ID == "" {
		rec.ID = util.UUIDv7()
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	tagsJSON, err := encodeTags(rec.Tags)
	if err != nil {
		return nil, fmt.Errorf("node create: %w", err)
	}
	cfgJSON, err := encodeParsedConfig(rec.ParsedConfig)
	if err != nil {
		return nil, fmt.Errorf("node create: %w", err)
	}
	_, err = r.db.Write.ExecContext(ctx, `
		INSERT INTO nodes(
			id, subscription_id, raw_uri, parsed_config_json,
			protocol, server, port, tag, tags,
			is_chain_proxy, chain_parent_id, position,
			created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.SubscriptionID, rec.RawURI, cfgJSON,
		rec.Protocol, rec.Server, rec.Port, rec.Tag, tagsJSON,
		boolToInt(rec.IsChainProxy), nullableString(rec.ChainParentID), rec.Position,
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert node: %w", err)
	}
	return &rec, nil
}

// GetByID returns the node identified by id. When userID is non-empty the
// row is filtered to the owning subscription so cross-user reads 404.
//
// The join against subscriptions populates NodeRecord.UserID so handlers can
// reuse the cached value without a second query.
func (r *NodeRepo) GetByID(ctx context.Context, id, userID string) (*NodeRecord, error) {
	if id == "" {
		return nil, ErrNodeNotFound
	}
	query := selectNodeSQL + " WHERE n.id = ?"
	args := []any{id}
	if userID != "" {
		query += " AND s.user_id = ?"
		args = append(args, userID)
	}
	row := r.db.Read.QueryRowContext(ctx, query, args...)
	return scanNodeRow(row)
}

// ListBySubscription returns every node belonging to subID, ordered by
// position then created_at. Empty result is non-error.
func (r *NodeRepo) ListBySubscription(ctx context.Context, subID string) ([]NodeRecord, error) {
	if subID == "" {
		return nil, nil
	}
	rows, err := r.db.Read.QueryContext(ctx,
		selectNodeSQL+" WHERE n.subscription_id = ? ORDER BY n.position ASC, n.created_at ASC",
		subID)
	if err != nil {
		return nil, fmt.Errorf("list nodes by subscription: %w", err)
	}
	defer rows.Close()
	return scanNodeRows(rows)
}

// ListByUser paginates nodes across every subscription owned by userID, with
// optional filters / search / sort applied.
func (r *NodeRepo) ListByUser(ctx context.Context, userID string, opts NodeListOptions) ([]NodeRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("node list: empty user id")
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 200 {
		opts.PageSize = 200
	}
	where := []string{"s.user_id = ?"}
	args := []any{userID}
	if opts.SubscriptionID != "" {
		where = append(where, "n.subscription_id = ?")
		args = append(args, opts.SubscriptionID)
	}
	if opts.Protocol != "" {
		where = append(where, "n.protocol = ?")
		args = append(args, opts.Protocol)
	}
	if opts.Search != "" {
		where = append(where, "(n.tag LIKE ? OR n.server LIKE ? OR n.raw_uri LIKE ?)")
		kw := "%" + opts.Search + "%"
		args = append(args, kw, kw, kw)
	}
	if opts.Tag != "" {
		// JSON membership test using LIKE; tag values are JSON-stringified.
		// We bracket with quotes to avoid prefix matches against substrings.
		where = append(where, "n.tags LIKE ?")
		args = append(args, "%\""+opts.Tag+"\"%")
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM nodes n JOIN subscriptions s ON s.id = n.subscription_id"+clause,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count nodes: %w", err)
	}

	orderBy := " ORDER BY n.created_at DESC"
	switch opts.Sort {
	case "latency_asc":
		// NULL latencies (untested) sink to the bottom of asc-sorted lists.
		orderBy = " ORDER BY n.last_latency_ms IS NULL, n.last_latency_ms ASC, n.created_at DESC"
	case "latency_desc":
		orderBy = " ORDER BY n.last_latency_ms IS NULL, n.last_latency_ms DESC, n.created_at DESC"
	case "created_asc":
		orderBy = " ORDER BY n.created_at ASC"
	}

	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectNodeSQL+clause+orderBy+" LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list nodes by user: %w", err)
	}
	defer rows.Close()
	out, err := scanNodeRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// NodeUpdate captures the fields a PATCH-style update may touch. Nil pointers
// leave the column untouched; non-nil values overwrite. Tags uses a pointer
// to allow distinguishing "clear" (empty slice) from "leave alone" (nil).
type NodeUpdate struct {
	Tag           *string
	Tags          *[]string
	ChainParentID *string // empty string clears the FK
	IsChainProxy  *bool
}

// Update applies a partial change to the node identified by (id, userID).
// Returns ErrNodeNotFound when nothing matched.
func (r *NodeRepo) Update(ctx context.Context, id, userID string, upd NodeUpdate) error {
	if id == "" {
		return fmt.Errorf("node update: empty id")
	}
	sets := []string{}
	args := []any{}
	if upd.Tag != nil {
		sets = append(sets, "tag = ?")
		args = append(args, *upd.Tag)
	}
	if upd.Tags != nil {
		tagsJSON, err := encodeTags(*upd.Tags)
		if err != nil {
			return fmt.Errorf("node update: %w", err)
		}
		sets = append(sets, "tags = ?")
		args = append(args, tagsJSON)
	}
	if upd.ChainParentID != nil {
		sets = append(sets, "chain_parent_id = ?")
		args = append(args, nullableString(*upd.ChainParentID))
	}
	if upd.IsChainProxy != nil {
		sets = append(sets, "is_chain_proxy = ?")
		args = append(args, boolToInt(*upd.IsChainProxy))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, r.now().UnixMilli())

	// Cross-user safety: only touch the row when the owning subscription
	// belongs to userID. Implemented as a sub-query so the UPDATE stays
	// simple (SQLite doesn't support JOIN in UPDATE).
	stmt := "UPDATE nodes SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	if userID != "" {
		stmt += " AND subscription_id IN (SELECT id FROM subscriptions WHERE user_id = ?)"
		args = append(args, userID)
	}
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// Delete removes the row identified by (id, userID). The FK on
// chain_parent_id is ON DELETE SET NULL, so referencing rows survive with a
// cleared link.
func (r *NodeRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" {
		return fmt.Errorf("node delete: empty id")
	}
	stmt := "DELETE FROM nodes WHERE id = ?"
	args := []any{id}
	if userID != "" {
		stmt += " AND subscription_id IN (SELECT id FROM subscriptions WHERE user_id = ?)"
		args = append(args, userID)
	}
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// UpsertBatch reconciles the desired set of nodes for a given subscription:
// new entries are inserted, existing entries are refreshed (server, port,
// protocol, parsed_config, tag, position), and stale entries — those absent
// from the new batch — are deleted.
//
// Identity is keyed by sha256(raw_uri) so callers don't have to maintain
// stable IDs across sync runs. Two rows whose raw_uri hashes match are
// considered the same logical node; this matches the spec's
// (subscription_id, raw_uri hash) uniqueness invariant. When the input has no
// raw_uri (manual create path), the (protocol, server, port) triple is used
// instead so the dedupe still works.
//
// The function is wrapped in a single transaction; partial failures roll
// back. The returned counts mirror substore.UpsertResult.
func (r *NodeRepo) UpsertBatch(ctx context.Context, subID string, nodes []NodeUpsertInput) (NodeUpsertResult, error) {
	if subID == "" {
		return NodeUpsertResult{}, fmt.Errorf("upsert: empty subscription_id")
	}
	tx, err := r.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return NodeUpsertResult{}, fmt.Errorf("upsert: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Load existing rows keyed by dedupe key.
	existing, err := r.loadExistingDedupe(ctx, tx, subID)
	if err != nil {
		return NodeUpsertResult{}, err
	}

	now := r.now().UnixMilli()
	var added, updated int
	seen := make(map[string]struct{}, len(nodes))
	for i, in := range nodes {
		key := nodeDedupeKey(in)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		tagsJSON, _ := encodeTags(nil)
		cfgJSON, err := encodeParsedConfig(in.ParsedConfig)
		if err != nil {
			return NodeUpsertResult{}, fmt.Errorf("upsert: encode config: %w", err)
		}
		pos := int32(in.Position)
		if pos == 0 {
			pos = int32(i)
		}
		if prevID, ok := existing[key]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE nodes SET
					raw_uri = ?, parsed_config_json = ?,
					protocol = ?, server = ?, port = ?, tag = ?,
					position = ?, updated_at = ?
				WHERE id = ?`,
				in.RawURI, cfgJSON,
				in.Protocol, in.Server, in.Port, in.Tag,
				pos, now, prevID,
			); err != nil {
				return NodeUpsertResult{}, fmt.Errorf("upsert: update %s: %w", prevID, err)
			}
			delete(existing, key)
			updated++
			continue
		}
		newID := util.UUIDv7()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO nodes(
				id, subscription_id, raw_uri, parsed_config_json,
				protocol, server, port, tag, tags,
				is_chain_proxy, chain_parent_id, position,
				created_at, updated_at
			) VALUES(?,?,?,?,?,?,?,?,?,0,NULL,?,?,?)`,
			newID, subID, in.RawURI, cfgJSON,
			in.Protocol, in.Server, in.Port, in.Tag, tagsJSON,
			pos, now, now,
		); err != nil {
			return NodeUpsertResult{}, fmt.Errorf("upsert: insert: %w", err)
		}
		added++
	}

	// Delete leftovers (rows that were present before but not in this batch).
	removed := 0
	for _, leftoverID := range existing {
		if _, err := tx.ExecContext(ctx, "DELETE FROM nodes WHERE id = ?", leftoverID); err != nil {
			return NodeUpsertResult{}, fmt.Errorf("upsert: delete %s: %w", leftoverID, err)
		}
		removed++
	}

	if err := tx.Commit(); err != nil {
		return NodeUpsertResult{}, fmt.Errorf("upsert: commit: %w", err)
	}
	return NodeUpsertResult{
		Added:   added,
		Updated: updated,
		Removed: removed,
		Total:   added + updated,
	}, nil
}

// BatchUpdateLatency persists a slice of TCPing measurements. NULL test
// results are not represented (callers pass -1 for unreachable). The query
// uses CASE expressions so all rows are updated in a single statement.
//
// IDs not present in the nodes table are silently skipped — the caller may
// have stale IDs after a parallel delete.
func (r *NodeRepo) BatchUpdateLatency(ctx context.Context, results []TCPingPersist) error {
	if len(results) == 0 {
		return nil
	}
	// Build a parameterised UPDATE per row; SQLite handles this fine inside a
	// single transaction and avoids the CASE/IN combinatorial of a bulk
	// UPDATE. The transaction also ensures atomicity for the dashboard read.
	tx, err := r.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("batch latency: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, res := range results {
		if res.NodeID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE nodes SET last_latency_ms = ?, last_tested_at = ?, updated_at = ? WHERE id = ?",
			res.LatencyMs, res.TestedAt, res.TestedAt, res.NodeID,
		); err != nil {
			return fmt.Errorf("batch latency: update %s: %w", res.NodeID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("batch latency: commit: %w", err)
	}
	return nil
}

// loadExistingDedupe returns a map from dedupe key → node id covering every
// row currently associated with subID. Used by UpsertBatch.
func (r *NodeRepo) loadExistingDedupe(ctx context.Context, tx *sql.Tx, subID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id, raw_uri, protocol, server, port FROM nodes WHERE subscription_id = ?",
		subID,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert: load existing: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var (
			id       string
			rawURI   sql.NullString
			protocol string
			server   string
			port     int
		)
		if err := rows.Scan(&id, &rawURI, &protocol, &server, &port); err != nil {
			return nil, fmt.Errorf("upsert: scan existing: %w", err)
		}
		key := rawDedupeKey(rawURI.String, protocol, server, port)
		out[key] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("upsert: iterate existing: %w", err)
	}
	return out, nil
}

// nodeDedupeKey returns the canonical key for an incoming upsert input.
func nodeDedupeKey(in NodeUpsertInput) string {
	return rawDedupeKey(in.RawURI, in.Protocol, in.Server, in.Port)
}

// rawDedupeKey computes sha256(raw_uri) when a URI is present, otherwise
// falls back to (protocol|server|port). This guarantees stable identity for
// rows imported from subscriptions (raw_uri available) as well as rows
// inserted manually (raw_uri empty).
func rawDedupeKey(rawURI, protocol, server string, port int) string {
	if rawURI != "" {
		sum := sha256.Sum256([]byte(rawURI))
		return "u:" + hex.EncodeToString(sum[:])
	}
	return fmt.Sprintf("t:%s|%s|%d", protocol, server, port)
}

// selectNodeSQL is the shared SELECT prefix; column order must match
// scanNodeRow / scanNodeRows.
const selectNodeSQL = `SELECT n.id, n.subscription_id, s.user_id,
		COALESCE(n.raw_uri,''), COALESCE(n.parsed_config_json,''),
		n.protocol, n.server, n.port, n.tag, n.tags,
		n.is_chain_proxy, COALESCE(n.chain_parent_id,''),
		n.position, n.last_latency_ms, n.last_tested_at,
		n.created_at, n.updated_at
	FROM nodes n
	JOIN subscriptions s ON s.id = n.subscription_id`

// scanNodeRow consumes a single QueryRow result.
func scanNodeRow(row *sql.Row) (*NodeRecord, error) {
	var rec NodeRecord
	var (
		tagsJSON  string
		cfgJSON   string
		chainParent string
		isChain   int
		latency   sql.NullInt32
		tested    sql.NullInt64
	)
	if err := row.Scan(
		&rec.ID, &rec.SubscriptionID, &rec.UserID,
		&rec.RawURI, &cfgJSON,
		&rec.Protocol, &rec.Server, &rec.Port, &rec.Tag, &tagsJSON,
		&isChain, &chainParent,
		&rec.Position, &latency, &tested,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNodeNotFound
		}
		return nil, fmt.Errorf("scan node: %w", err)
	}
	if err := finaliseNodeRecord(&rec, cfgJSON, tagsJSON, chainParent, isChain, latency, tested); err != nil {
		return nil, err
	}
	return &rec, nil
}

// scanNodeRows consumes a *sql.Rows iterator. Closes the iterator's resources
// only on error; the caller is responsible for the defer.
func scanNodeRows(rows *sql.Rows) ([]NodeRecord, error) {
	out := make([]NodeRecord, 0, 16)
	for rows.Next() {
		var rec NodeRecord
		var (
			tagsJSON    string
			cfgJSON     string
			chainParent string
			isChain     int
			latency     sql.NullInt32
			tested      sql.NullInt64
		)
		if err := rows.Scan(
			&rec.ID, &rec.SubscriptionID, &rec.UserID,
			&rec.RawURI, &cfgJSON,
			&rec.Protocol, &rec.Server, &rec.Port, &rec.Tag, &tagsJSON,
			&isChain, &chainParent,
			&rec.Position, &latency, &tested,
			&rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}
		if err := finaliseNodeRecord(&rec, cfgJSON, tagsJSON, chainParent, isChain, latency, tested); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return out, nil
}

// finaliseNodeRecord decodes JSON columns + materialises pointer fields after
// the raw column values are pulled from the database.
func finaliseNodeRecord(rec *NodeRecord, cfgJSON, tagsJSON, chainParent string, isChain int, latency sql.NullInt32, tested sql.NullInt64) error {
	tags, err := decodeTags(tagsJSON)
	if err != nil {
		return fmt.Errorf("decode tags: %w", err)
	}
	rec.Tags = tags
	rec.ParsedConfig = decodeParsedConfig(cfgJSON)
	rec.IsChainProxy = isChain != 0
	rec.ChainParentID = chainParent
	if latency.Valid {
		v := latency.Int32
		rec.LastLatencyMs = &v
	}
	if tested.Valid {
		v := tested.Int64
		rec.LastTestedAt = &v
	}
	return nil
}

// encodeParsedConfig serialises the structured config map to JSON. nil/empty
// becomes "{}" so the NOT NULL constraint on parsed_config_json holds.
func encodeParsedConfig(cfg map[string]any) (string, error) {
	if len(cfg) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal parsed_config: %w", err)
	}
	return string(b), nil
}

// decodeParsedConfig parses the JSON column. Empty / "{}" returns an empty
// map so JSON responses are always object-shaped. Malformed JSON returns nil
// so the caller can still surface the node with a missing config.
func decodeParsedConfig(raw string) map[string]any {
	if raw == "" || raw == "{}" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if out == nil {
		return map[string]any{}
	}
	return out
}

// NodeRecordToDTO projects the storage record to the API DTO. Exported so
// the handler layer can reuse the mapping without re-encoding.
func NodeRecordToDTO(rec *NodeRecord) types.Node {
	if rec == nil {
		return types.Node{}
	}
	dto := types.Node{
		ID:             rec.ID,
		SubscriptionID: rec.SubscriptionID,
		RawURI:         rec.RawURI,
		Protocol:       types.NodeProtocol(rec.Protocol),
		Server:         rec.Server,
		Port:           rec.Port,
		Tag:            rec.Tag,
		Tags:           rec.Tags,
		IsChainProxy:   rec.IsChainProxy,
		ChainParentID:  rec.ChainParentID,
		ParsedConfig:   rec.ParsedConfig,
		Position:       rec.Position,
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}
	if dto.Tags == nil {
		dto.Tags = []string{}
	}
	if dto.ParsedConfig == nil {
		dto.ParsedConfig = map[string]any{}
	}
	return dto
}

// NodeRecordToDTOWithLatency wraps NodeRecordToDTO and folds in the latency
// fields. When the row has never been tested (LastTestedAt nil) the response
// reports latency=-1 + reachable=false + tested_at=0.
func NodeRecordToDTOWithLatency(rec *NodeRecord) types.NodeWithLatency {
	base := NodeRecordToDTO(rec)
	out := types.NodeWithLatency{Node: base}
	if rec == nil {
		out.LatencyMs = -1
		return out
	}
	if rec.LastLatencyMs == nil {
		out.LatencyMs = -1
		return out
	}
	out.LatencyMs = *rec.LastLatencyMs
	out.Reachable = *rec.LastLatencyMs >= 0
	if rec.LastTestedAt != nil {
		out.TestedAt = *rec.LastTestedAt
	}
	return out
}
