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
event's existing aggregate ID (`shopping_list_id`), which `ordering` will have
stored on its own `Order` row.

**`depot`'s `AddItem` stays untouched.** `ordering` calls `AddItem` once per
unit rather than passing a `quantity`, avoiding a signature change to an
already-live method.

## Phase Plan

**Phase 1 — Order Aggregate & Persistence.** Define `domain.Order` (status enum:
`created` → `readied`/`canceled`, items with `product_id`/`store_id`/
`quantity`, a `shopping_list_id` field to be filled in later), its domain
events, and the Postgres repository — current-state rows per ADL 0002, with an
explicitly-chosen concurrency strategy. Testable standalone, no NATS/HTTP yet.

**Phase 2 — Contract Changes to `baskets` and `depot`.** Enrich
`BasketCheckedOut`'s payload in `baskets` (touching the `Checkout` use case,
since it needs each item's `store_id` from the catalog cache). Add `order_id` to
`depot`'s `CreateShoppingList` (domain constructor, application use case, proto
message, handler).

**Phase 3 — Consuming `BasketCheckedOut` → Creating the Order.** Wire up
`ordering`'s first `Subscriber` (the `baskets` stream); the use case that
projects the event into a new `Order` and publishes `OrderCreated`.

**Phase 4 — Orchestrating `depot`: `CreateShoppingList` + `AddItem`.** Build the
`depotclient` wrapper; call it from the order-creation flow to create and
populate the shopping list, storing the returned `shopping_list_id` on the
`Order`. Decide failure-handling philosophy for a synchronous downstream
command.

**Phase 5 — Consuming `ShoppingListCompleted` → Order Readied.** Second
`Subscriber` (the `depot` stream); look up the `Order` by `shopping_list_id`,
transition it, publish `OrderReadied`.

**Phase 6 — Synchronous API Surface.** `ordering`'s own ConnectRPC server:
`GetOrder` (ADL 0008-compliant query path) and `CancelOrder` (deciding whether
cancellation needs to reach into `depot` too).

**Phase 7 — Containerization & Live Verification.** Dockerfile,
`docker-compose.yaml`/`Makefile` wiring, migrations, and the same
live-verification discipline (curl script, concurrency test,
redelivery/idempotency tests) every prior service got.
