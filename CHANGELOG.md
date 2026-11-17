# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Release dates match the git tag dates (`git for-each-ref refs/tags`).

## [Unreleased]

### Added

- Direct tests for the write-lock primitives (`WithWriteLock`, `WriteLock`),
  the `IsBusy` positive path (a real SQLITE_BUSY reproduced with a
  contending writer and `busy_timeout=0`), `TableExists`, `TruncateSQL`,
  and `IsDuplicateColumnError` — every exported symbol now has test
  evidence; suite passes `go test -race -count=1 ./...`.
- Documentation set: `AGENTS.md`, `FEATURES.md`, `ROADMAP.md`,
  `TODO_LIST.md`; README gained an installation section and a
  compile-checked usage example.

### Fixed

- Corrected this changelog's own history: v0.1.1 named a
  `WithMaxConnections` option that never existed (the option is
  `WithMaxOpenConns`, shipped in v0.1.0; v0.1.1's change was the internal
  idle-connection cap), the v0.1.0 date now matches its tag (2026-09-22,
  not 2026-10-10), and a template-placeholder duplicate
  `[0.1.0] - 2026-01-01` entry was removed.

## [0.1.1] - 2026-10-14

### Added

- **Idle-connection cap**: `SetMaxIdleConns(maxConns)` now rides the
  connection-pool configuration so long-lived hosts stop holding one pooled
  handle per historical concurrency spike (closing and re-opening pooled
  SQLite connections costs a fresh page-cache warmup per connection).
- Repository scaffolding (LICENSE, CONTRIBUTING, SECURITY.md, dependabot,
  dprint config) from the LarsArtmann template generation.

## [0.1.0] - 2026-09-22

### Added

- Initial release: the shared SQLite engine core — connection setup with
  DSN/PRAGMA configuration (`Open`/`OpenDB`/`New`, `WithBusyTimeout`,
  `WithMaxOpenConns`), WAL mode, 3s busy timeout, membership-based
  migrations runner (`RunMigrations`, `AddColumnIfNotExists`,
  `TableExists`, `ColumnExists`), write mutex + SQLITE_BUSY retry
  (`ExecWrite`, `WithWriteLock`), and the purge helper (`PurgeByExpiry`).
