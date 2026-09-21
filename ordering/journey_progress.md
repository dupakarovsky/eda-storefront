# 🟢 Ordering Service — Project Plan

## Project Concept

`ordering` is the fan-in orchestrator of the system: it turns a checked-out
basket into a real `Order`, drives `depot` to build the fulfillment shopping
list for it, and advances the order to "ready" once that list is completed —
publishing its own events (`OrderCreated`, `OrderReadied`, `OrderCanceled`) for
`payments` (Project 4) to consume later. It's the first service in this series
that is both an event consumer and a synchronous client of another live service,
and the first to require coordinated changes to two already-shipped,
already-running services rather than building purely against a frozen contract.

## Concepts We'll Cover

Running two independent NATS JetStream subscribers inside one service (extending
`foundation/shutdown`'s multi-listener `Service` pattern from ADL 0012 to three
concurrent listeners: HTTP + two subscribers). Writing a ConnectRPC client for
the first time — calling `depot`'s live API from `ordering`'s own application
layer, rather than only ever being called. Orchestration: an aggregate whose
creation triggers a real side-effecting command in a different service, and
whose next state transition is driven by a completely different consumed event
than the one that created it. Making a deliberate, minimal, ADL-documented
breaking change to two live services' contracts. And reapplying, at higher
composition complexity than any single- subscriber service so far, everything
already established: per-aggregate concurrency strategy (ADL 0002),
context-scoped transaction locking (ADL 0010), retrying + idempotent publish
(ADL 0007/0015), bounded redelivery and dead-lettering (ADL 0013).

## Folder Structure

```txt
ordering/
├── journey_progress.md
├── go.mod
├── Dockerfile
├── migrations/
├── orderingpb/
│   └── api.proto
├── cmd/service/
│   └── main.go
├── ordering.go                    (Module — mirrors baskets.go/depot.go)
└── internal/
    ├── domain/
    │   ├── order.go
    │   ├── order_events.go
    │   ├── order_status.go
    │   └── order_repository.go
    ├── application/
    │   ├── create_order.go        (BasketCheckedOut → Order + depot orchestration)
    │   ├── complete_order.go      (ShoppingListCompleted → readied)
    │   ├── cancel_order.go
    │   ├── get_order.go
    │   ├── publish_event.go
    │   └── publish_retry.go
    ├── depotclient/
    │   └── client.go              (wraps depot's ConnectRPC client)
    ├── events/
    │   ├── envelope.go
    │   ├── payload.go
    │   ├── stream.go
    │   └── subjects.go
    ├── connectapi/
    │   ├── server.go
    │   ├── order.go
    │   ├── proto.go
    │   └── errors.go
    └── postgres/
        ├── interface.go
        ├── txcontext.go
        └── order_repository.go
```

## Design Decisions Made Before Phase 1

**Item-list gap (`BasketCheckedOut`/`ShoppingListCompleted`):** resolved by
enriching events rather than synchronous read-back, to stay consistent with the
system's event-driven philosophy. See ADL 0016.

**Fan-in correlation ("Design A" vs "Design B"):** resolved as orchestration
(`ordering` synchronously calls `depot.CreateShoppingList`/`AddItem`) rather
than choreography (a new `depot` subscriber reacting to an `ordering` event).
Chosen to minimize rewiring of already-live `depot`/`baskets` services. See ADL
0016\.

**`ShoppingListCompleted` needs no payload change.** Under Design A, `ordering`
already knows the order's contents (it supplied them to `depot` in the first
place) and only needs to know _which_ order is ready — already answered by the
event's existing aggregate ID (`shopping_list_id`), which `ordering` stores on
its own `Order` row.

**`depot`'s `AddItem` stays untouched.** `ordering` calls `AddItem` once per
unit rather than passing a `quantity`, avoiding a signature change to an
already-live method.

## Phase 1 — Order Aggregate & Persistence (COMPLETE)

