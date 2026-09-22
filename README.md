# go-sqlitestore

Shared SQLite engine core for the LarsArtmann tool fleet: connection setup
(DSN with per-connection PRAGMAs, WAL, optimizations), a versioned
membership-based migration runner, a write-mutex + SQLITE_BUSY retry
wrapper, and age-based purge helpers.

Zero application types — consumers own their schemas and query surfaces;
this package owns the mechanics they used to duplicate.

Consumers: BuildFlow (`modules/dbstore`), go-cqrs-lite (storage helpers).

## Use

```go
db, err := sqlitestore.Open(path)
defer db.Close()

if err := sqlitestore.RunMigrations(ctx, db.DB(), migrations); err != nil { ... }

_, err = db.ExecWrite(ctx, "INSERT ...", args) // mutex + busy retry
```

## Test

```bash
GOEXPERIMENT=jsonv2 go test ./...
```
