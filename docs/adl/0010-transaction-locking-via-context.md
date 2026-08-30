# 10. Transaction locking via context-scoped Executor, not an explicit TxManager

Date: 2026-08-27

## Status

Accepted

## Context

Phase 2 wired row-level locking (`SELECT ... FOR UPDATE`) into both
repositories but left it "wired but dormant" — every repository method ran
against the bare `*pgxpool.Pool` regardless of any surrounding
`WithLock`/`WithTx` call, since `WithLock`'s original implementation never
actually threaded the transaction it began into the calls made inside its
own callback. Phase 4 Step 7 needed to make the lock genuinely take effect,
since every one of the seven mutating command use cases depends on it to be
correct under concurrent access — confirmed as a real gap by a dedicated
concurrency test (`TestStoreRepository_WithLock_SerializesConcurrentAccess`)
that failed against the dormant version and passed once fixed.

Two structurally different fixes were considered, both requiring a real
`pgx.Tx` (obtained via `Executor.Begin`) to actually reach the repository
methods called inside a locked section:

**Context-scoped `Executor` (adopted).** `WithLock` stores the transaction
in the `context.Context` it passes into its callback
(`ctxWithTx`/`executorFrom` in `internal/postgres/txcontext.go`); every
repository method resolves its executor by checking the context first,
falling back to the bare pool. Every existing call site
(`uc.storeRepo.WithLock(ctx, func(txCtx context.Context) error { ... })`,
written across all seven use cases before this fix was finalized) needed no
changes at all — only `WithLock`'s own internals and every repository
method's executor resolution needed correcting.

**Explicit `TxManager`.** A `domain.TxManager` interface with a single
`WithTx(ctx, func(ctx context.Context, storeRepo StoreRepository, productRepo
ProductRepository) error) error` method, handing the callback tx-scoped
repository instances as arguments rather than relying on context lookup.
Makes the transactional boundary visible in the callback's own signature —
impossible to accidentally call a non-transactional repository instance
inside a locked section, since the closure is never given one. Prototyped in
full (all seven use cases rewritten to the new callback shape, a
`postgres.TxManager` implementation, `module.go` rewiring) before being set
aside.

## Decision

Adopt the context-scoped `Executor` pattern. The deciding factor was not the
implicitness trade-off itself — that cost is real and was named explicitly,
not dismissed — but that `TxManager.WithTx`'s signature requires handing
every callback both a `StoreRepository` and a `ProductRepository` regardless
of which aggregate the calling use case actually concerns. A `Store`-only
handler (`RebrandStore`, `EnableStoreParticipation`) would receive a
`ProductRepository` it never uses, and vice versa — a cross-aggregate
coupling in the transactional boundary itself that the rest of this
project's design has otherwise kept strictly separated per aggregate (see
ADL 0002's per-aggregate concurrency-strategy mandate). The context-scoped
approach has no equivalent coupling: `StoreRepository.WithLock` only ever
concerns itself with `StoreRepository`'s own methods.

## Alternatives considered

**`TxManager` with per-aggregate variants** (a `StoreTxManager` and
`ProductTxManager`, each scoped to one repository) was not prototyped, but
would resolve the cross-aggregate coupling objection while keeping
`TxManager`'s explicitness. Left unexplored because the context-scoped fix
was already sufficient and required zero rewriting of already-correct call
sites — but worth remembering as a real, not-yet-tried third option if the
implicitness cost of the adopted pattern turns out to matter more in
practice than expected (e.g. once `depot`/`baskets` or `ordering` need
genuine multi-aggregate transactions, which `01-stores` never has, since
`Store` and `Product` are never mutated together in one use case here).

## Consequences

- `internal/postgres/txcontext.go`'s `ctxWithTx`/`executorFrom` pair is now
  load-bearing, not incidental — any new repository method added to either
  `StoreRepository` or `ProductRepository` must resolve its executor via
  `executorFrom(ctx, r.db)`, never `r.db` directly, or it will silently run
  outside any surrounding lock. This is easy to get wrong by omission and
  was already caught once during Step 7's review of `product_repository.go`,
  `stores_repository.go`.
- Proven correct via a real concurrency test measuring actual blocked time
  (`TestStoreRepository_WithLock_SerializesConcurrentAccess`), not just
  final-state assertions — a lock that never engaged and a lock that
  engaged correctly can produce identical final database state under a
  single-threaded test, so this distinction required deliberately proving
  a timing property, not just a data property.
- If a future service's transactional needs turn out to genuinely require
  crossing aggregate boundaries in one transaction — something `01-stores`
  itself never needed — the per-aggregate-`TxManager` alternative above, or
  reopening this decision entirely, is the place to start, not a silent
  reintroduction of context-based locking's implicitness cost without
  remembering why it was accepted here.
