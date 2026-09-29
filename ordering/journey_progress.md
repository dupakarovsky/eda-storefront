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

## Phase 3 Summary — COMPLETE

**Status:** Complete

### What Was Built

The `ordering` service went from domain-and-repository-only (Phase 1) to a
genuinely running, NATS-consuming service for the first time.

- **`internal/events`** (`envelope.go`, `payload.go`, `subjects.go`,
  `stream.go`) — enveloping/marshaling infrastructure for `ordering`'s own
  domain events, mirroring `depot`'s/`baskets`' identical packages. Payload
  mappings exist for all three of `Order`'s events (`OrderCreated`,
  `OrderReadied`, `OrderCanceled`), even though this phase only publishes
  `OrderCreated` — the mapping is purely mechanical and costs nothing to
  complete now.
- **`internal/application/publish_event.go` + `publish_retry.go`** — the same
  retry-with-backoff, idempotent-msgID publish helper pattern established by ADL
  0007/0015, copied in shape from `depot`'s.
- **`internal/application/create_order.go`** (`CreateOrder` use case) —
  constructs and saves a new `Order` from a basket's items, publishes
  `OrderCreated`. Does **not** yet call `depot` — `ShoppingListID` is left blank
  by design; that's Phase 4's job.
- **`internal/baskets/handler.go`** — the translation boundary consuming
  `baskets`' `BasketCheckedOut` event. Lives in its own package (`baskets`,
  named for the upstream service it translates, not the action it performs),
  matching `depot`'s/`baskets`' own precedent of pulling this kind of code out
  of `application` and into a dedicated package. Defines its own local
  envelope/payload types rather than importing `baskets`' internal package, per
  ADL 0004.
- **`ordering.go`** (`Module`) — wires Postgres, NATS, the publisher, the one
  `baskets`-stream subscriber, and `CreateOrder` together.
- **`cmd/service/main.go` + `health.go`** — the service's entry point.
  Two-listener graceful shutdown (`ORDERING-HTTP` + the `baskets` subscriber)
  via `foundation/shutdown`'s `Service` interface (ADL 0012) — the
  three-listener case (a second subscriber for `ShoppingListCompleted`) arrives
  in Phase 5.
- **Tests, at every layer, against real Postgres/NATS**: `events` package
  (envelope/payload round-trip for all three events), `CreateOrder` (create
  succeeds, empty-items rejection, and idempotency-under-redelivery), and the
  `baskets.Handler` (success, unknown-event skip, malformed-envelope error,
  malformed-payload error).

### Issues Found & Resolved

- **`publishWithRetry`'s loop condition was inverted**
  (`attemp >= publishMaxAttempts` instead of `<=`), making the retry loop never
  execute and causing `publishDomainEvent` to dereference a nil
  `*jetstream.PubAck`. Caught immediately by `TestCreateOrder_Execute_Succeeds`
  — a real panic on first run, not a silent wrong-answer bug — and fixed to
  `<=`, matching every other `publishWithRetry` in the codebase.
- **Redelivery was not idempotent.** `baskets.Handler` originally minted a fresh
  `uuid.NewString()` on every invocation, so a genuine JetStream redelivery of
  one logical `BasketCheckedOut` (ADL 0013's at-least-once guarantee makes this
  a real possibility, not hypothetical) would have created a second, distinct
  `Order` for the same checkout. Resolved by deriving the `Order`'s ID
  deterministically from the basket's own `AggregateID`
  (`id := env.AggregateID`), plus a check-before-create guard in
  `CreateOrder.Execute` (`FindByID` first; if found, return the existing order
  and skip both the domain constructor and the publish entirely). This is a
  real, permanent modeling commitment — an `Order`'s ID is now always identical
  to the `Basket` ID that produced it — made deliberately, not as an incidental
  side effect. Proven by a dedicated test
  (`TestCreateOrder_Execute_IsIdempotentOnRepeatedID`) that asserts the
  underlying NATS stream's message count directly, the same
  verify-the-stream-not-just-the-return-value discipline ADL 0015 used to catch
  the original duplicate-publish bug this system already knows about.
