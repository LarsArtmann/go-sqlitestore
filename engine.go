package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib" // SQLITE_BUSY / SQLITE_LOCKED result code constants

	"github.com/larsartmann/go-error-family"
)

const (
	// driverName is the modernc.org/sqlite driver registration name.
	driverName = "sqlite"

	// defaultBusyTimeoutMS is the SQLite busy timeout in milliseconds.
	// Each write attempt waits up to this long for the lock before
	// returning SQLITE_BUSY. ExecWrite retries 3x with exponential
	// backoff, so the total worst-case stall per write is
	// busyTimeout x 3. 3s (not 10s) caps the retry cycle at ~9s;
	// with concurrent read connections, reads never block behind
	// writes regardless of this value.
	defaultBusyTimeoutMS = 3000

	// permDir is the directory permission for the database parent directory.
	permDir = 0o755
)

// DefaultBusyTimeoutMS is the busy timeout used by BuildDSN and Open when
// no WithBusyTimeout option overrides it.
const DefaultBusyTimeoutMS = defaultBusyTimeoutMS

// DB is the engine core over one SQLite database: it owns the write mutex,
// the SQLITE_BUSY retry policy, and the busy-retry/debug counters.
//
// DB is safe for concurrent use.
type DB struct {
	db *sql.DB

	// writeMu serializes all write operations at the Go level, preventing
	// SQLITE_BUSY contention between concurrent goroutines that share this
	// database.
	writeMu sync.Mutex

	// busyRetries counts total SQLITE_BUSY retries across the database's
	// lifetime. Atomic for lock-free reads from telemetry goroutines.
	busyRetries atomic.Int64

	// debugSQL, when true, enables slog.Debug logging of every statement
	// executed via ExecWrite.
	debugSQL atomic.Bool
}

// Option configures Open/New.
type Option func(*config)

type config struct {
	busyTimeoutMS  int
	maxOpenConns   int
	applyConnLimit bool
}

func defaultConfig() config {
	return config{busyTimeoutMS: defaultBusyTimeoutMS}
}

// WithBusyTimeout overrides the SQLite busy timeout in milliseconds.
func WithBusyTimeout(ms int) Option {
	return func(c *config) { c.busyTimeoutMS = ms }
}

// WithMaxOpenConns caps the connection pool. Zero (the default) picks
// max(4, min(NumCPU, 8)): enough concurrent read connections that reads
// never queue behind a retrying write, without oversubscribing small
// machines.
func WithMaxOpenConns(n int) Option {
	return func(c *config) {
		c.maxOpenConns = n
		c.applyConnLimit = true
	}
}

// Open creates or opens a SQLite database at path (parent directory is
// created), applies WAL + optimization PRAGMAs and the connection pool
// size, and returns the engine core. The caller must Close when done.
//
// Migrations are NOT applied here — run RunMigrations explicitly so the
// caller controls the migration list (state and cache databases carry
// different schemas).
func Open(path string, opts ...Option) (*DB, error) {
	sqlDB, err := OpenDB(path, opts...)
	if err != nil {
		return nil, err
	}

	return New(sqlDB, opts...), nil
}

// OpenDB opens the raw *sql.DB handle with the engine's DSN (per-connection
// PRAGMAs), WAL mode, optimization PRAGMAs, and pool size. For consumers
// that own the handle and query surface but want the engine's setup
// mechanics. Pair with New to get the write-mutex core.
func OpenDB(path string, opts ...Option) (*sql.DB, error) {
	cfg := defaultConfig()

	for _, opt := range opts {
		opt(&cfg)
	}

	return openDBWithTimeout(path, cfg)
}

func openDBWithTimeout(path string, cfg config) (*sql.DB, error) {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, permDir); err != nil {
		return nil, errorfamily.WrapInfrastructure(err, "sqlitestore.mkdir",
			"failed to create database directory "+dir)
	}

	db, err := sql.Open(driverName, BuildDSNWithBusyTimeout(path, cfg.busyTimeoutMS))
	if err != nil {
		return nil, errorfamily.WrapInfrastructure(err, "sqlitestore.open",
			"failed to open database "+path)
	}

	ctx := context.Background()

	if err := EnableWAL(ctx, db, cfg.busyTimeoutMS); err != nil {
		closeQuietly(db)

		return nil, err
	}

	if err := ApplyOptimizations(ctx, db); err != nil {
		closeQuietly(db)

		return nil, err
	}
	maxConns := cfg.maxOpenConns
	if !cfg.applyConnLimit {
		maxConns = max(4, min(runtime.NumCPU(), 8))
	}

	db.SetMaxOpenConns(maxConns)

	return db, nil
}

// New wraps an existing *sql.DB with the engine's write mutex and retry
// core. Use for handles opened via OpenDB or already circulating in a
// consumer.
func New(db *sql.DB, opts ...Option) *DB {
	cfg := defaultConfig()

	for _, opt := range opts {
		opt(&cfg)
	}

	core := &DB{db: db}
	core.debugSQL.Store(false)

	return core
}

// BuildDSN constructs a DSN for path with the engine's default busy
// timeout and the standard per-connection PRAGMA parameters.
func BuildDSN(path string) string {
	return BuildDSNWithBusyTimeout(path, DefaultBusyTimeoutMS)
}

