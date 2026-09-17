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

### Step 1 — `Item`, `OrderStatus`, and the `Order` struct

Two files, matching `depot`'s convention of keeping the status enum in its own
file (`depot-shopping_list_status.go`) separate from the aggregate struct
itself:

```go
// ordering/internal/domain/order_status.go
```

```go
// ordering/internal/domain/order.go
```

**What's happening here:**

`OrderStatus` has three values, not `baskets`' or `depot`'s four — there's no
`pending`/`open` stage. An `Order` is only ever created already-populated (it
comes into existence from `BasketCheckedOut`, which already carries a full item
list per ADL 0016), so there's no "empty, being built up one `AddItem` call at a
time" phase the way a `Basket` or `ShoppingList` has. `created` is the initial
status, not an editable-draft one.

`Item` carries `StoreID` where `Basket`'s `Item` doesn't. `Basket` never needed
it — it only cares about `product_id`/`quantity`, and `store_id` lives one layer
down in the catalog cache purely for `AddItem`'s existence check. `Order`'s
`Item` needs `StoreID` as a first-class field because `ordering` has to hand it
forward to `depot.AddItem` per unit (per our "call it N times" decision) —
`depot` groups items into per-store `Stops`, and `ordering` is the one supplying
that grouping key.

`Items` is a flat slice, not a map keyed by store the way `ShoppingList.Stops`
is. That grouping is `depot`'s concern, not `ordering`'s — `Order` just needs to
remember what was ordered; it never needs to iterate "all items at store X"
itself. Introducing that structure here would be modeling `depot`'s internal
shape inside a different aggregate for no benefit.

`ShoppingListID` starts as an empty string and has no dedicated "unset" sentinel
— it's populated later (Phase 4) once `depot.CreateShoppingList` returns one. I
left it a plain field for now rather than deciding how it gets set (a plain
assignment vs. something event-worthy) — that's next in Step 2.

Nothing runs yet — there's no behavior on `Order`, and Step 3's test file is
what will actually exercise these. For now, a quick `go build ./...` from the
`ordering` module root (once the file's saved in `internal/domain/`) is enough
to confirm both files compile and belong to the same package. Let me know how
that goes, and whether the `StoreID`-on-`Item` reasoning makes sense before we
move to Step 2's events and behavior methods.
