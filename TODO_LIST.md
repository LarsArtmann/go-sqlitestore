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

## Open items

| # | Item                                                                     | Status           | Notes                                                                                                                                                                                                                                               |
| - | ------------------------------------------------------------------------ | ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | Bump system Go to 1.27.4 (home-manager)                                  | 🔵 `BLOCKED`     | Go ships via the home-manager profile (HM 26.11) but no config declaring it is findable (`~/.config/home-manager` absent, vanilla `/etc/nixos`); owner must bump nixpkgs/HM. Clears the environmental govulncheck finding (AGENTS.md "Repo quirks") |
| 2 | docs/status/ corpus policy                                               | 🔴 `TODO` (user) | Standing corpus (a report per meaningful session) or rare one-offs? Carried from the 2026-11-17 report; two reports now exist                                                                                                                       |
| 3 | Watch dependabot weekly gomod run pick up v0.1.2 for long-tail consumers | 🔴 `TODO`        | Direct consumers (BuildFlow modules/dbstore, go-cqrs-lite storage/v4) bumped manually 2026-11-22; apps consuming transitively converge via dependabot                                                                                               |

Resolved 2026-11-17 → 2026-11-22 sessions: GOEXPERIMENT doc removal,
CONTRIBUTING/SECURITY/AGENTS refresh, the Go toolchain pin to go1.27.4
(GO-2026-6859), and the 15 golangci-lint warning fixes landed in CHANGELOG
[0.1.2].
