package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// UserRecord is the storage-layer projection of a users row. Callers in the
// auth / handler layers translate it into the appropriate DTO (auth.StoredUser,
// types.User, types.UserPublicProfile).
type UserRecord struct {
	ID                string
	Username          string
	PasswordHash      string
	Role              string
	IsActive          bool
	Email             string
	Locale            string
	TOTPSecret        string
	TOTPEnabled       bool
	RecoveryCodesHash string
	CreatedAt         int64
	UpdatedAt         int64
}

// UserListOptions narrows / paginates the list query.
type UserListOptions struct {
	Page     int
	PageSize int
	Keyword  string // matched against username (LIKE %kw%)
	Role     string // optional exact filter
}

// ErrUserNotFound is the canonical not-found sentinel returned by Get* /
// Update* / Delete when the target row does not exist.
var ErrUserNotFound = errors.New("storage: user not found")

// ErrUsernameTaken is returned by Create / Update when the requested username
// conflicts with an existing row.
var ErrUsernameTaken = errors.New("storage: username taken")

// UserRepo encapsulates SQL access to the users table.
type UserRepo struct {
	db  *DB
	now func() time.Time
}

// NewUserRepo wires a repo to db. When now is nil, time.Now is used.
func NewUserRepo(db *DB, now func() time.Time) *UserRepo {
	if now == nil {
		now = time.Now
	}
	return &UserRepo{db: db, now: now}
}

// Create inserts a new users row. The supplied record's CreatedAt / UpdatedAt
// are populated when zero. Returns the persisted record (with id/timestamps
// filled in).
func (r *UserRepo) Create(ctx context.Context, rec UserRecord) (*UserRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("user create: empty id")
	}
	if rec.Username == "" || rec.PasswordHash == "" || rec.Role == "" {
		return nil, fmt.Errorf("user create: required field missing")
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	if rec.Locale == "" {
		rec.Locale = "zh-CN"
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO users(id, username, password_hash, role, is_active, email,
		                  locale, totp_secret, totp_enabled, recovery_codes_hash,
		                  created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.Username, rec.PasswordHash, rec.Role, boolToInt(rec.IsActive),
		nullableString(rec.Email), rec.Locale, nullableString(rec.TOTPSecret),
		boolToInt(rec.TOTPEnabled), nullableString(rec.RecoveryCodesHash),
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "users.username") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &rec, nil
}

// GetByID returns the user identified by id, or ErrUserNotFound.
func (r *UserRepo) GetByID(ctx context.Context, id string) (*UserRecord, error) {
	row := r.db.Read.QueryRowContext(ctx, selectUserSQL+` WHERE id = ?`, id)
	return scanUserRow(row)
}

// GetByUsername returns the user with the matching username (case-sensitive),
// or ErrUserNotFound.
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*UserRecord, error) {
	row := r.db.Read.QueryRowContext(ctx, selectUserSQL+` WHERE username = ?`, username)
	return scanUserRow(row)
}

// List paginates users with optional keyword (LIKE on username) and role
// filters. Returns the page slice + total row count.
func (r *UserRepo) List(ctx context.Context, opts UserListOptions) ([]UserRecord, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}
	args := []any{}
	where := []string{}
	if opts.Keyword != "" {
		where = append(where, "username LIKE ?")
		args = append(args, "%"+opts.Keyword+"%")
	}
	if opts.Role != "" {
		where = append(where, "role = ?")
		args = append(args, opts.Role)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM users"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectUserSQL+clause+" ORDER BY created_at DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := make([]UserRecord, 0, opts.PageSize)
	for rows.Next() {
		u, err := scanUserRowMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}
	return out, total, nil
}

