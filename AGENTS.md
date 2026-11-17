# AGENTS.md — go-sqlitestore

Shared SQLite engine core for the LarsArtmann tool fleet. Single Go package
(`sqlitestore`), extracted from BuildFlow so the mechanics stop being
duplicated. Consumers: BuildFlow (`modules/dbstore`), go-cqrs-lite (storage
helpers).

## The cardinal rule: zero application types

This package owns SQLite **mechanics only** — connection setup, migrations
runner, write serialization, purge helpers. It must never learn a consumer's
domain (no tool/pipeline/finding/cache types). Consumers own their schemas and
query surfaces. If a change smells like domain logic, it belongs in the
consumer repo, not here.

## Commands

```bash
# Test (jsonv2 went stable in Go 1.27 — no GOEXPERIMENT flag needed)
go test ./...

# Race detector (CONTRIBUTING.md)
go test ./... -race

# Quality gate — BuildFlow auto-detects this repo (no .buildflow.yml, no
# flake.nix, no Makefile). Delegate lint/format/tidy to it, don't run the
# underlying tools by hand (see the buildflow skill).
buildflow                 # full quality gate
buildflow --build-mode fast   # ~30s iteration loop, skips tests
buildflow -s <tool> --format finding   # drill into one tool's findings

# Non-Go formatting (markdown/json/yaml) via dprint.json
dprint fmt
```

The findings gate fails when `error`-severity findings remain; the gate is
green as of 2026-11-17 — the known surviving warnings are the
go-structure-linter `internal/` suggestion (declined non-fix, single-
package lib) and branching-flow "swap hazard" branding hints on the
identifier-typed helpers. Read the summary, not just the exit code.
`reports/` is buildflow's artifact dir — gitignored, never commit it.

Go lint without buildflow: `golangci-lint run ./...` (CONTRIBUTING.md), but
prefer `buildflow -s golangci-lint` for the verify step and cache isolation.

## Layout

Flat, single package, no subpackages, and deliberately no `internal/`
(single-package library; the go-structure-linter's `internal/` suggestion
is a declined non-fix). All files stay under the 350-line file-size gate
(the 2026-11-17 split carved `engine.go` into three):

