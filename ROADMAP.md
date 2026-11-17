# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.

## Theme 1: Stay invisible

The engine core succeeds when consumers forget it exists. Direction: keep
the API surface minimal, the concurrency contract sharp, and the zero-domain-
types rule absolute.

Raw ideas:

- **v1.0 stability commitment** once BuildFlow and go-cqrs-lite usage
  settles — freeze the option set and the `ExecWrite`/`WithWriteLock`
  contract.
- **Benchmark suite** pinning the performance folklore: pool-sizing default
  (`max(4, min(NumCPU, 8))`), the idle-conn cap ("visible re-open churn"
  under multi-reader fan-out), and the 3s busy-timeout / 3× retry budget.
  Today these numbers are reasoned in comments, not measured.

## Theme 2: Maintenance plumbing

Raw ideas:

- **CI beyond dependabot**: a workflow running the buildflow gate (or at
  minimum `go test -race` + lint) so regressions surface in PRs, not in
  consumer repos.
- **VACUUM / incremental-vacuum helper**: `auto_vacuum=INCREMENTAL` is
  already set in the DSN, but nothing ever issues the `PRAGMA
  incremental_vacuum` that actually reclaims space. Demand-gated on a
  consumer's database actually growing.
- **Backup / snapshot helper** around SQLite's backup API for consumers
  that need point-in-time copies. Demand-gated.

## Open questions

- Do any consumers need a shared-cache or read-only connection mode
  (`mode=ro` DSN), or would that just widen the surface for no real user?
