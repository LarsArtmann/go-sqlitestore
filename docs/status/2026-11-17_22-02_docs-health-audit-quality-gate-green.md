# Status Report — docs-health audit + quality gate green (2026-11-17 22:02)

**Scope.** Verified here, with fresh commands run this session (20:30–22:02):
`go build ./...` and `go vet ./...` clean; `go test -race -count=1 ./...` ok
(both WITH `GOEXPERIMENT=jsonv2` and WITHOUT it — cache-busted with
`-count=1`); `buildflow` full gate **exit 0** (21:5x, after all changes);
docs-gates battery **5/5 PASS**; `go version` = go1.27.3; file sizes via
`wc -l`; all session work confirmed landed via auto-commit daemon (HEAD
`3a43345` at 21:57). NOT verified here: SECURITY.md (never read);
CONTRIBUTING.md not re-checked after this session's changes; govulncheck's
12 warnings only spot-checked (the GO-2026-6859 stdlib vuln identified,
the "+7 more" unread); coverage % never extracted from
`reports/coverage.out`; the buildflow stale-binary WARN (BuildFlow#81) not
investigated — it lives in the BuildFlow repo.

Context: single-session docs-health AUDIT (user-mandated "SUPERB" pass)
plus fix-on sight code work it uncovered. Baseline at session start:
buildflow exit 69 (4 error findings), no TODO_LIST/FEATURES/ROADMAP,
CHANGELOG with factual lies, zero direct tests on the v0.1.0 write-lock
primitives.

## a) FULLY DONE

1. **AGENTS.md created and kept truthful** — package contract, commands,
   architecture invariants, conventions, quirks; updated 3× as reality
   changed (doc set added, layout split, driver-import claim corrected
   after verification). Evidence: in HEAD, buildflow agents-md check ✔.
2. **CHANGELOG.md factual history repaired** — phantom `WithMaxConnections`
   API (never existed in any tag; verified via `git show v0.1.1:engine.go`)
   replaced with the real one-line idle-cap change; `[0.1.0]` date fixed to
   tag date 2026-09-22 (`git for-each-ref refs/tags`); template-placeholder
   duplicate `[0.1.0] - 2026-01-01` removed; transparent self-correction
   entry under [Unreleased]/Fixed. Evidence: diff in HEAD.
3. **TODO_LIST.md built** — currently exactly one honest BLOCKED row (Go
   1.27.4 toolchain bump). Evidence: docs-gates cadence PASS (it fails hard
   when the file is missing).
4. **FEATURES.md built** — 17 feature rows, all FULLY_FUNCTIONAL, every
   `file:line` evidence re-verified AFTER the file split (grep map of all
   exported symbols on disk); explicit out-of-scope table.
5. **ROADMAP.md built** — 2 themes + open questions; every idea demand-
   gated or decision-gated; nothing invented as "planned".
6. **README.md improved** — installation section added (closed buildflow's
   info finding); usage example **compile-checked against the real module**
   (temp module with `replace` directive → `go build` → COMPILES).
7. **Six new test specs close the evidence gaps** — `WithWriteLock`
   (4-goroutine serialization), `WriteLock` explicit release, `IsBusy`
   **positive path with a real SQLITE_BUSY** (two `busy_timeout=0`
   connections + open write txn — the driver error type cannot be
   synthesized, unexported fields), `TableExists`, `TruncateSQL`,
   `IsDuplicateColumnError`. Evidence: suite green under `-race`.
8. **engine.go split 443 → engine.go 246 / dsn.go 74 / write.go 140** —
   zero API change, natural seams (connection core / DSN+PRAGMAs / write
   path). Evidence: build + vet + race green; file-size-check finding gone.
9. **Test suite split 380 → sqlitestore_test.go 249 / migrations_test.go
   160** (my new specs had pushed it over the 350 cap).
10. **.golangci.yml created** (seeded minimal, curated by
    `buildflow -s golangci-lint-auto-configure --fix`, 283 lines, house
    pattern from BuildFlow/go-cqrs-lite) — closed the go-structure-linter
    error finding.
11. **buildflow pre-commit hook installed** (`buildflow precommit
    install`).
12. **Quality gate red → green: buildflow exit 69 → 0** (4 error findings
    → 0; surviving items are warnings: declined `internal/` suggestion,
    branching-flow branding hints, timing-regression noise).
13. **docs-gates battery 5/5 PASS** (freshness, audit-all, annotate,
    routing, cadence).

## b) PARTIALLY DONE

1. **GOEXPERIMENT=jsonv2 removal** — user ruled the flag unneeded in Go
   1.27. Grep located all 3 references (`README.md:87`, `AGENTS.md:20`,
   `AGENTS.md:140`); fix NOT applied (interrupted by this status request).
   Supporting evidence already gathered: cache-busted `go test -count=1`
   green without the flag; the freshly auto-configured `.golangci.yml`
   omits `goexperiment.jsonv2` from build-tags where BuildFlow's older
   config keeps it. Remaining: 3 small edits + gate re-run. Effort: S.
2. **This status report's HARVEST** — §f items below are annotated with
   routing status; TODO_LIST/ROADMAP update rides with this report's
   commit (same session). Effort: S.

## c) NOT STARTED

1. Go 1.27.4 toolchain bump — BLOCKED on release (GO-2026-6859 fixed in
   1.27.4). Wanted; TODO_LIST row exists.
2. CI beyond dependabot — ROADMAP raw idea; no decision yet.
3. Benchmark suite pinning pool-size/idle-cap/busy-timeout folklore —
   ROADMAP; not started by design (numbers are reasoned, not measured).
4. VACUUM / incremental-vacuum helper, backup/snapshot helper — ROADMAP,
   demand-gated on consumers; no demand signal collected yet.
5. v1.0 stability commitment — ROADMAP; waiting for consumer usage to
   settle.
6. Read-only DSN mode — ROADMAP open question; not investigated.
7. CONTRIBUTING.md refresh (name buildflow as canonical gate) — known
   stale-ish, untouched this session.
8. SECURITY.md freshness review — never read this session.

## d) TOTALLY FUCKED UP

1. **Self-duplicating edit during the split** — my multiedit "removal" of
   the write path from engine.go pasted the old block back as the
   replacement, producing 12 duplicate-declaration errors. Caught by LSP
   instantly; fixed with a second pass. Severity: none lasting. Root
   cause: hand-built giant old_string/new_string pairs. Fix: chunk edits.
2. **New tests broke a gate I was fixing** — adding the 6 specs pushed
   sqlitestore_test.go to 380 lines, over the 350 file-size cap; I created
   a new error finding while clearing old ones. Caught on the next
   buildflow run, not before. Should have sized the test file mid-change.
3. **Stale read-state edit failures (×3)** — after `sed -i`/`dprint fmt`
   mutated files outside the edit tool's tracking, subsequent
   edit/write calls were rejected. Each cost a re-read cycle. Fix: always
   re-view after shell-based mutation.
4. **Markdown-table sed mangle** — a `sed` for the 443-vs-444 line count
   produced nested-quote garbage inside a TODO_LIST table cell; took two
   more edits to clean. Should have used the edit tool directly.
5. **Docs written before the code restructure** — FEATURES.md was fully
   evidence-cited against pre-split line numbers, then invalidated by my
   own engine.go split; whole file re-verified and rewritten. Ordering
   failure: restructure first, document second.
6. **A near-lie in AGENTS.md** — first draft claimed write.go carries a
   "blank import (registration)"; it carries only named imports (any
   import registers the driver via init). Caught on verification pass
   against the actual file. Exactly the class of drift AGENTS.md exists
   to prevent.
7. **GOEXPERIMENT=jsonv2 propagated without verification** — I wrote the
   flag into README/AGENTS as "part of the documented contract" this
   session without ever testing that it was still needed; the user had to
   correct me. The evidence it was unnecessary was one flag-less test run
   away the whole time.

## e) WHAT WE SHOULD IMPROVE

1. **Verify inherited contracts before encoding them** — "README-canonical
   test flag" was cargo cult; one `go test -count=1` disproved it.
   Impact: stale instructions to every future session. Fix: re-derive any
   command that claims to be a contract.
2. **Order: restructure code, then write evidence-cited docs** — line
   references churn otherwise. Impact: full FEATURES rewrite this session.
3. **Run the gate after each code change, not at the end** — would have
   caught the test-file size overflow immediately. Impact: one red gate
   cycle.
4. **Chunked edits over mega old_strings** — the duplicate-block failure
   and the table mangle both came from bulk text surgery. Impact: broken
   intermediate states. Fix: smaller anchors, or rewrite via write tool.
5. **AGENTS.md docs-set paragraph is now stale** — it says this repo has
   NO docs/status/ corpus "deliberately"; this report creates one. Must
   be updated with the corpus decision (see §g).
6. **govulncheck output discipline** — 12 warnings summarized as "+7
   more"; a full read belongs in the TODO_LIST review item.

## f) Top N next tasks

Routing status: [T] = TODO_LIST (this session), [R] = already in ROADMAP,
[D] = done by another row.

1. [T] Remove `GOEXPERIMENT=jsonv2` from README.md:87, AGENTS.md:20,140;
   re-run buildflow. — High / S / Documentation
2. [T] Update AGENTS.md docs-set paragraph: docs/status/ corpus now
   exists (decision from §g). — Medium / S / Documentation
3. [T] CONTRIBUTING.md: name buildflow as the canonical gate, drop
   hand-run lint instructions, fix test command. — Medium / S /
   Documentation
4. [T] Read govulncheck's full 12-finding list; confirm they reduce to
   GO-2026-6859 (stdlib) or route each. — Medium / S / Quality
5. [T] SECURITY.md freshness check (never reviewed this session). —
   Low / S / Quality
6. [R] Bump Go toolchain to 1.27.4 when released. — Medium / S / Quality
   (also TODO_LIST, BLOCKED)
7. [R] CI workflow beyond dependabot (buildflow gate in CI). — Medium /
   M / Quality
8. [R] Benchmark suite: pool default, idle cap, busy-timeout budget. —
   Medium / M / Quality
9. [R] VACUUM/incremental-vacuum helper. — Low / M / Feature
   (demand-gated)
10. [R] Backup/snapshot helper. — Low / M / Feature (demand-gated)
11. [R] v1.0 stability commitment decision. — Low / S / Documentation
12. [R] Read-only DSN mode: answer the open question. — Low / S /
    Documentation
13. [T] After harvest, verify docs-gates still green with a live corpus. —
    High / S / Quality (rides with this report)
14. [T] Archive flow for this report once items are resolved
    (docs-health ANNOTATE). — Low / S / Documentation
15. [D] Capture a coverage baseline from reports/coverage.out — better:
    rely on buildflow test-coverage gate; no hand number. — Low / S /
    Quality

## g) Questions I cannot answer myself

1. **Corpus policy:** AGENTS.md currently declares "status reports about
   this library live in consumer repos (BuildFlow), deliberately". This
   report creates a local `docs/status/` corpus. Keep it local going
   forward (then I update AGENTS.md and the docs-gates battery gets a
   standing corpus), or is this a one-off snapshot?
2. **Toolchain bump policy for Go 1.27.4:** when it releases, bump
   go.mod's `go` directive + toolchain pin, or leave `go 1.27` floating
   and bump only the local toolchain? (Both keep the gate green; the
   repo's public consumers see different minimums.)
3. **Demand signals:** should I actively poll BuildFlow and go-cqrs-lite
   for VACUUM/backup/read-only-DSN demand (turning ROADMAP demand-gated
   items into real rows), or leave them dormant until a consumer asks?

— End of report. Waiting for instructions.
