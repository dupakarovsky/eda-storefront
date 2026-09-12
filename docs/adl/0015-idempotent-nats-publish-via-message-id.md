# 15. Idempotent NATS publish via message ID, not connection-level workarounds

Date: 2026-09-11

## Status

Accepted

## Context

`02-depot-baskets`' Phase 3 introduced the first real, sequence-number-verified
test of ADL 0007's publish-retry logic under an actual NATS outage
(`TestStartBasket_Execute_SucceedsAfterNATSRestart`). The test itself passed —
`Execute` eventually succeeded — but inspecting the stream afterward
(`nats stream info baskets`) showed `Messages: 4`, not the `2` a single
successful publish per test would predict (test 1's one call at `seq=1`, test
2's one call landing at `seq=2`). Instead, test 2's single logical `Execute`
call produced three durably stored messages (`seq=2,3,4`), confirmed by the
stream's own message count, not inferred from timing alone.

The root cause: `nats.go`'s client-side connection maintains a reconnect
buffer by default. A publish attempted while the connection is down but still
attempting to reconnect (exactly `RetryOnFailedConnect`'s intended behavior,
per ADL 0011) can be silently queued client-side and flushed once the
connection re-establishes — independent of whether the _caller's_ own
per-attempt context deadline (ADL 0007's 1.5s) had already expired and been
reported as a failure. The caller sees "attempt failed" and retries; the
original, supposedly-failed attempt's message still arrives at the server
later, as a genuine, separately-acked, sequence-numbered message. Both
`stores` and `baskets` share this exposure, since both use the same
`foundation/nats.Publisher.Publish` call, unguarded, in their respective
(currently duplicated, per ADL 0007) `publishWithRetry` helpers.

JetStream streams already carry a built-in mechanism for exactly this
ambiguity: a `Nats-Msg-Id` header attached to a publish, checked against a
per-stream `Duplicate Window` (2 minutes, by default — confirmed present on
the `baskets` stream via `nats stream info` without either service ever
having configured it explicitly). A publish carrying a message ID that
matches one already seen within the window is not stored a second time; the
original message's ack is returned instead. Neither service's `Publisher`
used this at all — `p.js.Publish(ctx, subject, payload)` was called with no
message ID, so every retry looked like an unrelated, brand-new message to
JetStream.

## Decision

`foundation/nats.Publisher.Publish` gains a required `msgID string`
parameter, passed to the underlying `jetstream.Publish` call via
`jetstream.WithMsgID(msgID)`. Both services' local `publishWithRetry`
helpers thread this through unchanged: `publishDomainEvent` generates
**one** `uuid.NewString()` per logical publish call, before entering the
retry loop, and the identical value is passed on every attempt (1 through 5) of that same call. This is what makes JetStream see repeated arrivals of
one message ID rather than five distinct ones — a fresh ID per attempt
would defeat the mechanism entirely.

## Alternatives considered

**Disable `nats.go`'s reconnect buffer** (`nats.ReconnectBufSize(0)` or
negative, forcing publishes attempted while disconnected to fail immediately
rather than queue). Rejected as treating the symptom, not the cause — it
would stop _this specific_ buffering-driven duplication, but the underlying
ambiguity (a publish landing server-side while the client never sees
confirmation, for any reason — a dropped ack packet, a process restart
between send and ack, not just a buffered reconnect) would remain entirely
unaddressed. Message-ID-based idempotency is correct regardless of _why_ the
client and server disagree about whether a publish succeeded; disabling
buffering only helps in the one narrow case this investigation happened to
surface.

## Consequences

- ADL 0009 stated `uuid.NewString()` in `connectapi` is "the only place in
  the codebase that currently needs the `github.com/google/uuid`
  dependency; the `application` and `domain` layers have no reason to
  import it." That's no longer accurate — `application` now imports `uuid`
  too, for message-ID generation, a genuinely different purpose (publish
  idempotency, not aggregate identity) sharing the same dependency. ADL
  0009 itself is not rewritten (historical decisions aren't retroactively
  edited, per ADL 0014's own precedent), but this ADL supersedes that one
  specific stated consequence.
- `foundation/nats.Publisher.Publish`'s signature change is breaking,
  shared-code surface — every existing call site (`stores` and `baskets`,
  both fixed same-day as this ADL) needed updating in lockstep, the same
  class of system-wide ripple ADL 0007's addendum caused when
  `SRV_REQUEST_TIMEOUT`'s default changed.
- This relies on the stream's `Duplicate Window` remaining comfortably
  longer than `publishWithRetry`'s own worst-case total duration (~10.5s,
  per ADL 0007) — currently true with a wide margin (2 minutes, JetStream's
  default, never explicitly configured by either service). If a future
  service's retry budget were ever configured much larger, or the window
  were ever narrowed, this protection would silently stop covering the
  full retry sequence — worth remembering as an implicit dependency, not a
  guaranteed one.
- `depot` has no publish call sites yet (no domain use cases built as of
  this ADL), so it inherits this pattern by simply writing its own
  `publishDomainEvent` the same way from the start, rather than needing a
  retrofit later.
- This was caught specifically because a test verified actual stream
  _contents_ (`nats stream info`/message count) rather than only asserting
  the use case's own return value — the same "prove the property, don't
  assume it" discipline ADL 0010 and ADL 0012 each already established
  with dedicated timing tests, now extended to publish-side durability.
