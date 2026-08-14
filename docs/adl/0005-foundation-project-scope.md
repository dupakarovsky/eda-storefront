# 5. Foundation-only project scope: splitting shared kit from `stores`

Date: 2026-08-14

## Status

Accepted

## Context

`docs/roadmap.md`'s original project table listed Project 1 as `01-stores`,
with the `foundation/` package described as "built once here and reused by
every later project" — implying `foundation` and `stores` would be built
together, in one project.

In practice, this project built only `foundation/` — config, logging,
graceful shutdown, the HTTP `Router`/`Middleware` stack, Postgres pool
wiring, and NATS JetStream wiring, each ported from or extending patterns
proven in `order-fulfillment` and `court-reservation` — with no
`stores`-specific domain logic. `journey_progress.md`'s own Phase 1 plan
stated this explicitly at the time: "No service-specific domain logic is
built in this project; the deliverable is the kit itself."

`foundation` alone grew into six independently-substantial packages, a
`docker-compose.yml`/`Makefile` pair, and — after later hardening work — a
non-trivial Postgres security posture (see ADL 0006), each verified against
real infrastructure via `foundation-smoketest` rather than assumed. This
was already comparable in weight to a full single-service project from the
prior two-project series, before any of `stores`' own domain logic (catalog
aggregates, event publishing, its own persistence layer) had been started.

## Decision

Split what the roadmap originally scoped as one "Project 1" into two
separate projects:

1. **`foundation`** (this project) — the shared kit only, proven runnable
   via `docker-compose` and a throwaway/permanent smoke-test service, no
   service-specific domain logic.
2. **`01-stores`** (next project) — the first real domain service, built
   on top of the now-complete and independently-verified `foundation`
   package.

## Consequences

- `docs/roadmap.md`'s project table and workspace structure describe
  `foundation/` as a standalone deliverable "built once during Project 1,
  reused everywhere" — that description remains accurate in spirit, but
  the project _sequence_ itself now has one more entry than originally
  laid out: a dedicated `foundation` project ahead of `01-stores`, rather
  than the two combined.
- `01-stores` starts from a position no prior project in this series has
  had: every piece of shared infrastructure it needs (config, logging,
  shutdown, HTTP, Postgres, NATS) already exists, compiles, and has been
  proven against real running containers — including a hardened Postgres
  posture (ADL 0006) and confirmed persistence across container restarts —
  before a single line of `stores`-specific code is written.
- This does not change the total scope of work across the system, only
  where the project boundary falls. `docs/system-state.md` reflects
  `foundation` as its own row-worthy milestone at this project's close,
  ahead of `stores` still showing "Not started."
