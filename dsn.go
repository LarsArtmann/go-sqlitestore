package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	errorfamily "github.com/larsartmann/go-error-family"
)

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
