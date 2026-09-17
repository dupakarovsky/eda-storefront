# 🟢 Ordering Service — Project Plan

## Phase 1

**Step 1 — `Item`, `OrderStatus`, and the `Order` struct itself.** Define the
shape: `OrderStatus` (`created`, `readied`, `canceled` — mirroring
`ShoppingListStatus`'s pattern), an `Item` type (`ProductID`, `StoreID`,
`Quantity`), and `Order` (`ID`, `Status`, `Items []Item`,
`ShoppingListID string`). `ShoppingListID` starts empty and gets filled in
during Phase 4 once `depot.CreateShoppingList` returns one — worth flagging now
since it means `Order`'s zero-value state briefly has an "existing but not yet
linked to a shopping list" phase, distinct from any of its named statuses.

**Step 2 — Domain events and aggregate behavior.** `order_events.go`:
`OrderCreated`, `OrderReadied`, `OrderCanceled`, following the same
`eventBase`/`Event` interface shape `baskets`/`depot` already use. Then the
behavior methods on `Order` itself — `NewOrder(id, items)` (constructor +
`OrderCreated`), `MarkReadied()`, `Cancel()` — each with the guard conditions
their status transitions require (e.g. can't ready an already-readied or
canceled order). This is also where we'll need to decide: does `Order` get a
method to record its `ShoppingListID` once `depot` returns one (a plain
field-set, no event of its own — it's bookkeeping for orchestration, not a
customer-facing state change), or does that belong somewhere else? Worth a
explicit call before Step 4's repository work, since it affects what `Save`
persists.

**Step 3 — `OrderRepository` interface, and the concurrency strategy decision.**
Define the interface shape (`Save`, `FindByID`, `FindByIDForUpdate`, `WithLock`
— matching `ShoppingListRepository`/`BasketRepository`'s existing shape) but
before locking that in, I want to actually talk through the concurrency question
ADL 0002 asks us to decide per-service rather than default: does `Order` need
the same row-level `SELECT ... FOR UPDATE` locking `baskets`/`depot` use, given
`Order`'s writes are driven by consumed events (with real redelivery risk — a
slow handler plus JetStream's `AckWait` could mean two deliveries of the same
event land concurrently) rather than direct concurrent user action? I have a
view on this but want to walk through it with you as part of this step rather
than just asserting it.

**Step 4 — Postgres repository implementation.** `postgres/interface.go` (the
`Executor` abstraction, copied pattern from `baskets`/`depot`),
`postgres/txcontext.go` (ADL 0010's context-scoped executor resolution — ported
as-is, not re-derived), and `postgres/order_repository.go` itself: the migration
(an `orders` table, plus an `order_items` table mirroring `shopping_list_items`'
replace-on-save pattern), and `Save`/`FindByID`/`FindByIDForUpdate`/`WithLock`.

**Step 5** — Postgres repository implementation (migration, `interface.go`,
`txcontext.go`, `order_repository.go`). Phase 1 ends here.
