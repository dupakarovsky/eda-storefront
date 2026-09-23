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

______________________________________________________________________

## Phase Plan

**Phase 1 — Order Aggregate & Persistence.** Define `domain.Order` (status enum:
`created` → `readied`/`canceled`, items with `product_id`/`store_id`/`quantity`,
a `shopping_list_id` field to be filled in later), its domain events, and the
Postgres repository — current-state rows per ADL 0002, with a concurrency
strategy we'll decide explicitly rather than default. Testable standalone, no
NATS/HTTP yet.

**Phase 2 — Contract Changes to `baskets` and `depot`.** Enrich
`BasketCheckedOut`'s payload in `baskets` (touching the `Checkout` use case,
since it needs each item's `store_id` from the catalog cache). Add `order_id` to
`depot`'s `CreateShoppingList` (domain constructor, application use case, proto
message, handler). This is where the `baskets`/`depot` source you provided gets
edited directly.

**Phase 3 — Consuming `BasketCheckedOut` → Creating the Order.** Wire up
`ordering`'s first `Subscriber` (the `baskets` stream); the use case that
projects the event into a new `Order` and publishes `OrderCreated`.

**Phase 4 — Orchestrating `depot`: `CreateShoppingList` + `AddItem`.** Build the
`depotclient` wrapper; call it from the order-creation flow to create and
populate the shopping list, storing the returned `shopping_list_id` on the
`Order`. This is where we'll work out failure-handling philosophy for a
synchronous downstream command — a new kind of gap, distinct from ADL 0007's
publish-retry gap.

**Phase 5 — Consuming `ShoppingListCompleted` → Order Readied.** Second
`Subscriber` (the `depot` stream); look up the `Order` by `shopping_list_id`,
transition it, publish `OrderReadied`.

**Phase 6 — Synchronous API Surface.** `ordering`'s own ConnectRPC server:
`GetOrder` (ADL 0008-compliant query path) and `CancelOrder` (deciding then
whether cancellation needs to reach into `depot` too).

**Phase 7 — Containerization & Live Verification.** Dockerfile,
`docker-compose.yaml`/`Makefile` wiring, migrations, and the same
live-verification discipline (curl script, concurrency test,
redelivery/idempotency tests) every prior service got.

______________________________________________________________________

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

## Phase 2 — Cross-Service Contract Edits: `baskets` and `depot` (COMPLETE)

Phase 2 executed the two breaking changes to already-live services that ADL 0016
required before `ordering` could be built against them: enriching `baskets`'
`BasketCheckedOut` event with itemized data, and adding `order_id` to `depot`'s
`CreateShoppingList`. Both were fully anticipated by name in each service's own
`CONTRACT.md` non-goals section before this phase began — no new design
questions, only the mechanics of applying a known, agreed change across domain,
application, wire format, and (for `depot`) schema and proto layers.

### Part A — `baskets`: enriching `BasketCheckedOut`

**Design decision (Step 1).** Store-ID resolution follows the same convention
`AddItem` already established: the application layer resolves catalog data, the
domain layer trusts what it's handed rather than re-validating it. Two shapes
were considered for getting resolved data into the domain — passing a
fully-built item list, or passing a `productID → storeID` map and letting domain
iterate its own `Items` — and the map (Option B) was chosen specifically to keep
`Basket.Items` as the single source of truth for "what's in this checkout," with
no parallel list that could drift out of sync.

**Domain (Step 2).** `Basket.Checkout(storeIDs map[string]string)` replaces the
old no-arg `Checkout()`. New sentinel `ErrProductNotInCatalog` guards against a
map miss — a defensive check that should never fire in practice (a product
surviving from `AddItem`-time validation to checkout should always resolve),
matching `AddItem`'s own precedent of trusting already-validated data. One
incidental improvement: the original single `b.Status != BasketOpenStatus` check
was split into an `ErrBasketAlreadyCheckedOut` check (checked first) and the
general `ErrBasketNotOpen` check — a second `Checkout` call on an
already-checked-out basket now reports the more specific error. Both still map
to `CodeInvalidArgument`, so no HTTP-visible behavior changed, only the error
message. New domain type `ItemCheckedOut{ProductID, StoreID, Quantity}`;
`BasketCheckedOut` event gained `Items []ItemCheckedOut`.