- `doc.go` — package documentation (keep in sync with README's description)
- `engine.go` — connection core: `DB` struct, `Open`/`OpenDB`/`New`, options
  and pool sizing, `HealthCheck`, `PurgeByExpiry`, `BusyRetryCount`,
  `SetDebugSQL`, `Close`
- `dsn.go` — `BuildDSN`/`BuildDSNWithBusyTimeout`, `EnableWAL`,
  `ApplyOptimizations` (every PRAGMA decision lives here)
- `write.go` — the write path: `ExecWrite` (mutex + SQLITE_BUSY retry),
  `WithWriteLock`/`WriteLock`, `IsBusy`, `TruncateSQL` (driver registration
  rides this file's modernc.org/sqlite import)
- `migrations.go` — `RunMigrations` + schema introspection helpers
  (`TableExists`, `ColumnExists`, `AddColumnIfNotExists`)
- `sqlitestore_test.go` + `migrations_test.go` + `suite_test.go` —
  Ginkgo/Gomega specs, split the same way

## Documentation set

Living docs follow the docs-health ownership model — each fact lives in
exactly one file: `FEATURES.md` (inventory + honest status), `TODO_LIST.md`
(open work only; done items move to CHANGELOG), `ROADMAP.md` (raw ideas),
`CHANGELOG.md` (append-only after release; dates must match git tag dates —
no template placeholders). The docs-gates battery rides inside buildflow
(`docs-gates cadence .` fails hard when `TODO_LIST.md` is missing).
Status reports live in `docs/status/` (first one landed 2026-11-17); before
that, reports about this library lived in consumer repos (BuildFlow).

## Architecture invariants (don't break these casually)

### Concurrency contract

WAL mode = unlimited concurrent readers, single writer. The write-mutex +
retry policy is the product:

- **All writes must go through `ExecWrite`** (or `WithWriteLock`/`WriteLock`
  for multi-statement units) — never raw `db.ExecContext`. The mutex
  serializes goroutine-local contention; the retry loop (3 attempts,
  exponential backoff 100/200/400ms) handles SQLITE_BUSY/SQLITE_LOCKED from
  external processes.
- `DB()` exposes the raw handle **for reads only** — that asymmetry is
  deliberate and documented in the doc comments.
- Pool sizing: default `max(4, min(NumCPU, 8))`, and `SetMaxIdleConns` equals
  the open cap deliberately — reconnecting pooled SQLite connections costs a
  page-cache warmup (visible churn under BuildFlow's multi-reader fan-out).

### PRAGMAs ride the DSN, not one-shot Exec

Every pooled connection must inherit `_busy_timeout`, `_journal_mode=wal`,
`_synchronous=NORMAL`, etc. via the DSN. A one-time `PRAGMA` exec only
configures the connection that happened to run it. If you add a PRAGMA, add
it to `BuildDSNWithBusyTimeout` (and mirror in `EnableWAL`/
`ApplyOptimizations` where it must also hold at pool level).

### Migration semantics: membership, never max-version

`RunMigrations` applies any version NOT in `schema_migrations`, regardless of
numeric position. This supports one database shared by multiple consumers
(state migrates to v10, cache owns v11, either order works). Consequences:

- Append-only: **never modify, reorder, or remove existing migrations** —
  history is immutable.
- Migrations must be idempotent within themselves (`CREATE TABLE IF NOT
  EXISTS`, `AddColumnIfNotExists`) because the version record is written
  AFTER `Up` returns; a mid-migration crash re-applies it.
- `Open` deliberately does NOT run migrations — callers pass their own list.

### SQL identifier interpolation

`PurgeByExpiry` and `AddColumnIfNotExists` interpolate table/column names
into SQL (identifiers can't be bound as parameters). The contract is that
callers pass compile-time constants. Preserve that warning in docs if you
touch these.

## Conventions

- **Errors**: every error path wraps via `go-error-family` with an op code
  prefixed `sqlitestore.<op>` — `WrapInfrastructure` for setup/environment
  failures, `WrapTransient` for runtime/retryable ones.
- **Errors API**: `errors.AsType[*sqlite.Error]` (Go 1.26+ generic form),
  never legacy `errors.As`.
- **Logging**: `log/slog` at package level; Debug for per-statement/retry
  noise, Info for migration application, Warn for exhausted retries.
- **Driver**: `modernc.org/sqlite` (pure Go, no cgo), driver name `"sqlite"`.
  `write.go` imports it as named `sqlite` (error type) plus
  `sqlite3 ... /lib` (result-code constants); importing the package at all
  runs its init, which registers the driver — no separate blank import
  needed.
- **Go version**: go.mod says `go 1.27`; the jsonv2 experiment went
  stable there, so no `GOEXPERIMENT` flag is needed for tests or builds.
- **Tests**: Ginkgo v2 + Gomega, external test package (`sqlitestore_test`),
  real on-disk databases in `GinkgoT().TempDir()` — no mocks, and the
  driver cannot be stubbed (unexported error fields), so contention specs
  reproduce a REAL SQLITE_BUSY with two connections and `busy_timeout=0`.
  Test behavior, not internals (e.g. migration tests assert via
  `schema_migrations` contents).
- **Style**: tabs in Go, 2-space in yml/json (`.editorconfig`); dprint owns
  non-Go formatting; `*.md` keeps trailing whitespace.

## Release

Versioned by git tag (`v0.1.0`, `v0.1.1`) + a `CHANGELOG.md` entry per
release (Keep-a-Changelog format, `## [x.y.z] - date` sections). Cut via the
go-release skill. Dependabot keeps go modules weekly (minor+patch grouped).

## Repo quirks

- An auto-commit daemon commits working-tree changes with heuristic
  messages ("chore: auto-commit N changed file(s)") — don't be surprised by
  commits you didn't make, and verify critical work lands with `git status`.
- `.config/metadata.yaml` is project-tracking metadata (tags: `lib`) —
  managed externally, leave alone.
- The buildflow pre-commit hook is installed (`buildflow precommit
  install`, 2026-11-17); it runs the pre-commit mode gate and re-stages
  formatted files on commit.
- `.golangci.yml` is auto-configure-managed — prefer
  `buildflow -s golangci-lint-auto-configure --fix` over hand-curating it.
- No CI workflows beyond dependabot; quality enforcement is local buildflow
  plus the pre-commit hook.
