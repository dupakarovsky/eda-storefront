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

Adopt inline retry with backoff (five attempts, each individually bounded
to 1.5s via a per-attempt derived context, with 200ms/400ms/800ms/1600ms
delays between attempts) as the interim handling for publish failures —
giving a real, tested worst-case budget of roughly 10.5 seconds before
giving up. On exhausted retries, log at `Error` level with the aggregate
ID, event name, and subject, and return the error to the caller alongside
the (non-nil) saved aggregate, so a caller can distinguish "nothing
happened" from "saved but not published."

This budget was arrived at empirically, not by estimate alone: an initial
version without per-attempt timeouts was found, via a real forced-outage
test, to block for an unbounded and unpredictable duration on any attempt
made while the NATS client itself was in an ambiguous reconnect-in-progress
state — one observed run blocked ~4.7 seconds on a single attempt before
returning an error, several times longer than the deliberate backoff sleeps
between attempts. Adding a 1.5-second timeout per attempt (via
`context.WithTimeout` derived from the caller's context) closed that gap,
making the total retry duration a real, boundable number rather than an
estimate that could be off by several times in practice.

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

## Consequences (addendum, 2026-08-28)

- Phase 4 exposed a real collision between this ADL's retry budget and
  `foundation/http`'s global request/write timeouts: `TimeoutMiddleware`
  wrapped every request (including every command handler depending on
  `publishWithRetry`) in a single global deadline that was, until now,
  shorter than the ~10.5s worst-case retry budget this ADL measured and
  documented — meaning a real NATS outage encountered through the actual API
  layer (as opposed to Phase 3's test, which called `Execute` directly
  against an undeadlined `context.Background()`) would have had the retry
  sequence cut short by the surrounding HTTP timeout before it could
  complete, never actually getting the fair five-attempt run this ADL's
  Decision describes.
- Resolved for now by raising `foundation/config`'s global
  `SRV_REQUEST_TIMEOUT`/`SRV_WRITE_TIMEOUT` defaults (12s/13s) past the
  measured worst case — a system-wide change, since `config.go` lives in
  `foundation` and every current and future service inherits its defaults,
  not a `01-stores`-local fix.
- This is a blunt fix, not a complete one: `foundation`'s `Router` only
  supports global middleware today, so pure-read RPCs (`GetStore`,
  `ListProducts`) now inherit the same generous ceiling as a command
  surviving a full retry sequence, despite never needing anywhere near that
  long. A per-procedure timeout (via a Connect interceptor, which can branch
  on `req.Spec().Procedure`) would resolve this properly but was deferred as
  its own piece of work — noted here as a standing item, in the same spirit
  as this ADL's original outbox deferral, rather than solved under pressure
  while closing out Phase 4.