**Step 1 — `Item`, `OrderStatus`, `Order` struct.** `OrderStatus` has three
values (`created`/`readied`/`canceled`), not four like `baskets`/`depot` — no
`pending`/`open` stage, since an `Order` is always created already
fully-populated from `BasketCheckedOut` rather than built up one `AddItem` call
at a time. `Item` carries `StoreID`, unlike `Basket.Item`, since `ordering` must
supply the store grouping key to `depot.AddItem` per unit (ADL 0016's "call it N
times" decision). `Items` stays a flat slice, not a map keyed by store like
`ShoppingList.Stops` — that grouping is `depot`'s concern, not `Order`'s.

**Step 2 — Domain events and aggregate behavior.** `OrderCreated`,
`OrderReadied`, `OrderCanceled` events, same `eventBase`/`Event` interface shape
as `baskets`/`depot`. Behavior methods: `NewOrder`, `LinkShoppingList`,
`MarkReadied`, `Cancel`. Key decision: `LinkShoppingList` is a plain
`error`-only method, not event-emitting — recording the `shopping_list_id` is
orchestration bookkeeping, not a customer-facing state change.
`ReconstituteOrder` is the first reconstitution function in the system that
rehydrates something beyond status and items (the `shoppingListID` field).

**Step 3 — Domain test file.** `order_test.go`, mirroring `basket_test.go`'s
`t.Run`-per-guard-condition structure. Surfaced two real gaps during review,
both closed before the step was considered done:

- `NewOrder` now guards against an empty `items` slice (`ErrOrderIsEmpty`) —
  previously trusted the caller (an already- checked-out, therefore non-empty,
  basket) rather than enforcing its own invariant, inconsistent with every other
  aggregate in the system.
- `MarkReadied` now guards against being called before a shopping list is linked
  (`ErrShoppingListNotLinked`) — the guard runs _after_ the
  already-readied/already-canceled status checks, not before, so a
  terminal-status order reports its actual status rather than a misleading "not
  linked."
- Adding the `MarkReadied` linkage guard meant `TestOrder_MarkReadied`'s and
  `TestOrder_Cancel`'s `"already readied"` subtests needed an explicit
  `LinkShoppingList` call before the first `MarkReadied()`, since reaching
  "already readied" now requires a legitimate linked-then-readied sequence
  rather than two bare `MarkReadied()` calls. Caught by the user running the
  suite, not anticipated in advance — worth remembering that a new guard on one
  method can silently invalidate an existing test's setup for a _different_
  method's test, if that test happened to be relying on the now-guarded method
  to reach its own state.
- Full suite green after fixes (`go test -v ./ordering/internal/domain/`).

**Step 4 — `OrderRepository` interface + concurrency strategy.** Interface shape
mechanical (matches `BasketRepository`/`ShoppingListRepository`). Real decision:
row-level locking (`SELECT ... FOR UPDATE` via `WithLock`), same mechanism
`baskets`/`depot` use — but for a different underlying reason. `Order` has no
direct-user-action concurrent-write scenario the way `Basket`/`ShoppingList` do;
its real concurrency source is JetStream redelivery (ADL 0013) — two
near-simultaneous deliveries of the same consumed event landing on the same
`Order` row. Optimistic version-column compare-and-swap was considered and
rejected: it suits scenarios where a conflict should fail loudly for a client to
retry (UI-facing), whereas here a "conflict" (duplicate delivery) should resolve
silently and correctly via blocking, which a lock does more naturally.

**Step 5 — Postgres repository implementation.** `orders` + `order_items` tables
(migration), `interface.go`/`txcontext.go` ported verbatim from `depot` (ADL
0010's pattern, nothing service-specific to change), `order_repository.go`
(`Save`/`FindByID`/`FindByIDForUpdate`/ `WithLock`). `shopping_list_id` stored
as `NOT NULL DEFAULT ''`, matching the domain's own empty-string-means-unlinked
convention rather than introducing a separate `NULL` state. Two deliberate
deferrals, not gaps:

- `Save` still does full delete-then-reinsert of `order_items` on every call,
  even though `Order.Items` never actually changes post-creation (no
  `AddItem`/`RemoveItem` equivalent exists for `Order`) — kept generic rather
  than special-cased, since `Save` is called uniformly by every use case and
  none of them know whether items changed. Revisit only if this proves costly at
  real write volume.
- No `FindByShoppingListID` / no index on `shopping_list_id` yet, despite Phase
  5's `CompleteOrder` needing exactly that lookup to correlate
  `ShoppingListCompleted` back to an `Order`. Deliberately deferred to Phase 5
  Step 5, when the need is concrete, rather than spec'd speculatively now.
- Per this project's established precedent (confirmed with the user): repository
  correctness is _not_ verified by a dedicated repository-test file — it gets
  proven indirectly and for real once Phase 3's `CreateOrder` use case exercises
  it against live Postgres, the same way `stores`/`baskets`/`depot`'s
  repositories were verified.
- `go build ./...` clean at Phase 1 close.

## State at End of Phase 1

- `domain` package complete and fully unit-tested (12 subtests across 4
  top-level test functions, all green).
- `postgres` package complete, compiles, **not yet exercised against a real
  database** — that verification is expected to happen naturally in Phase 3, not
  before.
- No NATS, no HTTP, no `connectapi`, no `cmd/service/main.go`, no `ordering.go`
  module wiring yet — all of that starts in later phases.
- `depot`'s `AddItem` confirmed to need zero changes; the only two cross-service
  contract edits remaining are `baskets`' `BasketCheckedOut` payload and
  `depot`'s `CreateShoppingList` signature — both already named in their
  respective `CONTRACT.md` non-goals sections before this project began.

## What Carries Into Phase 2

Phase 2 touches `baskets` and `depot` directly — this is the point where their
actual source (proto, domain aggregate, application use case) gets edited, not
just referenced. Two edits, and per our own scoping conversation, nothing else
in either service should move:

1. `baskets`' `Checkout` use case: enrich `BasketCheckedOut`'s payload with the
   basket's items, each item's `store_id` resolved from `baskets`' own
   `catalog_products` cache at checkout time (since `Basket.Item` itself carries
   no store reference today).
1. `depot`'s `CreateShoppingList`: add `order_id` — domain constructor,
   application use case, proto message, connect handler.
