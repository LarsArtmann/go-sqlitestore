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

| Task                                                           | Status    | Impact | Effort | Evidence                                                                                                                                                         |
| -------------------------------------------------------------- | --------- | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Split `engine.go` below the 350-line file-size cap (443 lines) | 🔴 `TODO` | High   | 1h     | buildflow file-size-check finding: "engine.go: 444 lines (max 350)"; natural seams: DSN/PRAGMA builders (~engine.go:183-245), purge helpers (~engine.go:421-437) |

## Medium Impact

| Task                                                          | Status    | Impact | Effort | Evidence                                                                                                                           |
| ------------------------------------------------------------- | --------- | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| Generate curated `.golangci.yml` via buildflow auto-configure | 🔴 `TODO` | Med    | 15min  | No `.golangci.yml` in repo root; step exists: `buildflow -s golangci-lint-auto-configure --fix` (review the diff before accepting) |

## Low Impact

| Task                                  | Status    | Impact | Effort | Evidence                                                                                         |
| ------------------------------------- | --------- | ------ | ------ | ------------------------------------------------------------------------------------------------ |
| Install the buildflow pre-commit hook | 🔴 `TODO` | Low    | 2min   | `.git/hooks/` carries only samples; `buildflow precommit install` wires the pre-commit mode gate |
