# docs/adl/0018-rollback-uses-independent-context.md

# 18. Transaction rollback uses an independent, freshly-timed context, not the caller's

Date: 2026-09-30

## Status

Accepted

## Context

Every service's `WithLock` implementation (`stores`, `depot`, `baskets`,
`ordering`) shares an identical shape: begin a transaction, derive `txCtx`
from the caller's own `ctx` via `context.WithValue`, run the callback, and on
failure call `tx.Rollback(txCtx)`. Since `txCtx` only adds a value to `ctx`
and inherits its deadline unchanged, a callback failure caused by `ctx`
itself expiring left `Rollback` holding an already-canceled context —
`pgx` refuses to attempt any I/O against a canceled context, so the rollback
failed immediately with its own error, compounding the original one rather
than actually telling Postgres to release the transaction.

This surfaced concretely during `ordering`'s Phase 6 `CancelOrder` work,
though as a symptom of a different bug (a `Save` call using the outer `ctx`
instead of `txCtx`, causing a same-request self-deadlock across two pooled
connections) — once that root cause was fixed, this rollback-context gap
remained as a separate, latent issue, confirmed by code inspection to be
present identically in every service's `postgres` package.

## Decision

`WithLock`'s rollback path uses a short-lived context derived from
`context.Background()`, not from the caller's `ctx`, so a rollback attempt is
never handed an already-expired deadline:

```go
if err := fn(txCtx); err != nil {
    rbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if rbErr := tx.Rollback(rbCtx); rbErr != nil {
        return fmt.Errorf("rolling back transaction after error %q: %w", err, rbErr)
    }
    return err
}
```

Applied identically across `stores`, `depot`, `baskets`, and `ordering`'s
`postgres` packages — the same shape, same 5-second budget, no
service-specific variation.

## Consequences

- Every future service's `WithLock` should be written with this shape from
  the start, the same way ADL 0013's redelivery values and ADL 0010's
  context-scoped `Executor` pattern are now defaults to follow rather than
  rediscover.
- This was found by code inspection, not by a dedicated test proving the
  property — unlike ADL 0010's `TestStoreRepository_WithLock_SerializesConcurrentAccess`,
  which deliberately proved its own claim with a timing test. No equivalent
  test exists yet proving a rollback actually completes when the original
  context has expired; worth treating as a real gap in verification
  discipline, not just accepting the fix on inspection alone, if a future
  phase has room for it.
- The 5-second budget is a reasonable default, not a measured one (unlike
  ADL 0007's 1.5-second publish-attempt timeout, which came from a real
  forced-outage test) — if rollback ever needs a different budget in
  practice, that's new information to fold back into this ADL, not a sign
  the number was wrong from the start.

<!-- docs/adl/0018-rollback-uses-independent-context.md (append) -->

## Consequences (addendum, 2026-10-10)

- The verification gap this ADL named is closed for `customers`:
  `TestCustomerRepository_WithLock_RollsBackAfterCallerContextExpires` proves
  rollback completes after the caller's context is canceled, and fails when the
  rollback uses the caller's context (experiment run). The other four services
  have the same fix but no equivalent test.
- A related defect surfaced during `customers`: a use case calling
  `Save(ctx, ...)` instead of `Save(txCtx, ...)` inside `WithLock` self-deadlocks
  (the write goes through a second pooled connection and blocks on the row lock
  the transaction holds). The compiler cannot see it; running tests with
  `-timeout 30s` turns it into a fast failure.
