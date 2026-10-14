# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.1.1] - 2026-10-14

### Added
- **`WithMaxConnections` / `SetMaxIdleConns`**: the engine now caps idle
  database handles (`SetMaxIdleConns(maxConns)`) so long-lived hosts stop
  holding one pooled handle per historical concurrency spike; the cap rides
  the existing connection-pool configuration.
- Repository scaffolding (LICENSE, CONTRIBUTING, SECURITY.md, dependabot,
  dprint config) from the LarsArtmann template generation.

## [0.1.0] - 2026-10-10

### Added
- Initial project structure: the shared SQLite engine core (migrations
  runner, DSN/PRAGMA setup, WAL mode, 3s busy timeout, write mutex +
  SQLITE_BUSY retry, WithWriteLock, purge helper).

## [0.1.0] - 2026-01-01

### Added
- Initial release
