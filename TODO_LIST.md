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

No other open items. The 2026-11-17 docs-health audit cleared the backlog:
the engine.go and test-suite file-size splits, the `.golangci.yml`
generation, and the pre-commit hook install are done (see CHANGELOG
[Unreleased]).
