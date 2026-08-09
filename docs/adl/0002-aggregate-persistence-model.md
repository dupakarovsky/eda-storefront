# 2. Aggregate persistence model: current-state over event sourcing

Date: 2026-08-09

## Status

Accepted

## Context

The source system's aggregates (e.g. `ordering`'s `Order`) are event-sourced
— an aggregate's state is derived by replaying its own event history, with
snapshots for performance, and optimistic concurrency enforced via a
`Version()`/`PendingVersion()` check on the aggregate's event stream
(confirmed by reading `internal/es/aggregate_repository.go` and
`internal/es/aggregate.go` directly).

Both `order-fulfillment` and `court-reservation` used a materially different
model: aggregates persisted as current-state rows, with domain events fired
as a side effect of a state change (for logging/integration purposes) rather
than serving as the source of truth for reconstructing state. Concurrency
was handled per-project — row-level locking (`SELECT ... FOR UPDATE`) in
`order-fulfillment`, optimistic locking via a `version` column
compare-and-swap in `court-reservation`.

Event-driven _integration between services_ (services reacting to events
published by other services) and event _sourcing_ (an aggregate's own
persistence model being a replayable event log) are separable concerns. The
source book couples them; we don't have to.

## Decision

Aggregates in this system will be persisted as **current-state rows**, the
same model used in both prior projects, with domain events published
externally (over NATS) as integration events for other services to consume
— but not used internally as the aggregate's own source of truth.

Concurrency control per aggregate will use whichever of the two
already-proven approaches (row-level locking or optimistic
version-column compare-and-swap) fits the specific write pattern, decided
per-service the same way it was decided per-project previously — not
defaulted to one approach system-wide.

## Consequences

- Cross-service event-driven integration (the actual subject of the source
  book, and the reason for building this system) is fully preserved — every
  service still publishes and consumes real events over NATS.
- Event sourcing as a pattern — event streams as the aggregate's own store,
  snapshotting, replay-based rehydration — is not covered by this roadmap.
  If it becomes independently interesting later, it would be a deliberate,
  separate exercise (e.g. rebuilding one existing service's persistence
  layer as a comparison), not folded into this system's default design.
- Each service's repository/persistence layer will look structurally closer
  to `order-fulfillment`'s and `court-reservation`'s Postgres adapters
  (`Reconstitute` rehydration constructors, current-state upserts) than to
  the source book's `internal/es` package, which will not be ported.
- This is a lower-risk, more directly transferable step from where the
  first two projects left off; the source book's event-sourcing machinery
  remains available as a reference if a future decision reverses this one
  for a specific service.
