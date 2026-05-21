package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PipelineRecord is the storage-layer projection of a pipelines row. Handler
// code translates this into types.Pipeline.
//
// Tech Lead §1.4 ruling: AST (ast_json) is the system of record; YAML is
// regenerated from AST on every Update. The yaml_content column stays NOT
// NULL because migration 0001 already declared it — the repo keeps it
// populated to honour the schema, but readers should treat AST as canonical.
type PipelineRecord struct {
	ID            string
	UserID        string
	Name          string
	YAMLContent   string
	ASTJSON       string
	Version       int32
	SchemaVersion string
	CreatedAt     int64
	UpdatedAt     int64
}

// PipelineListOptions narrows / paginates the list query.
type PipelineListOptions struct {
	Page     int
	PageSize int
	Keyword  string // matched against name (LIKE %kw%)
}

// Sentinels.
var (
	// ErrPipelineNotFound is the canonical not-found sentinel for the
	// pipelines table.
	ErrPipelineNotFound = errors.New("storage: pipeline not found")

	// ErrPipelineVersionConflict is returned by Update when the supplied
	// version does not match the persisted one (optimistic locking).
	ErrPipelineVersionConflict = errors.New("storage: pipeline version conflict")
)

// PipelineRepo encapsulates SQL access to the pipelines + pipeline_bindings
// tables.
type PipelineRepo struct {
	db  *DB
	now func() time.Time
}

// NewPipelineRepo wires a repo to db. When now is nil, time.Now is used.
func NewPipelineRepo(db *DB, now func() time.Time) *PipelineRepo {
	if now == nil {
		now = time.Now
	}
	return &PipelineRepo{db: db, now: now}
}

// Create inserts a new pipelines row. CreatedAt / UpdatedAt are populated when
// zero. Version is normalised to 1 when zero. SchemaVersion defaults to
// shiguang/v1.
func (r *PipelineRepo) Create(ctx context.Context, rec PipelineRecord) (*PipelineRecord, error) {
	if rec.ID == "" || rec.UserID == "" || rec.Name == "" {
		return nil, fmt.Errorf("pipeline create: required field missing")
	}
	if rec.ASTJSON == "" || rec.YAMLContent == "" {
		return nil, fmt.Errorf("pipeline create: ast_json / yaml_content required")
	}
	if rec.SchemaVersion == "" {
		rec.SchemaVersion = "shiguang/v1"
	}
	if rec.Version == 0 {
		rec.Version = 1
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO pipelines(id, user_id, name, yaml_content, ast_json,
		                     version, schema_version, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.YAMLContent, rec.ASTJSON,
		rec.Version, rec.SchemaVersion, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert pipeline: %w", err)
	}
	return &rec, nil
}

// GetByID returns a pipeline row scoped to userID. The userID filter is
// mandatory — pipelines are per-user resources and a cross-user GET must
// behave like "not found" rather than expose the existence of the resource.
func (r *PipelineRepo) GetByID(ctx context.Context, id, userID string) (*PipelineRecord, error) {
	row := r.db.Read.QueryRowContext(ctx, selectPipelineSQL+
		` WHERE id = ? AND user_id = ?`, id, userID)
	return scanPipelineRow(row)
}

// List paginates pipelines for a user, newest first.
func (r *PipelineRepo) List(ctx context.Context, userID string, opts PipelineListOptions) ([]PipelineRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("pipeline list: empty user_id")
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}
	args := []any{userID}
	where := []string{"user_id = ?"}
	if opts.Keyword != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+opts.Keyword+"%")
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pipelines"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pipelines: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectPipelineSQL+clause+" ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list pipelines: %w", err)
	}
	defer rows.Close()
	out := make([]PipelineRecord, 0, opts.PageSize)
	for rows.Next() {
		p, err := scanPipelineRowMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate pipelines: %w", err)
	}
	return out, total, nil
}

// Update overwrites the row identified by rec.ID + rec.UserID with the new
// name / YAML / AST, bumps version, and refreshes updated_at. The supplied
// rec.Version is used for optimistic locking: when the persisted row's
// version differs, ErrPipelineVersionConflict is returned and no write
// occurs.
func (r *PipelineRepo) Update(ctx context.Context, rec PipelineRecord) error {
	if rec.ID == "" || rec.UserID == "" {
		return fmt.Errorf("pipeline update: empty id / user_id")
	}
	if rec.ASTJSON == "" || rec.YAMLContent == "" {
		return fmt.Errorf("pipeline update: ast_json / yaml_content required")
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE pipelines
		   SET name = ?, yaml_content = ?, ast_json = ?,
		       version = version + 1, updated_at = ?
		 WHERE id = ? AND user_id = ? AND version = ?`,
		rec.Name, rec.YAMLContent, rec.ASTJSON, now,
		rec.ID, rec.UserID, rec.Version,
	)
	if err != nil {
		return fmt.Errorf("update pipeline: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Could be "not found" or "version conflict" — disambiguate with a
		// follow-up read so the caller gets a precise sentinel.
		exists, err := r.exists(ctx, rec.ID, rec.UserID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrPipelineNotFound
		}
		return ErrPipelineVersionConflict
	}
	return nil
}

// Delete removes the pipelines row (cascade nukes pipeline_bindings rows).
func (r *PipelineRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("pipeline delete: empty id / user_id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM pipelines WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete pipeline: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrPipelineNotFound
	}
	return nil
}

// exists is a cheap "row present" probe used by Update for sentinel
// disambiguation.
func (r *PipelineRepo) exists(ctx context.Context, id, userID string) (bool, error) {
	var n int
	err := r.db.Read.QueryRowContext(ctx,
		"SELECT 1 FROM pipelines WHERE id = ? AND user_id = ?", id, userID).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("probe pipeline: %w", err)
	}
	return true, nil
}

const selectPipelineSQL = `SELECT id, user_id, name, yaml_content, ast_json,
		version, schema_version, created_at, updated_at FROM pipelines`

// scanPipelineRow drains a QueryRow result.
func scanPipelineRow(row *sql.Row) (*PipelineRecord, error) {
	var p PipelineRecord
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.YAMLContent, &p.ASTJSON,
		&p.Version, &p.SchemaVersion, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPipelineNotFound
		}
		return nil, fmt.Errorf("scan pipeline: %w", err)
	}
	return &p, nil
}

// scanPipelineRowMulti drains a rows.Next result.
func scanPipelineRowMulti(rows *sql.Rows) (*PipelineRecord, error) {
	var p PipelineRecord
	if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.YAMLContent, &p.ASTJSON,
		&p.Version, &p.SchemaVersion, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan pipeline: %w", err)
	}
	return &p, nil
}