**Application (Step 3).** `Checkout.Execute` gained a
`catalogRepo catalog.Repository` dependency (interface, not the concrete
Postgres type — kept consistent with every other application-layer repository
dependency in this system). Rather than one `FindProductByID` call per basket
item, a new batched method was added to the `catalog.Repository` interface:
`FindStoreIDsByProductIDs(ctx, productIDs []string) (map[string]string, error)`,
implemented via a single `WHERE id = ANY($1)` query — avoids N round-trips for a
basket with N items, and returns exactly the map shape `Basket.Checkout` needs.
Runs inside the existing `WithLock` transaction, same scoping `AddItem`'s
product validation already uses. `baskets.go`'s `NewCheckout` call site updated
with the new arg — a reminder that a use case's constructor signature change
always ripples to its module wiring, not just its own file.

**Wire format & contract (Step 4).** `events/payload.go` gained
`ItemCheckedOutPayload`/updated `BasketCheckedOutPayload` matching the
`product_id`/`store_id`/`quantity` shape ADL 0016 specified.
`baskets-CONTRACT.md` updated: documented payload now shows the item list, and
the "known limitation: carries no item list" note (present since Phase 3 of the
original `baskets` build) is resolved and removed.

**Verification.** Domain suite: 5/5 `TestBasket_Checkout` subtests + existing
suite, all passing. Application suite (real Postgres/NATS):
`TestCheckout_Execute_Succeeds` (multi-store case — two products under two
different stores, confirms the map correctly disambiguates per-store resolution)
and `TestCheckout_Execute_RejectsEmptyBasket`, both passing, with NATS logs
confirming `BasketCheckedOut` actually published. One gap noted but deliberately
left open: no application-layer test exercises `ErrProductNotInCatalog`'s
propagation through `Execute` — the only way to trigger it for real requires a
product added then removed from the catalog before checkout, and the error is
already thoroughly covered at the domain level. Worth revisiting if `stores`'
`RemoveProduct` path becomes more directly exercised in a future project.

### Part B — `depot`: adding `order_id` to `CreateShoppingList`

**Domain (Step 5).** `NewShoppingList(id, orderID)` now requires a non-blank
`orderID` (new sentinel `ErrOrderIDIsBlank`, following the same pattern as
`ErrStoreIDIsBlank`/`ErrProductIDIsBlank` in the same file).
`ShoppingList.OrderID` and `ShoppingListCreated.OrderID` both added;
`ReconstituteShoppingList` signature updated to match. All 24 existing
`NewShoppingList("sl-1")` call sites in `shopping_list_test.go` needed a second
argument — mechanical, since none of those tests' actual assertions depend on
`OrderID`. Two new tests added: one confirming `OrderID` round-trips onto both
the struct and the event, one confirming the blank-`orderID` guard. Full 26-test
domain suite green.

**Application, proto, connectapi, Postgres (Step 6).**
`CreateShoppingList .Execute(ctx, id, orderID)` signature updated. Proto:
`ShoppingList` message gained `order_id` (so responses echo it back, not just
requests — kept symmetric with the domain struct and useful for downstream
debugging/ verification without cross-referencing); `CreateShoppingListRequest`
gained `order_id`. Regenerated via `buf generate` per ADL 0003's tooling choice.
`connectapi/errors.go`'s `mapError` gained a case for `ErrOrderIDIsBlank` →
`CodeInvalidArgument` (previously would have fallen through to `CodeInternal`,
which is wrong for a caller-input error). New migration
(`000002_add_order_id_to_shopping_lists`) adds
`order_id TEXT NOT NULL DEFAULT ''` — `DEFAULT ''` chosen specifically to avoid
breaking existing test rows from prior phases, matching `ordering`'s own
established empty-string-means-unset convention for its `shopping_list_id`
column. Postgres repository's `Save`/`find` both updated for the new column;
`order_id` is deliberately never touched on `Save`'s `ON CONFLICT` update clause
— immutable once set at creation, same as `id` itself.
`create_shopping_list_test.go` updated for the new signature plus an `OrderID`
round-trip assertion.

