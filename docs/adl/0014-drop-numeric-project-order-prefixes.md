# 14. Drop numeric project-order prefixes from folder names

Date: 2026-09-03

## Status

Accepted

## Context

Project folders were originally created with a numeric prefix
(`01-stores`, `02-depot-baskets`) indicating build order. `docs/roadmap.md`
and `docs/adl/0001` already record that ordering explicitly and
authoritatively — the folder-name prefix was redundant with it, not an
additional source of truth. `01-stores/go.mod`'s own `module` line was
already declared as `.../eda-storefront/stores`, without the prefix,
confirming the prefix was never load-bearing for Go's own module
resolution — only for path references in `go.work`, `docker-compose.yaml`,
the `Makefile`, and documentation.

## Decision

Drop the numeric prefix from every project folder, present and future:
`01-stores` → `stores`, `02-depot-baskets` → `depot-baskets`, and so on
through `search`/`notifications`. Multi-service project folders (currently
only `depot-baskets`) are named by hyphenating their contained services,
not given a generic name — consistent with every other folder describing
its own contents rather than its position in the sequence.

Build order remains fully recorded in `docs/roadmap.md`'s project table
and ADL 0001; nothing about this rename changes or re-derives that
ordering.

Internal project structure is unaffected: a multi-service project still
gets exactly one `journey_progress.md` at its root (per-session scope, per
ADL 0004) and one `CONTRACT.md` per service inside it (per-service scope,
also per ADL 0004) — this ADL only touches the top-level folder name.

## Consequences

- `01-stores`' rename to `stores` requires updating `go.work`,
  `docker-compose.yaml`, the `Makefile`'s path references, `CONTRACT.md`,
  and `system-state.md` — verified against actual file contents, not
  assumed; the Go module path itself needed no change.
- Historical ADLs (0001–0013) are left untouched — they're dated records
  of decisions made under the naming convention active at the time,
  not living documentation to retroactively correct.
- `docs/roadmap.md`'s workspace-structure diagram should be updated to
  match, since it's descriptive of current layout, not a historical record
  the way an individual ADL's prose is.
