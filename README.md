# go-sqlitestore

Shared SQLite engine core for the LarsArtmann tool fleet: connection setup
(DSN with per-connection PRAGMAs, WAL, optimizations), a versioned
membership-based migration runner, a write-mutex + SQLITE_BUSY retry
wrapper, and age-based purge helpers.

Zero application types — consumers own their schemas and query surfaces;
this package owns the mechanics they used to duplicate. Consumers:
BuildFlow (`modules/dbstore`), go-cqrs-lite (storage helpers).

## Installation

```bash
go get github.com/larsartmann/go-sqlitestore@latest
```

Pure Go (modernc.org/sqlite) — no cgo, no platform toolchain needed.

## Use

```go
package main

import (
    "context"
    "database/sql"

    "github.com/larsartmann/go-sqlitestore"
)

func main() {
    ctx := context.Background()

    db, err := sqlitestore.Open("data/app.db") // WAL + PRAGMAs + pool sized
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Migrations are explicit: the caller owns the list.
    err = sqlitestore.RunMigrations(ctx, db.DB(), []sqlitestore.Migration{
        {
            Version:     1,
            Description: "create events",
            Up: func(ctx context.Context, db *sql.DB) error {
                _, err := db.ExecContext(ctx,
                    "CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY)")
                return err
            },
        },
    })
    if err != nil {
        panic(err)
    }

    // Single writes: mutex + SQLITE_BUSY retry live here.
    if _, err := db.ExecWrite(ctx, "INSERT INTO events (id) VALUES (1)"); err != nil {
        panic(err)
    }

    // Multi-statement units serialize as one write.
    err = db.WithWriteLock(func() error {
        _, err := db.DB().ExecContext(ctx, "INSERT INTO events (id) VALUES (2)")
        return err
    })
    if err != nil {
        panic(err)
    }

    // Reads go straight through the raw handle — WAL never blocks them.
    var count int
    if err := db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count); err != nil {
        panic(err)
    }
    _ = count
}
```

The one contract that matters: **all writes go through `ExecWrite` or
`WithWriteLock`**, never raw `ExecContext` — that is where the write mutex
and the SQLITE_BUSY/LOCKED retry policy live. Reads use `DB()` freely.

## Test

```bash
GOEXPERIMENT=jsonv2 go test ./...
```
