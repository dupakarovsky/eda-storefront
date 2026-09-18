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

### Step 2 — Domain events and aggregate behavior

Three pieces: the `Event` interface + `eventBase` + concrete events (mirroring
`baskets`/`depot`'s exact shape), then the behavior methods added onto
`order.go`.

```go
// ordering/internal/domain/order_events.go
```

```go
// ordering/internal/domain/order.go
```

**Decisions worth flagging:**

**`LinkShoppingList` is a plain method returning only an `error`, not
`(Event, error)`.** This settles the question from Step 1: recording the
`shopping_list_id` is orchestration bookkeeping, not a customer-facing state
change — nothing downstream needs to know "the order got linked to a shopping
list" as its own event. It's guarded (blank ID, already-linked) the same way
every other mutation in this codebase is guarded, but it doesn't participate in
the event-publishing pipeline `NewOrder`/`MarkReadied`/`Cancel` do.

**`Cancel` and `MarkReadied` share the same two guard errors**, mirroring
`Basket.Cancel`'s pattern of naming the specific already-in-that-terminal-status
case rather than a single generic "wrong status" error the way
`Checkout`/`AddItem` do. Both of `Order`'s non-`created` statuses are terminal
(there's no path back to `created`), so both methods only ever need to
distinguish "already readied" from "already canceled" — there's no third,
`ErrOrderNotCreated`-style case, since `created` is the only non-terminal status
and it's the implicit success path for both methods.

**`ErrOrderNotYetReady` is declared but unused right now** — I added it
anticipating Phase 6's `CancelOrder` synchronous RPC might need to express "you
can't ready this via some future manual-override path," but if it turns out
unnecessary I'll remove it before Phase 1 closes. Flagging it now rather than
silently leaving dead code.

**`ReconstituteOrder` takes `shoppingListID` as a parameter**, unlike
`ReconstituteBasket`/`ReconstituteShoppingList/`, which don't have an equivalent
field to restore. This is the first aggregate in the system where reconstitution
needs to rehydrate something beyond status and items.

Ready for the test file whenever you are — let me know if the guard-error naming
or the `LinkShoppingList` shape needs adjusting first.

```go
// ordering/internal/domain/order_test.go
```

A couple of things worth noting about this file relative to `basket_test.go`'s
shape: **No blank-ID or "invalid item" guard tests**, unlike
`TestBasket_AddItem`'s `"blank product id returns error"` case. `NewOrder`
doesn't validate `items` at all right now — it accepts whatever slice it's
given, including an empty one or one with blank `ProductID`s. That's arguably a
gap: should `NewOrder` reject an order with zero items, the way
`Basket.Checkout` and `ShoppingList.Complete` both guard against emptiness?
Given `Order` is always constructed _from_ an already-checked-out (therefore
non-empty) basket, an empty-items `Order` should never occur in practice — but
"should never occur given how it's called" is a different guarantee than "the
constructor enforces it," and every other aggregate in this system enforces its
own invariants rather than trusting callers. Worth deciding before Step 3
closes: add an `ErrOrderHasNoItems`-style guard now, or leave it as a
trusted-caller assumption?

**`TestOrder_MarkReadied`'s happy-path test links a shopping list first**, even
though `MarkReadied` doesn't currently check `ShoppingListID` at all — I did
that to model the realistic call sequence, not because the method enforces it.
That raises the same kind of question as above: should `MarkReadied` guard
against being called before a shopping list is linked (an
`ErrShoppingListNotLinked`-style error), given that in the real flow it only
ever fires in response to a `ShoppingListCompleted` event that presupposes
linkage already happened? Right now nothing stops `order.MarkReadied()` from
succeeding on a freshly-constructed, never-linked `Order`.

Let me know your call on both before we move to Step 4. Good calls — both guards
are cheap and close real gaps. Here are the changes:

```go
// ordering/internal/domain/order.go
```

```go
// ordering/internal/domain/order.go — NewOrder guard
```

```go
// ordering/internal/domain/order.go — MarkReadied guard order
```

Note the guard ordering in `MarkReadied`: status checks run _before_ the linkage
check, not after. That's deliberate — if an order is already canceled, the
caller should hear "already canceled," not "not linked," even though a canceled
order was in fact never linked either. The status is the more fundamental fact
about the order; linkage is a precondition for the one transition that's
actually still available. This also means the existing
`"already readied returns error"` and `"already canceled returns error"`
subtests in `TestOrder_MarkReadied` don't need to change — they still hit their
intended error even without calling `LinkShoppingList` first, since status is
checked first.

Now the test additions:

```go
// ordering/internal/domain/order_test.go — new subtest in TestNewOrder
```

```go
// ordering/internal/domain/order_test.go — new subtest in TestOrder_MarkReadied
```

Both drop into their respective existing test functions (`TestNewOrder` gets the
empty-items case alongside its one existing subtest; `TestOrder_MarkReadied`
gets the not-linked case alongside its three existing ones). No other existing
test needs to change — `newTestOrder` already constructs with one real item, so
it's unaffected by the new `NewOrder` guard, and I confirmed above the
`MarkReadied` guard reordering doesn't touch the
already-readied/already-canceled subtests.

Run `go test ./internal/domain/...` and let me know what comes back before we
move to Step 4.

### Step 4 — Postgres repository implementation
