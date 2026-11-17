package sqlitestore

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	errorfamily "github.com/larsartmann/go-error-family"
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
	// Keep idle conns equal to the open cap: closing and re-opening pooled
	// SQLite connections costs a fresh page-cache warmup per connection, and
	// parallel-read bursts (database/sql default keeps only 2 idle) triggered
	// visible re-open churn under BuildFlow's multi-reader fan-out.
	db.SetMaxIdleConns(maxConns)

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