// Delete removes the users row + cascades via foreign keys. Returns
// ErrUserNotFound when no row matches.
func (r *UserRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.Write.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateProfile changes the mutable profile fields. Empty arguments (except
// the ID) leave the corresponding column untouched.
func (r *UserRepo) UpdateProfile(ctx context.Context, id, username, email, locale string) error {
	if id == "" {
		return fmt.Errorf("update profile: empty id")
	}
	sets := []string{}
	args := []any{}
	if username != "" {
		sets = append(sets, "username = ?")
		args = append(args, username)
	}
	if email != "" {
		sets = append(sets, "email = ?")
		args = append(args, email)
	}
	if locale != "" {
		sets = append(sets, "locale = ?")
		args = append(args, locale)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, r.now().UnixMilli(), id)
	stmt := "UPDATE users SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		if isUniqueViolation(err, "users.username") {
			return ErrUsernameTaken
		}
		return fmt.Errorf("update profile: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdatePassword sets password_hash.
func (r *UserRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	return r.touch(ctx, id, "password_hash = ?", hash)
}

// UpdateTOTPSecret persists the (un-confirmed) TOTP secret. Does NOT enable.
func (r *UserRepo) UpdateTOTPSecret(ctx context.Context, id, secret string) error {
	return r.touch(ctx, id, "totp_secret = ?", nullableString(secret))
}

// EnableTOTP sets totp_enabled = 1 (Secret must already be persisted).
func (r *UserRepo) EnableTOTP(ctx context.Context, id string) error {
	return r.touch(ctx, id, "totp_enabled = 1", nil)
}

// DisableTOTP clears totp_enabled and totp_secret.
func (r *UserRepo) DisableTOTP(ctx context.Context, id string) error {
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx,
		`UPDATE users SET totp_enabled = 0, totp_secret = NULL,
		                  recovery_codes_hash = NULL, updated_at = ?
		 WHERE id = ?`, now, id)
	if err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateRecoveryCodes overwrites the JSON-encoded sha256 hashes column. Pass
// "" to clear the codes.
func (r *UserRepo) UpdateRecoveryCodes(ctx context.Context, id, hashesJSON string) error {
	return r.touch(ctx, id, "recovery_codes_hash = ?", nullableString(hashesJSON))
}

// GetRecoveryCodesHash reads the JSON-encoded sha256 hashes column for the
// recovery_codes verifier. Empty string returned for NULL.
func (r *UserRepo) GetRecoveryCodesHash(ctx context.Context, id string) (string, error) {
	var v sql.NullString
	err := r.db.Read.QueryRowContext(ctx,
		"SELECT recovery_codes_hash FROM users WHERE id = ?", id).Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("load recovery_codes_hash: %w", err)
	}
	return v.String, nil
}

// UpdateRole changes the role column.
func (r *UserRepo) UpdateRole(ctx context.Context, id, role string) error {
	return r.touch(ctx, id, "role = ?", role)
}

// SetActive toggles is_active.
func (r *UserRepo) SetActive(ctx context.Context, id string, active bool) error {
	return r.touch(ctx, id, "is_active = ?", boolToInt(active))
}

// CountAdmins returns the number of users with role='admin'. Used by
// EnsureAdmin during startup.
func (r *UserRepo) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// touch issues a single-column UPDATE that also bumps updated_at.
//
// When extra is nil the SQL fragment is taken as-is (e.g. "totp_enabled = 1"),
// otherwise it's bound to that argument.
func (r *UserRepo) touch(ctx context.Context, id, fragment string, extra any) error {
	now := r.now().UnixMilli()
	args := []any{}
	if extra != nil {
		args = append(args, extra)
	}
	args = append(args, now, id)
	stmt := "UPDATE users SET " + fragment + ", updated_at = ? WHERE id = ?"
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// selectUserSQL is the shared SELECT prefix used by GetByID / GetByUsername /
// List. Keeping it in one constant guarantees column order matches scanUserRow.
const selectUserSQL = `SELECT id, username, password_hash, role, is_active,
		COALESCE(email,''), locale, COALESCE(totp_secret,''),
		totp_enabled, COALESCE(recovery_codes_hash,''),
		created_at, updated_at FROM users`

// scanUserRow consumes a single QueryRow result.
func scanUserRow(row *sql.Row) (*UserRecord, error) {
	var u UserRecord
	var active, totpOn int
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &active,
		&u.Email, &u.Locale, &u.TOTPSecret, &totpOn, &u.RecoveryCodesHash,
		&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.IsActive = active == 1
	u.TOTPEnabled = totpOn == 1
	return &u, nil
}

// scanUserRowMulti is the rows.Next variant used by List.
func scanUserRowMulti(rows *sql.Rows) (*UserRecord, error) {
	var u UserRecord
	var active, totpOn int
	if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &active,
		&u.Email, &u.Locale, &u.TOTPSecret, &totpOn, &u.RecoveryCodesHash,
		&u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.IsActive = active == 1
	u.TOTPEnabled = totpOn == 1
	return &u, nil
}

// nullableString converts an empty string into sql.NullString{} so the column
// stays NULL in DB (NULL is meaningful for email / totp_secret).
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// boolToInt projects bool to 1/0 for SQLite INTEGER columns.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isUniqueViolation reports whether err originated from a UNIQUE constraint
// violation on the named index / column. modernc.org/sqlite surfaces these
// as strings — there is no typed sentinel.
func isUniqueViolation(err error, marker string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") && strings.Contains(msg, strings.ToLower(marker))
}
