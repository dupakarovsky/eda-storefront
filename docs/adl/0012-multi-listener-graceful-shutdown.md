# 12. Multi-listener graceful shutdown via a `Service` interface

Date: 2026-09-02

## Status

Accepted

## Context

`foundation/shutdown.Run` was built during the `foundation` project
against a single `*http.Server`, and flagged at the time as untested for
anything else: `01-stores` (HTTP + NATS _publish_ only) never exercised a
second concurrent listener, so the gap stayed open and unverified. This
project (`02-depot-baskets`) is the first to add a second one — the NATS
JetStream `Subscriber`'s consume loop — making the gap concrete rather
than hypothetical.

The source book's own answer to this (`internal/waiter`, Chapter12) was
reviewed directly rather than assumed: a signal-scoped `context.Context`
shared across an `errgroup.WithContext`, each listener expressed as a
`WaitFunc(ctx) error` that blocks until `ctx.Done()`. Notably, the book's
version carries no shutdown _policy_ of its own — no grace-period-then
-force-close timing; that's left entirely to each `WaitFunc`'s own body.
Our existing `Run` is the opposite: single-listener, but with real,
already-proven policy (explicit timeout, `Shutdown` then `Close`
fallback).

## Decision

Generalize our own policy to N listeners rather than adopt the book's
`errgroup`/`WaitFunc` shape, to minimize the change in `foundation`'s
public surface:

```go
type Service interface {
    Name() string
    Run() error
    Shutdown(ctx context.Context) error
    Close() error
}
```

`Run(logger, shutdownTimeout, services ...Service)` starts every service's
`Run()` in its own goroutine, feeding one shared `errs` channel sized
`len(services)`. Shutdown triggers on whichever comes first — a signal, or
any service's `Run()` returning an error — and **both paths fall through
to the same shutdown sequence**. Every service's `Shutdown(ctx)` is then
broadcast concurrently against one shared deadline (not sequentially,
which would silently starve later services of most of the configured
timeout), each with its own independent `Close()` fallback if its
`Shutdown` errors or overruns.

`httpService` (`NewHTTPService(name, logger, srv)`) is the first, and so
far only, concrete adapter — a thin wrap of `*http.Server`'s existing
`Shutdown`/`Close` methods. The NATS `Subscriber` (Phase 2) is designed to
implement the same interface from the start: `Shutdown` calls
`ConsumeContext.Drain()` (stop fetching, finish in-flight, then ack);
`Close` calls `Stop()` (abandon in-flight, unacked — safe here because
every handler is an idempotent cache upsert, so JetStream redelivery on
reconnect is a correctness non-event, not a failure mode).

The shared `*nats.Conn` itself is deliberately **not** wrapped as a
`Service` — a `Subscriber.Close()` closing the connection out from under
an in-flight `Publisher` retry (ADL 0007) would be a real cross-listener
coupling bug. `main()` calls `nc.Close()` once, directly, only after
`Run()` returns.

## Alternatives considered

**Adopt the book's `errgroup` + `WaitFunc` shape.** Would have meant
re-embedding our grace-then-force-close policy inside every individual
`WaitFunc` body instead of once, centrally, in `Run`. Rejected specifically
to keep the policy centralized and `foundation`'s public surface change
minimal — the `Service` interface is additive, not a rewrite of `Run`'s
contract.

## Consequences

- Real, structurally-invisible-with-one-service bugs were caught and fixed
  during implementation, not found later: a non-deferred `wg.Done()` (made
  `Run` return before shutdown actually completed — the same "lock that
  never engaged" failure class as ADL 0010), a duplicate `Close()` call on
  the failure path, and — the most consequential — the `errs`-branch
  originally `return`ing immediately instead of falling through to the
  shared shutdown sequence, which would have left every _other_ service
  (e.g. a live HTTP server) never notified that one of its siblings had
  died. Caught by writing a real timing-based test
  (`TestRun_WaitsForInFlightRequestBeforeReturning`) and deliberately
  proving it fails against the broken version before trusting it against
  the fixed one — same discipline ADL 0010 established.
- **Known, accepted limitation:** the `errs` channel only ever reports the
  _first_ service failure through `runErr`/the logs. If two services fail
  near-simultaneously, or one fails during a shutdown already triggered by
  another, the second error is silently absorbed into the (now
  correctly-sized) buffer and never read or logged. Not a goroutine leak,
  but real information loss in a multi-failure scenario. Deferred
  deliberately, in the same spirit as ADL 0007's outbox deferral — worth
  closing before this becomes a debugging blind spot, not forgotten.
- Every future service registers by implementing `Service` and passing
  itself into `Run(...)` — this is now the default pattern, not something
  to re-derive per service.
- `01-stores`' call site changed from
  `shutdown.Run(logger, srv, cfg.Server.ShutdownTimeout)` to
  `shutdown.Run(logger, cfg.Server.ShutdownTimeout, shutdown.NewHTTPService("STORES", logger, srv))`
  — a one-line adaptation, not a rewrite, confirming the surface-area goal
  was met.
