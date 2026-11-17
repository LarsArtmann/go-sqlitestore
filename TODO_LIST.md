# TODO List

> Short-term, actionable, bounded work items, verified against the actual
> code. For long-term vision and unrefined ideas, use ROADMAP.md. Items are
> ranked by impact. Status is verified, not assumed.

## Status legend

| Status           | Meaning                                                 |
| ---------------- | ------------------------------------------------------- |
| 🔴 `TODO`        | Not started. Needs doing.                               |
| 🟡 `IN_PROGRESS` | Actively being worked on.                               |
| 🔵 `BLOCKED`     | Cannot proceed; external dependency or decision needed. |
| 🟢 `DONE`        | Completed. Remove here, log in `CHANGELOG.md`.          |

## Medium Impact

| Task                                                        | Status       | Impact | Effort | Evidence                                                                                                                         |
| ----------------------------------------------------------- | ------------ | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------- |
| Bump Go toolchain to 1.27.4 when released (stdlib vuln fix) | 🔵 `BLOCKED` | Med    | 5min   | govulncheck: GO-2026-6859 (net/http MaxBytesReader crash) found in go1.27.3, fixed in go1.27.4; `go version` says go1.27.3 today |
| Read govulncheck's full 12-finding list                     | 🔴 `TODO`    | Med    | 15min  | Only GO-2026-6859 verified this session; "+7 more" unread (`buildflow -s govulncheck --format finding`)                          |

Harvested from `docs/status/2026-11-17_22-02_docs-health-audit-quality-gate-green.md`
§f (items routed to ROADMAP at harvest time: CI workflow, benchmark suite,
VACUUM/backup helpers, v1.0 commitment, read-only DSN — already rows there).
Done in the 2026-11-17 doc-truth pass and moved to CHANGELOG [Unreleased]:
GOEXPERIMENT removal, AGENTS.md docs-set paragraph, CONTRIBUTING.md
refresh, SECURITY.md freshness fix, plus the earlier audit work (file
splits, `.golangci.yml`, pre-commit hook).
