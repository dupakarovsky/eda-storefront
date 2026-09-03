# 13. Consumer redelivery: explicit NakWithDelay, bounded MaxDeliver, and a real dead letter

Date: 2026-09-03

## Status

Accepted

## Context

`foundation/nats.Subscriber` (ADL 0012's NATS `Service` adapter) needs a
redelivery and failure-exhaustion policy before `depot`/`baskets` can be
built against it. Two things surfaced during design that weren't obvious
going in:

`msg.Nak()` — the obvious call to make on a handler error — does **not**
respect a JetStream consumer's configured `AckWait` or `BackOff`. Confirmed
directly against the `nats.go/jetstream` package docs: `Nak` triggers
instant redelivery regardless of consumer config; only `NakWithDelay`, or
the server's own AckWait-timeout path (a message that's delivered and never
acked/nak'd/termed at all — e.g. a crash mid-handler), respects backoff
timing. A naive `Nak()`-on-error implementation would silently make any
configured `BackOff` schedule inert on the failure path this system
actually exercises, and hammer a struggling downstream dependency at full
speed instead of backing off from it.

Separately: `MaxDeliver` bounds redelivery attempts, but exhausting it has
no built-in persistence or visibility — the server simply stops
redelivering. Left alone, this is the same class of gap ADL 0007 already
named for publish failures: a message can be permanently, silently dropped
with nothing but (at best) a log line marking it, discovered only by
someone noticing missing data later.

## Decision

**Redelivery:** `Subscriber.onMessage` inspects the message's own
`NumDelivered` (via `msg.Metadata()`) on every handler failure and calls
`msg.NakWithDelay(delay)`, computing `delay` from the consumer's own
`BackOff` slice indexed by delivery count — manually replicating what the
timeout path gets automatically, since our explicit-Nak path bypasses it.

**Standard values**, to be used as the default for every future
`Subscriber` built against this system's streams unless a specific
consumer has a concrete reason to deviate:

```go
AckPolicy:  jetstream.AckExplicitPolicy,
AckWait:    5 * time.Second,
MaxDeliver: 5,
BackOff: []time.Duration{
    5 * time.Second,
    15 * time.Second,
    30 * time.Second,
    60 * time.Second,
},
```

`len(BackOff)` (4) plus the initial attempt equals `MaxDeliver` (5) —
deliberately sized so every configured backoff slot is actually used
before exhaustion, rather than the two numbers being independently chosen
and only coincidentally consistent.

**Dead lettering:** on the delivery attempt that reaches `MaxDeliver`,
`onMessage` calls an injected `DeadLetterFunc` (if one is provided), then
calls `msg.Term()` — an explicit, unconditional "stop redelivering this,"
made once in our own code rather than left as an implicit consequence of
`MaxDeliver`'s bookkeeping. `foundation/nats.PublishDeadLetter` is the
default `DeadLetterFunc`: it logs at `Error` level and republishes the
original message's raw payload onto `<service>.deadletter.<original
-subject>`, reusing the existing `Publisher` rather than inventing a
second publish mechanism.

## Alternatives considered

**Rely on JetStream's own `MAX_DELIVERIES` advisory subject**
(`$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.*`) instead of handling
exhaustion inline in `onMessage`. Would decouple dead-lettering from the
consumer's own message-handling path entirely. Not adopted for now,
specifically to keep the failure-exhaustion decision visible in the same
place the retry logic lives, and because it would require a second
`Subscriber`-like listener just to consume advisories — real added
groundwork surface for a system that doesn't have a concrete second
consumer of dead letters yet. Worth reconsidering once `notifications`
(Project 8) or an operational dashboard gives dead letters a real
downstream reader.

If `msg.Metadata()` itself fails (returns a `nil` metadata pointer) — a
rare condition, but one that leaves delivery count genuinely unknown —
`onMessage` deliberately does not call `Ack`, `Nak`, or `Term` at all. The
message is left to the consumer's own `AckWait` timeout, which (unlike our
explicit `NakWithDelay` calls) natively honors `BackOff` without needing a
known delivery count to compute against.

**Defer dead-lettering entirely, matching ADL 0007's outbox deferral.**
Rejected this time specifically — asked for directly rather than
deferred, and unlike ADL 0007 (where no consumer of `stores`' events
existed yet to make the cost concrete), this system is about to have two
real consumers from Phase 3/4 onward, making a dropped message a live risk
from the start rather than a hypothetical one.

## Consequences

- Every future `Subscriber` in this system inherits `NakWithDelay`-based
  backoff and the standard `AckWait`/`MaxDeliver`/`BackOff` values above by
  default — a service with a genuinely different processing-time profile
  can deviate, but that would itself be a decision worth naming, not a
  silent per-service drift.
- Dead letters are real, persisted, replayable JetStream messages, not log
  lines alone — but the publish-the-dead-letter step itself has no retry
  of its own (unlike ADL 0007's `publishWithRetry`). A dead-letter publish
  failing during an already-exhausted message is a real, currently
  unhandled compounding failure — narrow, but not zero, and worth
  remembering rather than assuming solved by this ADL.
- The dead-letter subject convention (`<service>.deadletter.<original
-subject>`) needs to actually be included in whatever stream's subject
  list receives it — this ADL does not itself define or provision that
  stream; that's a concrete task for whichever phase first wires a real
  `DeadLetterFunc` into a running `Subscriber` (`depot`/`baskets`,
  Phase 3/4).
- `MaxAckPending` (the consumer's cap on simultaneously-outstanding
  unacked messages) was not addressed by this ADL and is left at its
  server default — worth a deliberate look once real throughput/latency
  numbers exist for `depot`/`baskets`, not invented speculatively here.
