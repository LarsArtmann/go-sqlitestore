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

## High Impact

| Task                                                             | Status    | Impact | Effort | Evidence                                                                                                                                                                                                                         |
| ---------------------------------------------------------------- | --------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Remove `GOEXPERIMENT=jsonv2` from the docs (unneeded in Go 1.27) | 🔴 `TODO` | High   | 5min   | `README.md:87`, `AGENTS.md:20`, `AGENTS.md:140`; cache-busted `go test -count=1 ./...` green without the flag; auto-configured `.golangci.yml` omits `goexperiment.jsonv2` (harvested from `docs/status/2026-11-17_22-02_*` §f1) |

## Medium Impact

| Task                                                           | Status       | Impact | Effort | Evidence                                                                                                                               |
| -------------------------------------------------------------- | ------------ | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| Bump Go toolchain to 1.27.4 when released (stdlib vuln fix)    | 🔵 `BLOCKED` | Med    | 5min   | govulncheck: GO-2026-6859 (net/http MaxBytesReader crash) found in go1.27.3, fixed in go1.27.4; `go version` says go1.27.3 today       |
| Update AGENTS.md docs-set paragraph (local `docs/status/` now) | 🔴 `TODO`    | Med    | 5min   | `AGENTS.md` "no `docs/status/` corpus" claim contradicted by `docs/status/2026-11-17_22-02_*.md`; awaiting corpus-policy answer (§g1)  |
| CONTRIBUTING.md: name buildflow as the canonical gate          | 🔴 `TODO`    | Med    | 10min  | `CONTRIBUTING.md` still teaches hand-run `go test ./... -race` + `golangci-lint run ./...`; AGENTS.md already routes through buildflow |
| Read govulncheck's full 12-finding list                        | 🔴 `TODO`    | Med    | 15min  | Only GO-2026-6859 verified this session; "+7 more" unread (`buildflow -s govulncheck --format finding`)                                |

## Low Impact

| Task                        | Status    | Impact | Effort | Evidence                                                                   |
| --------------------------- | --------- | ------ | ------ | -------------------------------------------------------------------------- |
| SECURITY.md freshness check | 🔴 `TODO` | Low    | 5min   | Never reviewed in the 2026-11-17 session (scope line of the status report) |

Harvested from `docs/status/2026-11-17_22-02_docs-health-audit-quality-gate-green.md`
§f (items routed to ROADMAP at harvest time: CI workflow, benchmark suite,
VACUUM/backup helpers, v1.0 commitment, read-only DSN — already rows there).
Earlier 2026-11-17 audit work (file splits, `.golangci.yml`, pre-commit
hook) is done — see CHANGELOG [Unreleased].