**Contract.** `depot-CONTRACT.md` updated: `CreateShoppingList`'s row now
documents `{order_id}` as the request shape; `ShoppingListCreated`'s payload now
shows `order_id`; the "No `orderID` on `ShoppingList`" non-goal (present since
`depot`'s original build, explicitly flagged there as provisional pending this
exact project) is replaced with a note describing the breaking change, the
`DEFAULT ''` fallback for pre-existing rows, and the immutability guarantee.

**Verification.** Full 16-test application suite green (every use case depending
on `CreateShoppingList` under the hood — `AddItem`, `Cancel`, `Complete`,
`Initiate`, `RemoveItem`, `Get` — passed without direct modification, confirming
the shared test setup path absorbed the new required argument cleanly). Live
curl script run against the full containerized stack (`stores` + `depot` +
Postgres + NATS): full lifecycle (create → add → add-duplicate →
add-second-store → remove → initiate → blocked-add-after-initiate → complete →
blocked-cancel-after-complete) confirmed working end-to-end, with `orderId`
round-tripping correctly through every response. Two run-throughs were needed —
the first used a stale `curl -d '{}'` (pre-ADL-0016 request shape) for
`CreateShoppingList`, which correctly triggered `ErrOrderIDIsBlank` and cascaded
`not found` through every dependent step; this confirmed the guard works
end-to-end through the real HTTP/Connect layer, not just in unit tests, even
though it wasn't the run's original intent. Script fixed to send a real
`order_id`, second run fully green.

## State at End of Phase 2

- `baskets` and `depot` both remain "Live" in `system-state.md` — no service was
  taken down or newly stood up this phase, only two already-live services'
  contracts changed in place.
- `baskets-CONTRACT.md` and `depot-CONTRACT.md` are both now accurate and
  current — the two provisional/known-limitation notes that existed specifically
  to flag this future work are both resolved.
- `ordering`'s Phase 1 work (the `Order` aggregate, built against the
  _pre_-Phase-2 assumption that these contracts would eventually look this way)
  is now unblocked: `depotclient` (Phase 3) can be built against
  `CreateShoppingList`'s real, current signature rather than a forward reference
  to a not-yet-applied ADL.
- No changes to `stores`, `foundation`, or any other service this phase —
  confirmed by ADL 0016's own scoping ("nothing else in `baskets`/`depot`/
  `stores` changes").

## What Carries Into Phase 3

Phase 3 wires `ordering`'s HTTP/NATS/module layer and builds the orchestration
logic itself — the actual point of Project 3, per ADL 0016's Design A:

1. **`depotclient`** — a ConnectRPC client wrapping `depot`'s now-final
   `CreateShoppingList(order_id)` and `AddItem` (unit-at-a-time, per ADL 0016's
   decision to leave `AddItem` untouched rather than add a `quantity`
   parameter).
1. **Two concurrent NATS subscribers** — `baskets`' `BasketCheckedOut` (now
   carrying the item list Phase 2 added) and `depot`'s `ShoppingListCompleted` —
   extending `foundation/shutdown`'s multi-listener `Service` pattern (ADL 0012)
   to three concurrent listeners (HTTP + two subscribers) for the first time in
   this system.
1. **`CreateOrder` orchestration**: consume `BasketCheckedOut` → construct
   `Order` from its now-itemized payload → call `depot.CreateShoppingList`
   synchronously → call `depot.AddItem` once per unit → store the returned
   `shopping_list_id` on the `Order` row.
1. **`CompleteOrder`**: consume `ShoppingListCompleted` → correlate back to the
   `Order` via the stored `shopping_list_id` (Phase 1 deferred the
   `FindByShoppingListID` lookup/index to exactly this point — now concrete) →
   `MarkReadied()`.
1. A new failure mode this system hasn't had to handle yet, per ADL 0016's own
   Consequences: a downstream synchronous command
   (`depot.CreateShoppingList`/`AddItem`) failing or timing out
   mid-orchestration — distinct from ADL 0007's publish-failure gap, and needs
   an explicit decision in Phase 4 rather than being assumed away.
