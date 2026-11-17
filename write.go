package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib" // SQLITE_BUSY / SQLITE_LOCKED result code constants

	"github.com/larsartmann/go-error-family"
)

const (
	// busyRetries is how many times ExecWrite retries on SQLITE_BUSY from
	// external processes. With a 3s busy timeout, the total worst-case
	// stall is 3 x 3s = 9s.
	busyRetries = 3

	// busyRetryBase is the initial backoff for SQLITE_BUSY retries. Each
	// retry doubles the delay (100ms, 200ms, 400ms), giving external
	// processes time to release the write lock.
	busyRetryBase = 100 * time.Millisecond
)

// ExecWrite runs a write statement with the write mutex held and retries on
// SQLITE_BUSY/SQLITE_LOCKED errors from external-process contention. All
// write operations (INSERT, UPDATE, DELETE, VACUUM) must go through here
// instead of db.ExecContext.
//
// When debug SQL is enabled (SetDebugSQL), each statement is logged at
// Debug level before execution.
func (d *DB) ExecWrite(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if d == nil || d.db == nil {
		return nil, errorfamily.NewInfrastructure("sqlitestore.closed", "write on closed store")
	}

	d.writeMu.Lock()
	defer d.writeMu.Unlock()

	if d.debugSQL.Load() {
		slog.Debug("sqlitestore: ExecWrite", "query", TruncateSQL(query), "args", args)
	}

	var lastErr error

	for attempt := range busyRetries {
		result, err := d.db.ExecContext(ctx, query, args...)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !IsBusy(err) {
			return nil, err
		}

		d.busyRetries.Add(1)

		delay := busyRetryBase << attempt // 100ms, 200ms, 400ms

		slog.Debug(
			"sqlitestore: SQLITE_BUSY retry",
			"attempt", attempt+1,
			"max_attempts", busyRetries,
			"backoff_ms", delay.Milliseconds(),
			"error", err.Error(),
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	slog.Warn(
		"sqlitestore: SQLITE_BUSY exhausted all retries",
		"max_attempts", busyRetries,
		"error", lastErr.Error(),
	)

	return nil, lastErr
}

// WithWriteLock runs fn while holding the engine's write mutex, for
// multi-statement units (transactions, batch inserts) that must serialize
// as one write. Single statements should use ExecWrite instead.
func (d *DB) WithWriteLock(fn func() error) error {
	if d == nil || d.db == nil {
		return errorfamily.NewInfrastructure("sqlitestore.closed", "write lock on closed store")
	}

	d.writeMu.Lock()
	defer d.writeMu.Unlock()

	return fn()
}

// WriteLock acquires the engine's write mutex and returns its release
// function, for flows where a deferred unlock reads better than a closure.
// Multi-statement units (transactions) held across the lock still count as
// one serialized write.
func (d *DB) WriteLock() func() {
	d.writeMu.Lock()

	return d.writeMu.Unlock
}

// TruncateSQL truncates a SQL string for debug logging, keeping the first
// 200 characters so large INSERT payloads cannot flood the log.
func TruncateSQL(query string) string {
	if len(query) <= 200 {
		return query
	}

	return query[:200] + "..."
}

// IsBusy detects SQLITE_BUSY (result code 5) and SQLITE_LOCKED (result
// code 6) errors from the modernc.org/sqlite driver. The low byte is
// masked so extended result codes (SQLITE_BUSY_RECOVERY,
// SQLITE_LOCKED_SHAREDCACHE, etc.) are caught too.
func IsBusy(err error) bool {
	if err == nil {
		return false
	}

	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok {
		return false
	}

	primary := sqliteErr.Code() & 0xFF

	return primary == sqlite3.SQLITE_BUSY || primary == sqlite3.SQLITE_LOCKED
}
