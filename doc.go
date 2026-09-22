// Package sqlitestore is the shared SQLite engine core for the
// LarsArtmann tool fleet: connection setup (DSN with per-connection
// PRAGMAs, WAL, optimizations), a versioned migration runner, a
// write-mutex + SQLITE_BUSY retry wrapper, and age-based purge helpers.
//
// It carries ZERO BuildFlow types — the engine knows nothing about tools,
// pipelines, or findings. Consumers (BuildFlow's modules/dbstore,
// go-cqrs-lite's storage helpers) own their schemas and query surfaces;
// this package owns the mechanics they used to duplicate.
//
// Concurrency model: WAL mode permits unlimited concurrent readers with a
// single writer. ExecWrite serializes writes at the Go level and retries
// SQLITE_BUSY/SQLITE_LOCKED (external-process contention) with exponential
// backoff, so reads never queue behind a contended write.
package sqlitestore