- **First-run stream replay was observed, not a bug.** A brand-new durable
  consumer with no prior position defaults to `DeliverAll`, so `ordering`'s
  first startup replayed the entire historical `baskets` stream. Every
  historical event with no handler mapping was correctly skipped (`WARN`-logged,
  acked as a no-op) — this is `depot`'s own `catalog.Project` precedent for "we
  don't care about this event," working as intended.

### Known Gap Found — Not Fixed This Phase

While observing that same replay, one historical `BasketCheckedOut` message
(predating Phase 2's item-enrichment) correctly failed domain validation
(`ErrOrderIsEmpty`), exhausted redelivery, and fell into the dead-letter path —
but **the dead-letter publish itself failed** with
`nats: no response from stream`. Root cause: `events.OrderingStreamSubjects`
only declares `ordering.order.*`; `foundationnats.PublishDeadLetter` publishes
onto `ordering.deadletter.<original-subject>`, which no stream currently owns.
The message still terminates cleanly (`msg.Term()` runs unconditionally
regardless of the dead-letter publish's own success), so nothing loops forever —
but the dead-lettered payload is silently lost rather than durably stored,
defeating ADL 0013's actual purpose.

**Confirmed to affect `depot` and `baskets` identically** — both services' own
`*StreamSubjects` declarations have the same gap (no `<service>.deadletter.*`
entry), so this is a systemic, cross-service issue, not an `ordering`-specific
bug. Fixing `depot`/`baskets` is out of this project's boundary per ADL 0004 and
is not addressed here — flagged explicitly so it isn't silently lost, the same
discipline this system has applied to ADL 0007's outbox deferral and other
named-but-deferred gaps.

### State at End of Phase 3

- `ordering` is a real, running service: `go run ./ordering/cmd/service` (or
  `make ordering/run`) starts Postgres/NATS connections, ensures the `ordering`
  stream, starts the HTTP health endpoint on `:8083`, and consumes `baskets`'
  stream for real, live `BasketCheckedOut` events.
- Every use-case layer built this phase (`events`, `CreateOrder`,
  `baskets.Handler`) is proven against real infrastructure via passing tests,
  not just plausible by inspection — continuing the test-alongside- code
  discipline adopted partway through this phase.
- No live, end-to-end verification against a real `baskets` checkout (triggered
  through `eda-baskets` itself) has been performed yet — deliberately deferred
  until the RPC server/client exist (Phase 4/6), so it can be part of a fuller
  verification script rather than a one-off manual check now.
- No `connectapi`, no `orderingpb`, no `depotclient` yet —
  `Order. ShoppingListID` stays blank for every order created so far, by design.
- `orderingdb` currently contains at least one `Order` row created from the
  historical replay during first startup,

\--

## Phase 4 Summary — COMPLETE

**Status:** Complete (core orchestration logic; live end-to-end verification
deferred to Phase 6/7)

### What Was Built

`ordering` became a synchronous ConnectRPC _client_ for the first time,
orchestrating shopping list creation on `depot` as the final step of order
creation.

- **`internal/domain/depot_client.go`** — the `DepotClient` port:
  `CreateShoppingList`, `AddItem`, `GetShoppingList`, following the same
  interface-in-`domain`/implementation-elsewhere convention already used for
  `OrderRepository`.
- **`internal/depotclient/client.go`** — the adapter, wrapping `depot`'s
  generated `depotpbconnect.DepotServiceClient` (imported directly via
  `go.work`, Option A from Step 1's design discussion). Translates wire types to
  plain Go values at the boundary; `GetShoppingList` in particular flattens
  `depot`'s nested `Stops`/`Items` response into a flat
  `map[productID]quantity`.
- **`foundation/config.go`** — gained `DepotConfig{URL string}` (`DEPOT_URL` env
  var), following the same per-package `Config` shape
  `postgres.Config`/`foundationnats.Config` already use.
- **`ordering.go`** — `Module` gained a `DepotClient domain.DepotClient` field,
  constructed from `cfg.Depot.URL`.
- **`internal/application/create_order.go`** — `CreateOrder.Execute`'s
  idempotency check changed from "does the order exist" to "is `ShoppingListID`
  already set," since Phase 3's original guard would have silently skipped
  retrying the `depot` orchestration entirely on redelivery. A new private
  `orchestrateShoppingList` helper runs `CreateShoppingList` → `GetShoppingList`
  (to compute per-product deltas) → `AddItem` only for the remaining units →
  `LinkShoppingList` → `Save`, called from both the fresh-creation path and the
  existing-but-unlinked retry path.
- **`depot`'s side (out of `ordering`'s own boundary, but required to make the
  above safe):** `ShoppingListRepository` gained `FindByOrderID`;
  `CreateShoppingList.Execute` now checks it first and returns the existing list
  on a hit rather than always constructing a new one.
- **`docker-compose.yaml`** — `ordering`'s service block gained
  `DEPOT_URL: http://depot:8080` (internal Docker network address, distinct from
  `depot`'s host-mapped `8082`).
- Dead-letter subject gap from Phase 3's "Known Gap Found" (missing
  `<service>.deadletter.*` stream subject) was fixed across all three affected
  services (`ordering`, `depot`, `baskets`) as prep work before this phase,
  though not yet tested live.

### Issues Found & Resolved

- **The core problem this phase exists to solve:** a naive "always call
  `CreateShoppingList`, then loop `AddItem`" orchestration is not safe under
  JetStream's at-least-once redelivery (ADL 0013). Traced in detail via chat
  discussion rather than by hitting it in a live test: a partial `AddItem`
  failure leaves `Order.ShoppingListID` unset, so a redelivered retry re-enters
  `orchestrateShoppingList` from scratch. Two independent failure modes were
  identified from that one root cause — duplicate shopping list creation (since
  `depot`'s `CreateShoppingList` had no memory of `order_id`), and
  double-counted item quantities (since `AddItem` is additive, not idempotent,
  and a blind retry would re-add already-added units).
- **Rejected approach:** a `RemoveShoppingList`/`Delete` capability on `depot`
  (new domain method, event, repository method, proto RPC, possible migration
  for cascading delete) was designed in detail as a first candidate — it would
  have cleaned up every partial attempt by deleting it before retrying. Rejected
  as disproportionate to the actual requirement ("adequate, not perfect") once a
  smaller alternative was found; also correctly identified as having its own
  placement flaw (the ID needed for removal only ever exists as a local variable
  inside `orchestrateShoppingList`, never persisted, so a naive removal call at
  the top of `Execute`'s retry branch would have had nothing to remove).
- **Adopted approach, smaller and non-destructive:** idempotent
  `CreateShoppingList` on `depot` (query by `order_id` before constructing)
  closes the duplication problem; a `GetShoppingList`-based delta computation in
  `orchestrateShoppingList` closes the double-counting problem. Both verified
  live for `depot`'s half (two `curl` sequences: a pre-existing `order_id`
  returning its existing list unchanged, and a fresh `order_id` creating once
  then returning the same list on a second call — confirmed both via response
  payloads and `depot`'s own logs showing the publish happening exactly once).

### Known, Explicitly Accepted Gaps

- **No locking around the check-then-act sequence.** Neither `depot`'s
  `FindByOrderID`-then-`Save` nor `ordering`'s `FindByID`-then-orchestrate is
  wrapped in `WithLock`. Two genuinely concurrent redeliveries of the same
  message could both pass the "doesn't exist yet" check before either finishes,
  still producing a duplicate. Narrower window than the "every single retry
  duplicates" bug this phase fixes, but not zero. This is the same category of
  gap Phase 1 Step 4 already flagged for `Order` and never fully closed —
  carried forward again, not newly introduced.
- **`AddItem` partial-failure ordering.** If one product's `AddItem` loop fails
  partway, `orchestrateShoppingList` returns immediately rather than continuing
  to the next product in `order.Items`. Nothing double-counts on retry (the
  delta logic handles that correctly), but products listed after the failed one
  see no progress until the next retry, even though nothing was actually
  blocking them individually.
- **Live verification deferred, not skipped.** Per the user's direction, testing
  an actual `AddItem`-partway-failure scenario and a real concurrent-redelivery
  race was explicitly postponed to Phase 6/7, once `connectapi` exists and the
  full stack can be exercised end-to-end (real basket checkout → real `ordering`
  → real `depot` calls) rather than simulated via manual
  `curl`/direct-database-state tricks. Problem A (duplicate creation) is the
  only piece verified live so far; Problem B (double-counting) is reasoned
  through and code-complete but unverified under a real partial failure.
- **Dead-letter subject fix (`ordering.deadletter.*` and equivalents) is applied
  but untested.** Fixed as prep work before this phase began; not yet exercised
  by an actual dead-lettered message in any service.

### State at End of Phase 4

- `ordering` now has a working `depotclient` and an orchestration flow in
  `CreateOrder.Execute` that survives redelivery without duplicating shopping
  lists or double-counting items, so far as static review and `depot`-side
  testing can confirm.
- `go build ./...` clean across the workspace; the service runs under
  `docker compose up` with `DEPOT_URL` correctly wired.
- No `connectapi`, no `orderingpb`, still no synchronous API surface for
  `ordering` itself — `GetOrder`/`CancelOrder` (Phase 6) remain unbuilt.
- No second subscriber yet (`ShoppingListCompleted` → `MarkReadied`) —
  `Order.Status` never advances past `created` in practice yet, even once
  `ShoppingListID` gets linked.

### Carried Into Phase 5

Phase 5 builds the second `Subscriber` — consuming `depot`'s
`ShoppingListCompleted` event, correlating it back to an `Order` via the
now-functional `shopping_list_id` linkage this phase produces, and calling
`order.MarkReadied()`. This is the first point at which today's
`FindByShoppingListID` deferral (named back in Phase 1 Step 5, held open
specifically for this moment) becomes concrete rather than speculative — a
repository method and likely an index need to actually be built now. It's also
the third concurrent listener in `ordering` (HTTP + baskets subscriber + this
new one), extending ADL 0012's multi-listener pattern one step further than any
prior phase has.

______________________________________________________________________

## Phase 5 Summary — COMPLETE

**Status:** Complete

### What Was Built

`ordering` gained its second concurrent NATS listener and the closing half of
its orchestration loop: reacting to `depot`'s `ShoppingListCompleted` event to
advance an `Order` from `created` to `readied`.

- **`internal/domain/order_repository.go`** — `OrderRepository` gained
  `FindByShoppingListID`, the lookup Phase 1 Step 5 deliberately deferred to
  this exact moment.
- **`internal/postgres/order_repository.go`** — `FindByShoppingListID`
  implemented: a plain (non-locking) `SELECT ... WHERE shopping_list_id = $1`,
  same convention as `FindByID`, followed by the existing `loadItems` helper and
  `ReconstituteOrder`. Two real bugs were caught and fixed before this method
  saw real data: a `Scan` call missing an `&` on one destination (masked
  initially because every early test hit the zero-rows path first, which returns
  before `Scan` ever binds that field), and a stray `sql.ErrNoRows` where every
  other repository method in this codebase checks `pgx.ErrNoRows` — silently
  falling through to the generic `default` error branch instead of
  `domain.ErrOrderNotFound` had it gone uncaught.
- **`internal/application/complete_order.go`** (`CompleteOrder` use case) —
  structurally parallel to `CreateOrder`: locates the order by shopping list ID,
  re-locks it via `FindByIDForUpdate` for a fresh, serialized read, checks
  whether it's already `readied` (the idempotency path, using the same agreed-on
  pattern as `CreateOrder`), calls `MarkReadied()`, saves, and publishes
  `OrderReadied` outside the transaction. An unresolved `shopping_list_id` (no
  matching order at all) is treated as a hard error and allowed to dead-letter
  after exhausting redelivery, per this Phase's explicit decision — not defended
  against or special-cased.
- **`internal/depot/depot.go`** (`depot.Handler`) — the translation boundary
  consuming `depot`'s `ShoppingListCompleted` event, mirroring `baskets`'s own
  `Handler` shape and local envelope type (no import of `depot`'s internal
  packages, per ADL 0004). `ShoppingListCompleted` carries no payload, so the
  handler needs only the envelope's `AggregateID` (the shopping list ID) to call
  `CompleteOrder.Execute`.
- **`internal/events/stream.go`** — gained `DepotStreamName`, the stream
  `ordering`'s new subscriber consumes (owned by `depot`, not `ordering`).
  `OrderingStreamSubjects`'s dead-letter entry was corrected from
  `ordering.deadletter.*` to `ordering.deadletter.>` — a real, previously
  undetected bug: `*` matches exactly one subject token, while a dead-letter
  subject built as `<service>.deadletter.<original-subject>` is several tokens
  deep once the original subject itself has dots in it
  (`depot.shopping_list.ShoppingListCompleted` alone is three). The stream
  simply refused to accept a publish to a subject it didn't actually own,
  surfacing as `nats: no response from stream`. Confirmed and fixed identically
  across `stores`, `depot`, and `baskets`' own equivalent declarations, not just
  `ordering`'s — the same systemic gap Phase 3's "Known Gap Found" section had
  already flagged as affecting all three services, now actually closed rather
  than just named.
- **`ordering.go`** (`Module`) — `Subscriber *foundationnats.Subscriber`
  (singular) generalized to `Subscribers map[string]*foundationnats.Subscriber`,
  keyed `"baskets"`/`"depot"`, to hold both concurrent listeners without
  inventing a second named field per subscriber. `CompleteOrderUC` wired
  alongside `CreateOrderUC`.
- **`cmd/service/main.go`** — `foundationshutdown.Run` now receives three
  concurrent `Service` implementations (`ORDERING-HTTP` +
  `Subscribers["baskets"]` + `Subscribers["depot"]`), the widest application of
  ADL 0012's multi-listener pattern in this system so far.

### Issues Found & Resolved

- **Dead-letter wildcard bug (`*` vs. `>`)** — see above. This is the second
  time this exact class of NATS subject-matching bug has surfaced in this
  project (the first being Phase 3's "Known Gap Found" against `ordering`'s own
  stream), and this time it was actually fixed everywhere it existed, rather
  than named and deferred again.
- **`Scan` missing an `&`, and `sql.ErrNoRows` instead of `pgx.ErrNoRows`** —
  both in the initial `FindByShoppingListID` draft, both caught in code review
  before being exercised against real data, both fixed to match the conventions
  already established by every other repository method in this codebase.
- **Test fixtures using invented store/product IDs cannot seed a fully
  orchestrated order.** `TestCompleteOrder_Execute_Succeeds` and
  `TestCompleteOrder_Execute_IsIdempotentOnRepeatedShoppingListID` both need a
  seed order whose `AddItem` call to `depot` actually succeeds — which requires
  a store and product `depot`'s catalog cache genuinely recognizes, which in
  turn requires those to have been created through the real, live `stores`
  service and projected over NATS, not invented on the spot the way every other
  fixture in this test file does. This is the identical gap Phase 4's summary
  already named ("no application-layer test exercises \[a real catalog
  dependency\]... deliberately left open") — Phase 5 is simply the first place a
  test actually tripped over it directly rather than leaving it theoretical.

### Known, Explicitly Accepted Gaps

- **`CompleteOrder`'s success and idempotency paths are unit-tested only by
  deferral, not by an automated test.** Both were instead verified live,
  end-to-end, against the real containerized stack (`stores` → `baskets` →
  `ordering` → `depot` → back to `ordering`), and both passed cleanly on the
  first real run: `OrderReadied` published exactly once, `orders.status`
  confirmed `readied` in Postgres, no retries or dead-lettering observed. The
  automated coverage gap is deferred specifically to Phase 6, once `ordering`'s
  own `connectapi` layer exists and a `storesclient`-style test seeding helper
  (or an equivalent fixture strategy) can be built as part of a broader, more
  comprehensive test pass — not deferred indefinitely.
- **No guard against `MarkReadied` being called on an already-`canceled`
  order.** `CompleteOrder.Execute` doesn't special-case
  `ErrOrderAlreadyCanceled` the way it special-cases "already readied" — a
  `ShoppingListCompleted` event arriving for a canceled order would hit the
  hard-error/dead-letter path, same as an unresolved shopping list ID. This is
  arguably the correct behavior (a canceled order should not silently become
  readied), but it's untested and untestable today, since no `CancelOrder` use
  case exists yet in this project. Named here explicitly so it's a known
  consequence to revisit once Phase 6 builds `CancelOrder`, not a gap discovered
  by surprise later.
- **The same check-then-act race named in Phase 4** (no locking around a
  correlation lookup followed by a separate mutation) applies here too:
  `FindByShoppingListID` is a plain, non-locking read, and only the subsequent
  `FindByIDForUpdate` call actually takes the row lock. Two genuinely concurrent
  redeliveries of the same `ShoppingListCompleted` message could both resolve
  the same order ID before either takes the lock — narrower than "every retry
  duplicates," same as Phase 4's equivalent gap, and carried forward rather than
  newly introduced.

### State at End of Phase 5

- `ordering` now runs three concurrent listeners (HTTP health endpoint,
  `baskets`-stream subscriber, `depot`-stream subscriber) under one
  `foundationshutdown.Run` call, per ADL 0012.
- The full orchestration loop this project exists to build is now live and
  proven end-to-end for the first time: a real `stores` product, added to a real
  `baskets` basket, checked out, triggers `ordering` to create an `Order` and
  synchronously drive `depot` to build a matching shopping list; completing that
  shopping list in `depot` flows back through `ordering`'s second subscriber to
  advance the same `Order` to `readied`.
- No `connectapi`, no `orderingpb`, still no synchronous API surface for
  `ordering` itself — `GetOrder`/`CancelOrder` remain unbuilt, and there is
  still no way to query or cancel an order except by reading Postgres directly,
  which is exactly what Phase 6 addresses.
- Every dead-letter subject across `stores`, `depot`, `baskets`, and `ordering`
  now uses the correct `>` wildcard — a cross-project fix applied this Phase
  even though only `ordering`'s own gap was originally in scope for this
  project.

### Carried Into Phase 6

Phase 6 builds `ordering`'s own ConnectRPC server per the original Phase Plan:
`GetOrder` (a straightforward ADL 0008-compliant query path, structurally
identical to `depot`/`baskets`' own `GetShoppingList`/`GetBasket`) and
`CancelOrder`, which raises a genuine open design question this project hasn't
answered yet — whether canceling an `Order` in `ordering` needs to reach into
`depot` to also cancel its linked shopping list, or whether the two lifecycles
are allowed to diverge. Once `connectapi` exists, this is also the point to
build the `storesclient`-style test-seeding helper this Phase deferred, and to
write the fuller, more comprehensive test pass this Phase's "Known, Explicitly
Accepted Gaps" section named as pending — including, finally, real automated
coverage for `CompleteOrder`'s success and idempotency paths, and a first real
test of the already-canceled-order interaction with `ShoppingListCompleted` once
`CancelOrder` gives us a way to produce that state at all.
