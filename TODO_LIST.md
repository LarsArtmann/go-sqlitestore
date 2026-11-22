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

No open items. Everything from the 2026-11-17 audit and the 2026-11-22
release prep landed in CHANGELOG [0.1.2]: GOEXPERIMENT doc removal,
CONTRIBUTING/SECURITY/AGENTS refresh, the Go toolchain pin to go1.27.4
(GO-2026-6859), and the 15 golangci-lint warning fixes. The govulncheck
gate finding is environmental (machine Go still 1.27.3) — see AGENTS.md
"Repo quirks"; it needs no repo work.