// BuildDSNWithBusyTimeout constructs a DSN with per-connection PRAGMA
// parameters. Every connection in the pool inherits these PRAGMAs
// automatically, which is essential once the pool has more than one
// connection: without DSN-level PRAGMAs, new connections would use SQLite
// defaults (rollback journal, busy_timeout=0).
//
// The modernc.org/sqlite driver applies shorthand keys (_busy_timeout,
// _journal_mode, etc.) in a fixed order before _pragma values, ensuring
// auto_vacuum is set before the first write.
func BuildDSNWithBusyTimeout(path string, busyTimeoutMS int) string {
	return path +
		"?_busy_timeout=" + strconv.Itoa(busyTimeoutMS) +
		"&_journal_mode=wal" +
		"&_synchronous=NORMAL" +
		"&_auto_vacuum=INCREMENTAL" +
		"&_pragma=wal_autocheckpoint(1000)" +
		"&_pragma=cache_size(-65536)" +
		"&_pragma=temp_store(MEMORY)" +
		"&_pragma=mmap_size(268435456)"
}

// EnableWAL configures write-ahead logging for crash safety and concurrent
// read access. synchronous=NORMAL is 3-10x faster than FULL while remaining
// safe against power loss under WAL. wal_autocheckpoint=1000 keeps the WAL
// file bounded (~1000 pages) during long-running workloads.
func EnableWAL(ctx context.Context, db *sql.DB, busyTimeoutMS int) error {
	return applyPragmas(ctx, db, []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		fmt.Sprintf("PRAGMA busy_timeout=%d", busyTimeoutMS),
		"PRAGMA wal_autocheckpoint=1000",
	}, "sqlitestore.wal_pragma")
}

// ApplyOptimizations sets performance-oriented PRAGMAs. These trade memory
// for speed — appropriate for a local debug/analytics store.
func ApplyOptimizations(ctx context.Context, db *sql.DB) error {
	return applyPragmas(ctx, db, []string{
		"PRAGMA auto_vacuum = INCREMENTAL", // enable incremental vacuum for new DBs
		"PRAGMA cache_size=-65536",         // 64 MB page cache
		"PRAGMA temp_store=MEMORY",         // avoid temp files on disk
		"PRAGMA mmap_size=268435456",       // 256 MB memory-mapped I/O
	}, "sqlitestore.optimize_pragma")
}

// applyPragmas runs a list of PRAGMA statements in order, returning the
// first error encountered wrapped under opPrefix.
func applyPragmas(ctx context.Context, db *sql.DB, pragmas []string, opPrefix string) error {
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return errorfamily.WrapTransient(err, opPrefix,
				"failed to apply pragma: "+pragma)
		}
	}

	return nil
}

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

// BusyRetryCount returns the total number of SQLITE_BUSY retries across
// this database's lifetime, for telemetry and write-path diagnostics.
func (d *DB) BusyRetryCount() int64 {
	return d.busyRetries.Load()
}

// SetDebugSQL enables or disables Debug-level logging of every statement
// executed via ExecWrite. Diagnostic use only — do not leave enabled.
func (d *DB) SetDebugSQL(enabled bool) {
	d.debugSQL.Store(enabled)
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

// DB returns the underlying handle for read queries. Writes must go through
// ExecWrite to inherit the mutex and retry policy.
func (d *DB) DB() *sql.DB { return d.db }

// Close closes the underlying handle.
func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}

	return d.db.Close()
}

// HealthCheck verifies the database responds to a trivial query. Suitable
// for DI health-check wiring.
func (d *DB) HealthCheck(ctx context.Context) error {
	if d == nil || d.db == nil {
		return errorfamily.NewInfrastructure("sqlitestore.closed", "health check on closed store")
	}

	if err := d.db.PingContext(ctx); err != nil {
		return errorfamily.WrapTransient(err, "sqlitestore.ping",
			"database ping failed")
	}

	return nil
}

// PurgeByExpiry deletes rows from tableName whose timeColumn is non-null
// and older than now, returning the count of deleted rows.
//
// The timeColumn parameter makes the implicit column assumption EXPLICIT:
// a helper that hardcodes one column name silently purges nothing for any
// table using a different time column. tableName and timeColumn MUST be
// trusted compile-time constants — they are interpolated as SQL
// identifiers, never bound as parameters.
func (d *DB) PurgeByExpiry(ctx context.Context, tableName, timeColumn string, now time.Time) (int64, error) {
	result, err := d.ExecWrite(ctx,
		"DELETE FROM "+tableName+" WHERE "+timeColumn+" IS NOT NULL AND "+timeColumn+" < ?",
		now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, errorfamily.WrapTransient(err, "sqlitestore.purge",
			"failed to purge expired rows from "+tableName)
	}

	deleted, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return 0, errorfamily.WrapTransient(affectedErr, "sqlitestore.purge_rows_affected",
			"failed to get purge count").WithContext("table_name", tableName)
	}

	return deleted, nil
}

func closeQuietly(db *sql.DB) {
	if err := db.Close(); err != nil {
		slog.Debug("sqlitestore: failed to close database on error path", "error", err)
	}
}
