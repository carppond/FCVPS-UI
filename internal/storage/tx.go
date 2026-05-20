package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// TxFn is the user-supplied callback for WithTx. The supplied *sql.Tx is
// already started; returning a non-nil error rolls back, otherwise commits.
type TxFn func(*sql.Tx) error

// WithTxOptions tunes retry behaviour for WithTx. Zero values fall back to
// sensible defaults (3 attempts, 50ms..200ms exponential backoff).
type WithTxOptions struct {
	// MaxAttempts caps total attempts including the first try. Must be >= 1.
	MaxAttempts int
	// MinBackoff is the lower bound for the first retry delay.
	MinBackoff time.Duration
	// MaxBackoff is the upper bound for any retry delay.
	MaxBackoff time.Duration
}

// defaultWithTxOpts is the production retry profile.
var defaultWithTxOpts = WithTxOptions{
	MaxAttempts: 3,
	MinBackoff:  50 * time.Millisecond,
	MaxBackoff:  200 * time.Millisecond,
}

// WithTx runs fn within a write transaction, retrying on SQLITE_BUSY /
// SQLITE_LOCKED. Reads should bypass this and use db.Read directly.
//
// fn must be idempotent: it may be invoked multiple times on retry. The
// transaction is rolled back automatically when fn returns an error.
func (db *DB) WithTx(ctx context.Context, fn TxFn) error {
	return db.WithTxOptions(ctx, defaultWithTxOpts, fn)
}

// WithTxOptions is WithTx but with explicit retry parameters. Used by tests
// to assert backoff behaviour without sleeping seconds.
func (db *DB) WithTxOptions(ctx context.Context, opts WithTxOptions, fn TxFn) error {
	if db == nil || db.Write == nil {
		return ErrDBClosed
	}
	if fn == nil {
		return fmt.Errorf("storage.WithTx: nil fn")
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultWithTxOpts.MaxAttempts
	}
	if opts.MinBackoff <= 0 {
		opts.MinBackoff = defaultWithTxOpts.MinBackoff
	}
	if opts.MaxBackoff < opts.MinBackoff {
		opts.MaxBackoff = opts.MinBackoff
	}

	var lastErr error
	for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("storage.WithTx: %w", err)
		}
		err := runOnce(ctx, db.Write, fn)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			return err
		}
		if attempt+1 >= opts.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("storage.WithTx: %w", ctx.Err())
		case <-time.After(backoffDuration(attempt, opts)):
		}
	}
	return fmt.Errorf("storage.WithTx: gave up after %d attempts: %w", opts.MaxAttempts, lastErr)
}

// runOnce begins a transaction, calls fn, and commits or rolls back as
// indicated. The transaction is always finalised before returning.
func runOnce(ctx context.Context, writer *sql.DB, fn TxFn) error {
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		// Commit may surface SQLITE_BUSY; propagate so caller can retry.
		_ = tx.Rollback()
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// isRetryable returns true when err is the kind of transient SQLite locking
// failure that benefits from a backoff retry. modernc.org/sqlite surfaces
// these as strings rather than typed sentinels.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "busy") || strings.Contains(msg, "locked")
}

// backoffDuration returns the duration for attempt index (0-based) using
// exponential backoff capped at opts.MaxBackoff with a small jitter.
func backoffDuration(attempt int, opts WithTxOptions) time.Duration {
	d := opts.MinBackoff << attempt
	if d > opts.MaxBackoff {
		d = opts.MaxBackoff
	}
	// Add up to 25% jitter to reduce thundering herd among readers.
	jitter := time.Duration(rand.Int64N(int64(d)/4 + 1))
	return d + jitter
}
