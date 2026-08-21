# 7. Inline retry for event publishing; transactional outbox deferred

Date: 2026-08-21

## Status

Accepted

## Context

`01-stores`' application layer (`CreateStore.Execute`, Phase 3 Step 4) saves an
aggregate to Postgres, then publishes its resulting domain event to NATS
JetStream. These are two separate operations with no shared transaction —
if `Save` succeeds and the subsequent `Publish` fails (NATS unreachable,
network blip, process crash between the two calls), the aggregate exists in
Postgres with no corresponding integration event ever reaching downstream
consumers, silently and permanently unless something notices and manually
reconciles.

Three options were considered, in increasing order of correctness and
complexity: log-and-continue (cheapest, no automatic recovery), inline
retry with backoff (closes the transient-failure case, not the crash
case), and a transactional outbox — writing the event to an `outbox` table
in the same transaction as the aggregate save, with a separate poller
publishing pending rows to NATS. The outbox pattern is the only one of the
three that survives a mid-operation crash, since durability comes from
Postgres's transaction guarantees rather than the publish call succeeding
in the same breath as the save.

No downstream consumer of `stores`' events exists yet (`depot`/`baskets`
are Project 2, not yet built), so the gap this decision leaves open is
currently hypothetical rather than actively harmful.

## Decision

Adopt inline retry with backoff (three attempts, 200ms/400ms delays) as
the interim handling for publish failures. On exhausted retries, log at
`Error` level with the aggregate ID, event name, and subject, and return
the error to the caller alongside the (non-nil) saved aggregate, so a
caller can distinguish "nothing happened" from "saved but not published."

The transactional outbox pattern is explicitly deferred, not rejected — it
remains the correct answer for a system whose stated purpose (per ADL
0001/0002) is genuine, durable event-driven integration between services.
It is deferred specifically because implementing it properly (an outbox
table, a migration, a poller/worker process, a polling-vs-`LISTEN`/`NOTIFY`
decision) is scope disproportionate to Phase 3's remaining goals, and is
better tackled as its own deliberate unit of work.

## Alternatives considered

**Log-and-continue with no retry.** Rejected as the final answer (though
it's effectively what remains after retries are exhausted) — a single
transient NATS blip would strand an aggregate's event permanently with no
attempt to recover from something likely to self-resolve within
milliseconds.

**Transactional outbox now.** Rejected for this phase specifically due to
scope, not correctness — see Context and Decision above.

## Consequences

- A publish failure that outlasts three retries (600ms of attempts) still
  leaves an aggregate saved with no corresponding event ever published —
  the same fundamental gap as log-and-continue, just narrowed to
  longer-than-600ms outages instead of any outage at all.
- This gap is currently invisible to the system as a whole, since no
  service yet consumes `stores`' events — it becomes a concrete, visible
  problem the moment `depot`/`baskets` (Project 2) are built and start
  depending on never missing a `StoreCreated`.
- Revisiting this decision — building the transactional outbox properly —
  should happen before, or very early in, Project 2, once a real consumer
  exists to make the cost of a dropped event concrete rather than
  hypothetical. Worth treating as a standing item to reconsider, not a
  closed question.
- The retry helper (`publishWithRetry`) is currently local to `01-stores`'
  own `internal/application` package. If the same pattern proves useful
  in `depot`/`baskets` unchanged, it's a candidate for promotion into
  `foundation/nats`, following the same "prove it twice before extracting"
  discipline used for everything else in `foundation`.
