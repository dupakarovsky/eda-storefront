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

### Step 5 — Postgres repository implementation

Four pieces: the migration, `interface.go` (the `Executor` abstraction),
`txcontext.go` (ADL 0010's context-scoped resolution, ported as-is), and
`order_repository.go` itself.

**Migration:**

```sql
-- ordering/migrations/000001_create_orders_table.up.sql
```

```sql
-- ordering/migrations/000001_create_orders_table.down.sql
```

`shopping_list_id` is `NOT NULL DEFAULT ''` rather than a nullable column —
matches the domain's own "empty string means unlinked" convention (no separate
`NULL`-vs-`""` state to reconcile between Go and SQL). `order_items`' primary
key is `(order_id, product_id)`, same shape as `basket_items`, since an `Order`,
like a `Basket`, never has two rows for the same product — quantity absorbs
repeats.

**`interface.go` and `txcontext.go`** — copied verbatim from `depot`'s versions,
since ADL 0010 already settled this pattern and there's nothing service-specific
in either file:

```go
// ordering/internal/postgres/interface.go
```

```go
// ordering/internal/postgres/txcontext.go
```

**`order_repository.go`:**

```go
// ordering/internal/postgres/order_repository.go
```

Two things worth flagging:

**`Save` still replaces all items on every call**, same delete-then-reinsert
pattern `baskets`/`depot` use — even though, unlike `Basket`/`ShoppingList`,
`Order.Items` never actually changes after `NewOrder` (there's no
`AddItem`/`RemoveItem` equivalent; items are fixed at creation). This is
slightly wasteful on every `Save` call after the first (re-deleting and
re-inserting identical rows), but I kept it rather than special-case an
insert-only path, since `Save` is called generically from every use case
(`CreateOrder`, `LinkShoppingList`'s persistence, `MarkReadied`, `Cancel`) and
none of them know or care whether items changed. If this turns out to matter for
`Order`'s specific write volume later, it's a cheap optimization to add — not
worth pre-optimizing for now.

**No explicit index on `shopping_list_id`** even though Phase 5's
`CompleteOrder` use case will need to look up an `Order` by it (that's how
`ShoppingListCompleted` gets correlated back to the right `Order`) — `FindByID`
above only looks up by primary key. That's a real gap for Phase 5, not Phase 1:
I'd rather add `FindByShoppingListID` (and its supporting index) as part of Step
5 of Phase 5 specifically, when we're building `CompleteOrder` and the need is
concrete, rather than speculatively now.

That closes Phase 1. Run `go build ./...` from the `ordering` module root to
confirm everything compiles together, and let me know when you're ready to move
to Phase 2 — the `baskets`/`depot` contract changes.

---

## Phase 2

Here's how I'd like to break Phase 2 into steps:

**Part A — `baskets`: enrich `BasketCheckedOut`**

- **Step 1:** Design discussion — decide the enriched event's exact shape, and
  where `store_id` resolution happens (domain vs. application layer). I have a
  specific recommendation here that follows the same pattern `depot`'s `AddItem`
  already uses, but it's worth walking through before touching code.
- **Step 2:** Update `baskets`' domain layer — `Basket.Checkout()`'s signature
  and the `BasketCheckedOut` event.
- **Step 3:** Update `baskets`' application layer — `Checkout.Execute` resolves
  each item's store via `catalogRepo` before calling into the domain.
- **Step 4:** Update `baskets`' events package (`payload.go`) for the new wire
  shape, and update `baskets-CONTRACT.md`.

**Part B — `depot`: add `order_id` to `CreateShoppingList`**

- **Step 5:** Update `depot`'s domain — `NewShoppingList` constructor and
  `ShoppingList` struct gain an `OrderID`.
- **Step 6:** Update `depot`'s application layer, proto (`api.proto` +
  regenerated files), and connect handler for the new required field.
- **Step 7:** Update `depot-CONTRACT.md`.

Each step gets its own check-in as usual. Does this breakdown look right to you,
or would you split/merge anything before we start on Step 1's design discussion?

### Step 1 — Design: enriching `BasketCheckedOut`

This resolves the "known limitation" already flagged in `baskets-CONTRACT.md`:
`BasketCheckedOut` currently carries `{}`, no item list at all.

#### Where does `store_id` resolution happen?

`depot`'s `AddItem` already set the precedent here: the **application** layer
resolves catalog data first (`catalogRepo.FindProductByID`), then hands the
resolved data down into the domain method, which builds the fully-formed event.
Domain stays ignorant of the catalog package entirely — same boundary as
everywhere else in this system.

So `Checkout` (application) needs a new dependency it doesn't have today:
`catalogRepo catalog.Repository`, injected the same way `AddItem`'s `Checkout`
struct already does it.

#### How does the enriched data get into the domain?

Two ways to shape this, worth deciding explicitly rather than defaulting:

**Option A — application builds the full item list and hands it to `Checkout`.**

```go
func (b *Basket) Checkout(items []CheckedOutItem) (Event, error)
```

Application constructs `[]CheckedOutItem{ProductID, StoreID, Quantity}` itself,
one per `basket.Items` entry. Domain still validates state (`Status == Open`,
non-empty) using its own `b.Items`, but trusts the passed slice for the event
payload. Risk: the passed slice and `b.Items` could theoretically drift out of
sync (a bug elsewhere), and domain has no way to catch that.

**Option B — application passes only a `productID → storeID` lookup map; domain
still owns iteration.**

```go
func (b *Basket) Checkout(storeIDs map[string]string) (Event, error)
```

Domain still iterates its own `b.Items` (single source of truth for
quantity/product list) and enriches each with `storeIDs[item.ProductID]`. No
duplicate list to drift out of sync — domain owns exactly what it always owned,
plus one extra piece of external data per product.

I'd recommend **Option B** — it keeps `Basket.Items` as the one and only source
of truth for "what's in this checkout," and the map is clearly just enrichment
data, not a second copy of the basket's contents. It also raises a real question
we should decide now: **what happens if a product ID in `b.Items` has no entry
in the map?** Two choices:

- Trust it — every `b.Items` entry was validated against the catalog at
  `AddItem` time, so in practice this can't happen unless the product was later
  removed from the catalog (`stores`' `RemoveProduct` → `ProductRemoved`
  projected into `baskets`' local cache, deleting the row). If that happened
  between add and checkout, `catalogRepo.FindProductByID` in the application
  layer would already fail loudly there, before `Checkout()` is ever called
  domain-side — so by the time the map reaches domain, it's guaranteed complete.
- Defend anyway — domain returns a new sentinel error (e.g.
  `ErrProductNotInCatalog`) if a lookup misses, even though the application
  layer should never let that happen.

Given the same "trust the caller has already validated" convention `AddItem`'s
domain method already follows (it takes `store.ID, store.Name, store.Location`
as trusted, pre-validated params), I'd lean toward **trusting it** — consistent
with existing precedent, and the real failure mode (stale product) is already
caught earlier, application-side, with a proper `CodeNotFound` response to the
caller.

#### New domain type and event shape

```go
type CheckedOutItem struct {
    ProductID string
    StoreID   string
    Quantity  int
}
```

```go
type BasketCheckedOut struct {
    eventBase
    Items []CheckedOutItem
}
```

And on the wire (`events/payload.go`):

```go
type CheckedOutItemPayload struct {
    ProductID string `json:"product_id"`
    StoreID   string `json:"store_id"`
    Quantity  int    `json:"quantity"`
}

type BasketCheckedOutPayload struct {
    Items []CheckedOutItemPayload `json:"items"`
}
```

matching the `product_id`/`store_id`/`quantity` shape ADL 0016 specified.

One more application-layer detail worth deciding now: the store lookup happens
**inside** the existing `WithLock` transaction (same as `AddItem`'s product
validation), so a mid-checkout catalog change can't produce an inconsistent
result.

---

Does Option B (map-based enrichment, domain trusts the map's completeness) match
how you'd want this shaped? If so, I'll ask for `basket_test.go` next so Step
2's domain changes land consistent with existing test conventions.

Good — this confirms `ApplicationTest` tests run against a real
Postgres/`Module`, not a mock/fake `catalogRepo`, which simplifies things: no
test double needs updating for the new interface method, since `mod.CatalogRepo`
is the real `postgres.CatalogRepository` under the hood.

One thing this test file surfaces that we still need to close out:
**`baskets-module.go`'s wiring**. `CheckoutUC` is currently constructed as:

```go
checkoutUC := application.NewCheckout(basketRepo, publisher, logger)
```

With `NewCheckout`'s signature now requiring `catalogRepo`, this call site won't
compile until it's updated — this is exactly the "don't break what's already
working" case, so it needs to land in this same step, not deferred.

```go
// baskets/internal/baskets.go — update the CheckoutUC construction line
checkoutUC := application.NewCheckout(basketRepo, catalogRepo, publisher, logger)
```

(`catalogRepo` is already in scope in `New()` — it's the same variable already
passed into `NewAddItem` two lines above it.)

With that wiring fix, the `checkout.go` I gave you last turn should compile
cleanly against your `checkout_test.go` as-is:

- `TestCheckout_Execute_Succeeds` seeds two products under two _different_
  stores, adds both to the basket, checks out — exercises the multi-store case,
  which is a good test of the map actually holding more than one entry
  correctly.
- `TestCheckout_Execute_RejectsEmptyBasket` confirms the empty-basket guard
  still fires before the (now-added) catalog lookup step even matters.

Go ahead and run both `go test -v .` in `baskets/internal/domain` (to reconfirm
nothing regressed) and `go test -v .` in `baskets/internal/application` with the
module.go fix applied, and let me know the results.

Step 3 is done and confirmed — both application-layer tests pass, and the NATS
log confirms `BasketCheckedOut` actually published on checkout (seq 49), so the
whole path (domain → repo → publish) is exercised end-to-end against real
Postgres/NATS, not just asserted on return values.

**Step 3 check-in summary:**

- `catalog.Repository` interface gained `FindStoreIDsByProductIDs`
- `postgres.CatalogRepository` implements it via a single `= ANY($1)` batched
  query
- `Checkout.Execute` resolves the map inside `WithLock`, passes it to
  `basket.Checkout`
- `baskets.go`'s `NewCheckout` call site updated with the new `catalogRepo` arg
- Confirmed: no test double needed updating, since `ApplicationTest` runs
  against the real `Module`/Postgres

On additional coverage — one gap is real but low-value to close right now:
there's no application-layer test exercising `ErrProductNotInCatalog`'s
propagation through `Execute`. The only way to trigger it for real is a product
added to the basket, then removed from the catalog before checkout (mirroring
`AddItem`'s `TestAddItem_Execute_RejectsUnknownProduct` pattern, but via
delete-after-add rather than never-seeded). Given this error is explicitly the
"shouldn't happen, but be defensive" case we discussed in Step 1 — already
thoroughly covered at the domain level — I'd call this optional rather than
blocking. Your call: add it now, or move on and revisit if it ever becomes
relevant (e.g. once `stores`' `RemoveProduct` path is exercised more directly in
this project).

Assuming you'd rather move on: **Step 4** closes out Part A — updating
`baskets`' events package (`payload.go`) so the wire format actually carries the
new `Items` field, plus `baskets-CONTRACT.md`.

Here's the payload change:

```go
// baskets/internal/events/payload.go
```

```go
// baskets/internal/events/payload.go
```

Note the `switch e := event.(type)` at the top of `toPayload` already binds `e`,
so this slots into the existing switch structure without other changes.

Want me to also draft the `baskets-CONTRACT.md` diff (updating
`BasketCheckedOut`'s documented payload and removing the now-resolved "known
limitation" note) as part of this step, or would you rather do that yourself
once the code's confirmed working?

## Step 5 — `depot` domain: `NewShoppingList` gains `OrderID`

This is more mechanical than Part A's design fork — no real branching decision
here, just applying the existing pattern consistently. A few small things worth
calling out as I draft it:

**Validation:** Following the same precedent as
`ErrStoreIDIsBlank`/`ErrProductIDIsBlank` in this same file (and `ordering`'s
own `ErrShoppingListIDIsBlank` guard on `LinkShoppingList`), the constructor
should reject a blank `orderID` rather than silently accepting one — a shopping
list is meaningless without a real order behind it under Design A.

**Where `OrderID` lives:** Three places need it — the `ShoppingList` struct
itself (so `ReconstituteShoppingList` can carry it back from Postgres), the
`NewShoppingList` constructor signature, and the `ShoppingListCreated` event (so
it's available for envelope/payload construction, and matches the convention
`OrderCreated` already established in `ordering` of carrying its own payload
data rather than relying only on `AggregateID()`).

```go
// depot/internal/domain/shopping_list.go — add to the existing var block
ErrOrderIDIsBlank = errors.New("the order id cannot be blank")
```

```go
// depot/internal/domain/shopping_list.go
```

```go
// depot/internal/domain/shopping_list.go
```

```go
// depot/internal/domain/shopping_list.go
```

```go
// depot/internal/domain/shopping_list_events.go
```

Two call sites elsewhere in `depot` will break until Step 6 catches up
(`create_shopping_list.go`'s `domain.NewShoppingList(id)` call, and the Postgres
repository's `find`/`ReconstituteShoppingList` call, and its `Save`'s
INSERT/UPDATE which doesn't yet write an `order_id` column) — expected, since
domain and its consumers move together across these two steps, same as Part A.

Before you run anything: could you share `shopping_list_test.go` (domain layer)?
I want to check its existing `NewShoppingList`/`ReconstituteShoppingList` call
sites against this new signature before you spend time updating it blind.

## Step 6 — `depot`: wiring `order_id`

This touches more files than any step so far, so let me lay out the full set
before code, flag two decisions, then give you everything to apply together.

**Decision 1 — should the `ShoppingList` proto message echo back `order_id`?**
The request will carry it, but nothing currently returns it. I'd add `order_id`
to the `ShoppingList` message itself (not just the request) — `GetShoppingList`
and every other response already returns `id`/`status`/`stops` for confirmation,
and a future consumer (or you, debugging) benefits from being able to see which
order a list belongs to without cross-referencing. This also keeps the proto
message symmetric with the domain struct, same as `id`/`status` already are.

**Decision 2 — `mapError` needs updating.** `depot-errors.go`'s switch doesn't
have a case for the new `domain.ErrOrderIDIsBlank` sentinel yet — without it,
that error would fall through to `CodeInternal`, which is wrong; it's a caller
input error, same class as `ErrStoreIDIsBlank`.

Here's the full set of changes:

```sql
-- depot/migrations/000002_add_order_id_to_shopping_lists.up.sql
```

```sql
-- depot/migrations/000002_add_order_id_to_shopping_lists.down.sql
```

```proto

```

```proto
}
```

You'll need to regenerate `depotpb`/`depotpbconnect` (via `buf generate`, per
ADL 0003) after this proto change — I can't hand-edit the generated
`.pb.go`/`.connect.go` files reliably, so let me know once that's done before we
run anything depending on the new `OrderId` field.

```go
// depot/internal/domain/shopping_list.go
```

```go
// depot/internal/connectapi/errors.go
```

(fold into the existing `CodeInvalidArgument` case's `errors.Is(...)` list,
alongside `ErrStoreIDIsBlank`, `ErrProductIDIsBlank`, etc.)

```go
// depot/internal/application/create_shopping_list.go
```

```go
// depot/internal/connectapi/proto.go
}
```

```go
// depot/internal/connectapi/shopping_list.go
```

```go
// depot/internal/events/payload.go
```

```go
// depot/internal/events/payload.go
```

Now the Postgres repository — `Save`, `find`, and the `ReconstituteShoppingList`
call site all need the new column:

```go
// depot/internal/postgres/shopping_list_repository.go
```

```go
// depot/internal/postgres/shopping_list_repository.go
```

Note: I did **not** change `ON CONFLICT`'s update clause to touch `order_id` — a
shopping list's `order_id` is set once at creation and should never change on a
later `Save` (e.g. from `AddItem`/`Initiate`/`Complete`), same immutability the
`id` column itself already has by virtue of being the conflict key.

Lastly, `create_shopping_list_test.go` needs updating for the new `Execute`
signature:

```go
// depot/internal/application/create_shopping_list_test.go
```

That's the full set for Step 6. Given the size, let me know if you want to apply
and test this in smaller pieces (e.g. migration + repository first, then
application + proto + connectapi), or all at once — your call. Either way,
you'll need to run `buf generate` for the proto changes before anything
referencing `OrderId`/`order_id = 2` will compile.

This actually confirms everything is wired correctly — the failures aren't bugs,
they're the `CreateShoppingList` call cascading. Walking through it:

- `CreateShoppingList` was called with `-d '{}'`, still the pre-ADL-0016 request
  body — no `order_id` field at all.
- The connect handler received a zero-value `""` for `OrderId`, passed it
  straight to `Execute`, which rejected it via `ErrOrderIDIsBlank` — correctly
  mapped to `invalid_argument` / HTTP 400 by `mapError`, exactly as designed.
- Since that call failed, `LIST_ID` got set to `null` (jq's `-r` on a missing
  `.shoppingList.id` field), so every subsequent step operated against a
  shopping list ID of `"null"` — hence the uniform cascade of
  `shopping list not found` from that point on.
- The three tests unrelated to `LIST_ID` (unknown store, unknown product,
  unknown shopping list ID at the very end) all passed correctly, since they
  don't depend on the earlier `CreateShoppingList` call succeeding.

So this run is genuinely good news: it proves the blank-`order_id` guard fires
correctly all the way through the HTTP/Connect layer, not just in unit tests —
the script itself is just stale against the new required field.

The fix is a two-line change to the script — add a real `order_id` to the
`CreateShoppingList` call, matching the ADL's requirement:

```bash
echo "== [depot] CreateShoppingList =="
```

Worth also asserting `orderId` actually round-trips in the response, since
that's new surface this step added — you could add a quick check right after,
e.g.:

```bash
echo "$CREATE_LIST_RESP" \
| jq -e '.shoppingList.orderId == "order-verify-001"' > /dev/null \
&& echo "orderId round-trip OK" || echo "orderId MISMATCH"
```

Go ahead and re-run with that fix — I'd expect the rest of the script (already
proven correct in prior phases, unaffected by this change) to pass exactly as it
did before this project touched `depot`.

Clean end-to-end run — everything's working correctly, including `orderId`
round-tripping through every single response (`CreateShoppingList` through
`CompleteShoppingList`), confirming it's genuinely persisted and reloaded on
every `find`, not just returned once at creation time.

One line worth a quick note rather than a concern: the last check —
_"CompleteShoppingList on an empty (never-initiated) list"_ — returned
`not_found` / HTTP 404 instead of the `invalid_argument` / HTTP 400 the script's
own comment expected. Looking at it: `NEW_LIST_ID` was set from `NEW_LIST_RESP`,
but that `CreateShoppingList` call in the script (further up) still uses
`-d '{}'` — no `order_id` — so it also hit `ErrOrderIDIsBlank` and never
actually created a list, same cascade as the first run. That's a second stale
spot in the script, not a code bug; harmless here since the resulting
`not_found` still correctly exercises the "unknown ID" class of check, just not
the specific "empty completed list" scenario the comment describes. Not worth
re-running for — the guard it was meant to test (`ErrShoppingListIsEmpty` after
`Initiate`) is already proven by the domain and application test suites from
Steps 5–6. Flagging only so it doesn't cause confusion if you revisit this
script later.

**Phase 2 is complete.** Full summary:

- **`baskets`**: `BasketCheckedOut` now carries itemized
  `product_id`/`store_id`/`quantity` data, resolved from the catalog cache
  inside the checkout transaction. Domain, application, wire format, and
  `baskets-CONTRACT.md` all updated and verified — unit tests, application tests
  (with real NATS publish), and this run's earlier `Checkout` path all green.
- **`depot`**: `CreateShoppingList` now requires `order_id`, threaded through
  domain, application, proto/connect, and Postgres (new immutable column).
  `depot-CONTRACT.md` updated. Verified via 16 passing application tests and
  this full curl run against live containers.
- Both `CONTRACT.md` files reflect the ADL 0016 breaking changes accurately, so
  `ordering`'s Phase 3 work can build against them as the current source of
  truth rather than the now-superseded "provisional" language.

That closes the two changes `journey_progress.md` flagged as needed before
`ordering` could proceed. Should I draft the `journey_progress.md` update for
Phase 2's close, or are you ready to move straight into Phase 3 (wiring
`ordering`'s HTTP/NATS/module layer, plus the `depotclient` and orchestration
logic itself)?

## **Phase 3 — Consuming `BasketCheckedOut` → Creating the Order**

- **Step 1 — `internal/events` package.** `envelope.go`, `payload.go`,
  `subjects.go`, `stream.go` — same structure as `depot`'s, giving `ordering`
  the ability to envelope and address its own domain events (`OrderCreated` for
  now; `OrderReadied`/`OrderCanceled` will get payload mappings added in later
  phases as those use cases arrive).
- **Step 2 — `publish_event.go` + `publish_retry.go`.** The retry-with-backoff
  publish helper (ADL 0007 policy, ADL 0015 idempotent msg-ID), same shape as
  `depot`'s.
- **Step 3 — `CreateOrder` use case.** Takes a `BasketCheckedOut` payload
  (already itemized per Phase 2's `baskets` change), constructs and saves an
  `Order` via `domain.NewOrder`, publishes `OrderCreated`. No `depot` call yet —
  `ShoppingListID` stays unset on the `Order` at this point; that gets filled in
  when Phase 4 adds the `depotclient` orchestration into this same use case.
- **Step 4 — `BasketCheckedOut` subscriber wiring.** Consumes off the `baskets`
  stream, unmarshals the envelope, invokes `CreateOrder.Execute`.
- **Step 5 — `module.go`.** Wires Postgres, NATS, the publisher, the one
  subscriber, and `CreateOrder` together — mirrors `depot.go`'s shape.
- **Step 6 — `main.go`.** Two concurrent listeners under `foundation/shutdown`
  (HTTP health endpoint + the one subscriber) — same two-listener shape `depot`
  already proved in ADL 0012, not yet the three-listener case (that arrives in
  Phase 5 once the second subscriber exists).

This is a tighter, more self-contained phase than what I first sketched — by the
end of it, `ordering` is a genuinely running service that reacts to a real
`basket` checkout and produces a real, persisted, published `Order`, even though
that `Order` doesn't have a shopping list linked to it yet.

### Step 1 -- Events Package

Let's build the `events` package. This mirrors `depot`'s own `internal/events`
package almost exactly — same four files, same responsibilities — since this is
the third time this system has built this exact pattern (`stores`, `depot`, now
`ordering`), and ADL 0007/0015 already settled how it should work.

```go
// ordering/internal/events/envelope.go
```

```go
// ordering/internal/events/payload.go
```

```go
// ordering/internal/events/subjects.go
```

```go
// ordering/internal/events/stream.go
```

What's happening here, and a couple of things worth flagging:

I mapped payloads for all three of `Order`'s domain events (`OrderCreated`,
`OrderReadied`, `OrderCanceled`), not just `OrderCreated`. That might look ahead
of where we are — this phase only publishes `OrderCreated` — but the domain
layer already fully defines all three events (Phase 1), and `toPayload`'s job is
purely mechanical: "here's how this domain event's fields become wire JSON."
There's no orchestration logic hiding in there, so mapping all three now costs
nothing and avoids a `switch` that's deliberately incomplete for events we
already know the shape of. Contrast this with `depotclient` or the subscriber
wiring, where _actually calling_ `CreateOrder`/`CompleteOrder` for events not
yet built would be a real placeholder gap — that's not the case here.

Second, notice `stream.go` only defines `OrderingStreamName` — the stream
`ordering` owns and publishes to. `depot`'s equivalent file also defines
`StoresStreamName`, because `depot` needs to reference an external stream to
subscribe to it. I've deliberately left `BasketsStreamName` out of this file for
now; it'll get added in Step 4 when we actually wire the subscriber that needs
it, rather than introduced speculatively here.

Third — `Envelope`, unlike the domain layer, has zero changes from `depot`'s
version beyond the import path. That's expected and fine: enveloping/marshaling
is infrastructure, not domain-specific behavior, so it should look identical
across every service using this pattern.

There's nothing to run yet — this package has no entry point of its own, it only
gets exercised once `CreateOrder` (Step 3) calls `NewEnvelope`. If you want to
sanity-check it compiles on its own before we move forward,
`go build ./ordering/...` from the workspace root should succeed cleanly
(nothing calls this package yet, so it's dead code for the moment, but it should
compile). Let me know how that goes, or if you'd rather just eyeball it and move
on — either is fine at this stage since there's no real behavior to observe yet.

Any questions on this before we move to Step 2 — the
`publish_event.go`/`publish_retry.go` pair?

### Step 2 — the publish helpers

These are copied verbatim in shape from `depot`'s (and `stores`' before that) —
same retry budget, same per-attempt timeout, same reused-`msgID`-across-attempts
idempotency discipline from ADL 0015. Nothing new to design here; this is
exactly the kind of infrastructure code that should look identical every time
it's built, per ADL 0007's own precedent.

```go
// ordering/internal/application/publish_retry.go
```

```go
// ordering/internal/application/publish_event.go
```

Both files land in `internal/application` — same package `CreateOrder` (Step 3)
will live in, since `publishDomainEvent` is a package-private helper every use
case in that package calls, exactly the pattern `stores` and `depot` already
established. One thing worth naming explicitly, since it's easy to let slide by:
`msgID := uuid.NewString()` happens once, outside `publishWithRetry`'s loop, and
the same value rides along every one of the five attempts. That's the whole
mechanism ADL 0015 exists to explain — a fresh ID per attempt would make
JetStream see five unrelated messages instead of one message retried five times,
defeating the deduplication the stream's `Duplicate Window` provides. It's a
one-line difference from "obviously wrong," so it's worth double-checking
against your own copy once you've typed it out.

This package still won't compile _cleanly_ as a whole yet —
`internal/application` doesn't exist as a directory with any other files, and Go
is fine with that, but there's nothing calling `publishDomainEvent` yet, so like
Step 1, this is inert until Step 3 gives it a caller. `go build ./ordering/...`
should still succeed.

Any questions on the retry/publish mechanics before we move to Step 3 — the
`CreateOrder` use case itself?

### Step 3 -- CreateOrder

`CreateOrder` follows the same shape as `stores`' `CreateStore` and `depot`'s
`CreateShoppingList`: construct the aggregate via the domain constructor, `Save`
it, publish the resulting event. No `WithLock` here — same reasoning those two
use cases already established: a brand-new aggregate has no existing row to
protect with `SELECT ... FOR UPDATE`, so the transactional locking pattern only
applies to use cases that read-modify-write an aggregate that already exists.

```go
// ordering/internal/application/create_order.go
```

A few things worth flagging before we move on.

`Execute` takes `id string` and `items []domain.Item` as already-clean
parameters, not a raw NATS message or a `BasketCheckedOut` payload struct. This
mirrors how `depot`'s `AddItem` use case takes plain strings, not a wire
envelope — the `application` layer's contract is "give me domain-shaped inputs,"
and unmarshaling/translating whatever arrived from outside into that shape is
the caller's job, not this use case's. Here, that caller is the subscriber we'll
build in Step 4: it'll unmarshal the `BasketCheckedOut` envelope, convert the
payload's items into `[]domain.Item`, and hand `Execute` the clean result.

That also answers where the `id` comes from. ADL 0009 established that ID
generation belongs to "the layer that owns talking to the outside world" — for
every RPC-triggered `Create` so far, that's been `connectapi`, calling
`uuid.NewString()` right before invoking `Execute`. Here there's no RPC; the
outside-world boundary is the NATS subscriber instead. So Step 4's subscriber,
not `CreateOrder` itself, will be the one generating the order's ID — keeping
`CreateOrder` exactly as agnostic about _where_ its ID came from as
`CreateStore.Execute` already is.

Last thing, and it's deliberate: `order.ShoppingListID` stays at its zero value
(`""`) coming out of this use case — there's no `depotclient` call here yet.
That's not an oversight, it's this phase's actual scope: Phase 3 gets a real,
persisted `Order` with `OrderCreated` published; Phase 4 is where this same use
case grows a call to `depot.CreateShoppingList`/`AddItem` and starts populating
`ShoppingListID`. Worth sitting with for a second, since it means the `Order`
this phase produces is genuinely incomplete by design, not by accident — the
next phase's job is named specifically because of this gap.

Any questions before Step 4 — wiring the actual `BasketCheckedOut` subscriber
that will call this?

### Step 4 — the `BasketCheckedOut` subscriber handler

First, `stream.go` needs the missing piece I deliberately left out in Step 1:

```go
// ordering/internal/events/stream.go
```

Now the handler itself:

```go
// ordering/internal/application/basket_checked_out.go
```

A few things worth walking through.

Notice `basketCheckedOutEnvelope`/`basketCheckedOutPayload` are _local_ types,
duplicating the shape `baskets`' own
`events.Envelope`/`events.BasketCheckedOutPayload` already define. That's not an
oversight — it's the same discipline `depot`'s `catalog/project.go` already
established and `baskets-CONTRACT.md` names explicitly in its non-goals: ADL
0004 prohibits importing another service's internal packages, so a consumer
can't share types with the producer even when it would be more convenient. The
event-name string `"BasketCheckedOut"` and the JSON field names below are
matched by convention against `baskets-CONTRACT.md`, with no compiler-enforced
link — a silent rename on `baskets`' side would not fail this build, it would
just silently stop matching. Same accepted risk, same reasoning, third time this
pattern's been built.

Second — `id := uuid.NewString()` happening right here, inside the handler, is
this phase's answer to "where does ID generation happen" from Step 3. The
handler is `ordering`'s boundary with the outside world for this flow, same role
`connectapi.Server` plays for RPC-triggered creates under ADL 0009 — so it's the
one that mints the new `Order`'s ID before calling into `application`.

Now, something I want to flag rather than quietly build past: this handler is
**not idempotent under redelivery**, unlike `depot`'s catalog projector.
`depot`'s projector upserts rows keyed by the _source_ event's own
`aggregate_id` — redelivering the same `ProductAdded` twice just writes the same
row twice, a no-op the second time. Here, every invocation of this handler —
including a JetStream redelivery of the exact same `BasketCheckedOut` message,
which ADL 0013's at-least-once delivery guarantee makes a real possibility, not
a hypothetical — calls `uuid.NewString()` fresh and creates a _second_, distinct
`Order` for the same basket checkout. That's a genuine gap, structurally similar
in spirit to the ones ADL 0007 and `depot-CONTRACT.md`'s non-goals name
explicitly rather than silently accept.

I'd suggest we note this as a known, deliberate gap for now — consistent with
how this series has generally deferred a newly-surfaced correctness question to
its own decision point rather than solving it under pressure mid-step — and
decide explicitly whether/how to close it (e.g., deriving a deterministic order
ID from the basket's own aggregate ID, so a redelivery naturally collides on the
same row) once we're further into the phase and can see the full shape of the
problem. Want to handle that now, or flag it and keep moving to Step 5 —
`module.go`?

### Step 5 — `module.go`

This mirrors `depot.go`'s shape closely, but simpler in one direction (no
catalog cache, no `connectapi.Server` yet — that's Phase 6 per your Phase Plan)
and mildly more involved in another (this is the first service that both owns a
stream to publish on _and_ subscribes to a stream it doesn't own, in the same
module).

```go
// ordering/ordering.go
```

A couple of things worth walking through.

Notice `EnsureStream` is called for `events.OrderingStreamName` — the stream
`ordering` owns — but there's no equivalent call for `events.BasketsStreamName`.
That's intentional and mirrors `depot.go`/`baskets.go` exactly: a service only
ever ensures the existence of a stream _it_ owns; the streams it merely
subscribes to are assumed already ensured by their owning service (`baskets`,
which is already live). `foundationnats.NewSubscriber` takes `BasketsStreamName`
as a plain string argument to know where to consume from — it doesn't need to
have created it.

Second, look closely at `NewCreateOrder(orderRepo, publisher, logger)` — it's
called **twice**: once inline while building `subscriber`, and again a few lines
later to populate `Module.CreateOrderUC`. That's a real wart, not a stylistic
choice — two separate `*CreateOrder` instances now exist, both wrapping the same
`orderRepo`/`publisher`/`logger`, functionally identical but literally different
objects in memory. It's harmless _today_ because `CreateOrder` holds no mutable
state of its own, but it's exactly the kind of duplication that becomes a real
bug the moment someone adds state to that struct later and only remembers to
initialize one of the two copies. I left it as-is here specifically so we could
talk about it, rather than silently fixing it — want to clean it up now
(construct once, pass the same instance to both the subscriber and the `Module`
field), or note it and move on since it's cosmetic at this exact moment?

Also flagging, not fixing yet: this file has no `ConnectServer` field and no
`connectapi.NewServer(...)` call, unlike every prior `module.go` — that's
expected, since Phase 6 owns building the synchronous API surface, not this
phase.

Let me know your call on the duplicate `NewCreateOrder` construction, and once
that's settled we'll move to Step 6 — `main.go`.

### Step 6 -- main.go

Two files for this step: a health handler (mirroring `depot-http-health.go`,
since `ordering` has no `connectapi` server yet to piggyback health info onto),
and `main.go` itself.

```go
// ordering/cmd/service/health.go
```

A few things worth noting.

The router only registers `GET /health` — no ConnectRPC handler line
(`router.Handle(path, connectHandler.ServeHTTP)`), because there's no
`mod.ConnectServer` yet; that arrives in Phase 6. Everything else in `main.go` —
config load, logger, module init, router/middleware stack, `http.Server`
construction — is otherwise structurally identical to `depot-main.go`, since
none of that depends on whether an RPC server exists yet.

The real point of this step is the last line:
`foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout, orderingHTTPService, mod.Subscriber)`.
That's ADL 0012's `Service` interface doing exactly the job it was generalized
for — both the HTTP server and the NATS subscriber start concurrently in their
own goroutines, and a SIGINT/SIGTERM (or either one failing) triggers the same
coordinated shutdown sequence for both. This is only a two-listener case right
now (same shape `depot`/`baskets` already proved), not yet the three-listener
case Phase 5 will bring once `ShoppingListCompleted`'s subscriber exists — but
it's worth noticing this is the same pattern, just instantiated with a different
second `Service`.

Also worth flagging: unlike `depot-main.go`, there's no
`path, connectHandler := ...pbconnect.NewOrderingServiceHandler(...)` line at
all, because `ordering`'s `.proto`/generated code doesn't exist yet either —
that's also Phase 6 territory (the `orderingpb` package in the folder structure
hasn't been created). This `main.go` should compile and run as-is with zero
references to anything not yet built.

To actually run this, you'll need `go run ./ordering/cmd/service` (or add an
`ordering/run` target to the `Makefile`, following `stores/run`'s pattern, if
you'd rather not type the full path each time). Once it's up,
`curl localhost:8083/health` should return `ok`, and the real test will be
triggering an actual `baskets` checkout against the live `eda-baskets` container
and watching `ordering`'s logs pick up the `BasketCheckedOut` event and create a
real `Order` — want to try that now, or hold off until we've closed the
idempotency gap first so a real redelivery doesn't immediately demonstrate the
bug we already know about?

### Step 7 -- idempotency in CreateOrder.Execute

Two changes. First, `Execute` in `create_order.go` needs the check-before-create
guard:

```go
// ordering/internal/application/create_order.go
```

That replaces the existing `Execute` method body entirely. Note the added
`"errors"` import at the top of the file, alongside the existing
`"context"`/`"fmt"`/`"log/slog"`.

Second, `baskets.Handler` needs to stop generating a random ID and use the
basket's own aggregate ID instead:

```go

```

That replaces the `id := uuid.NewString()` line and the `Execute` call right
after it. The `"github.com/google/uuid"` import at the top of `handler.go` is
now unused and needs to be removed, or the build will fail.

A few things worth walking through.

The guard distinguishes two very different outcomes from `FindByID`:
`err == nil` means we've genuinely seen this basket before (a real redelivery,
or literally any repeat call with the same ID) — that's the happy idempotent
path, log it and return the existing order untouched, no domain reconstruction,
no publish. `errors.Is(err, domain.ErrOrderNotFound)` means this is a real
first-time creation — fall through to the original logic unchanged. Any _other_
error (a genuine Postgres failure, a connection drop) is a third case that must
not be silently treated as either "already exists" or "doesn't exist" — it gets
propagated as a real failure, since guessing wrong here would either silently
skip a legitimate creation or silently duplicate one, exactly the class of bug
we're trying to close.

Also worth noticing: this makes `CreateOrder.Execute` itself the thing that's
genuinely idempotent now, not just the handler that calls it. That matters
because `Execute` is also directly exercised by
`TestCreateOrder_Execute_Succeeds`, which called it once with a fresh
`uniqueID`-generated ID — that test's behavior is unaffected, since a fresh ID
always misses on `FindByID` and takes the creation path exactly as before. But
it does mean we now have an _untested_ branch — the "already exists, skip" path
— sitting in code that previously had no such branch at all.

Given the test-first discipline we've been using this phase, want me to write a
test proving the idempotent branch actually works (call `Execute` twice with the
same ID, confirm only one `OrderCreated` gets published and both calls return
the same order) before we consider this gap actually closed, or run it manually
first and add the test after?

Good instinct — and this is a case where checking `Execute`'s return value alone
isn't enough, since a duplicated publish is a NATS-stream-level bug, not
something either overload's return value would ever reveal. We need to verify
stream contents directly, the same discipline ADL 0015 used to catch the
original duplicate-publish bug in the first place.

```go
// ordering/internal/application/create_order_test.go
```

This needs two additional imports at the top of `create_order_test.go`:
`"context"` and
`"github.com/dupakarovsky/eda-storefront/ordering/internal/events"` (for
`events.OrderingStreamName`), alongside the existing `"errors"` and domain
import.

A few things worth walking through.

`streamMessageCount` measures a **delta**, not an absolute count —
`before`/`afterFirst`/`afterSecond` — specifically because this test doesn't run
in isolation. By this point in the suite, `TestCreateOrder_Execute_Succeeds`,
`TestCreateOrder_Execute_RejectsEmptyItems`, and every test in `baskets_test`
have already published real messages onto this same shared `ordering` stream
(they're all hitting the same live NATS/Postgres, not a fresh instance per
test). Asserting `afterFirst == 1` would be wrong and flaky depending on run
order; asserting `afterFirst == before + 1` is the only version of this claim
that's actually true regardless of what ran before it — same reasoning ADL
0015's own investigation applied when it inspected real stream state rather than
trusting an assumption about isolation.

Second, this is the one test in the suite so far that genuinely needs `context`
imported directly (rather than relying on `t.Context()` alone) — actually,
`t.Context()` still covers that; the import is needed because
`streamMessageCount`'s own signature takes `ctx context.Context` as an explicit
parameter rather than pulling it from `t` internally, keeping that helper
reusable outside a `*testing.T` if we ever needed it that way. If you'd rather
have it call `t.Context()` internally and drop the parameter, that's a fine
simplification — I kept it explicit mostly to make the helper's dependencies
obvious at the call site.

Third — and this is the real proof the fix works — the test doesn't just check
"no error happened twice," it specifically proves the stream's message count is
unchanged after the second call, which is the only way to actually verify
`publishDomainEvent` was never invoked on the redelivery path, distinct from
merely proving `Save` didn't error (an upsert wouldn't error either way,
publish-skipped or not).

Run this with `go test -v ./ordering/internal/application/...` — all three tests
in that package should now pass, including the new one. Let me know what you
get.

## Phase 4 — Orchestrating `depot`: `CreateShoppingList` + `AddItem`

**Goal:** after `CreateOrder` saves a new `Order` and publishes `OrderCreated`,
it synchronously calls `depot` to build the matching shopping list —
`CreateShoppingList(order_id)` once, then `AddItem` once per unit across every
item — and persists the returned `shopping_list_id` back onto the `Order` row.
This is also where we make a real decision about what happens when that
synchronous downstream call fails partway through, since nothing in this system
has had to handle that failure shape yet.

### Steps

**Step 1 — `depotclient` package.** Build the ConnectRPC client wrapper around
`depot`'s already-generated `depotpbconnect.DepotServiceClient` — two methods,
`CreateShoppingList` and `AddItem`, translating to/from `ordering`'s own domain
types so `internal/application` never imports `depotpb` directly. This is also
where we decide how `ordering` learns `depot`'s base URL (a config question —
I'll bring options to the check-in) and whether `depotclient` is exposed to the
application layer as a concrete type (matching how `*foundationnats.Publisher`
is used today) or as an interface (matching how `domain.StoreRepository` is
used). Worth deciding deliberately rather than defaulting.

**Step 2 — Wire `depotclient` into `ordering.go`.** Instantiate the client in
`Module.New`, alongside the existing Postgres/NATS wiring.

**Step 3 — Orchestration in `CreateOrder.Execute`.** After the existing
save-and-publish sequence, call `depotclient.CreateShoppingList`, then loop
`AddItem` once per unit, then `order.LinkShoppingList(...)` and a second `Save`.
This step is where we settle the failure-handling philosophy: what does
`CreateOrder.Execute` do if `CreateShoppingList` succeeds but the third
`AddItem` call fails? Order already exists and `OrderCreated` already published
— this is a genuinely new class of partial-failure to reason about, distinct
from ADL 0007's publish-retry gap.

**Step 4 — Verification.** A live check against the real running `depot` +
`ordering` + `baskets` stack (triggering a real basket checkout and confirming
the shopping list appears in `depot` with the right items), plus talking through
what an application-layer test for this orchestration would need (a fake
`depotclient` implementation, most likely).

Four steps feels like the right size — Steps 1 and 3 carry the real design
weight, 2 is mechanical wiring, 4 is where we prove it against the live stack
the same way every prior phase has.

Let me know if this breakdown looks right, or if you'd split/merge anything —
and once you're good with it, I'll start on Step 1.

## Step 1 The proto/client question

To call `depot` from `ordering`, we need Go bindings for `depot`'s protobuf
messages and its ConnectRPC client (`depotpb`/`depotpbconnect`). Two ways to get
them:

**Option A — import `depot`'s own generated package directly**, via `go.work`
(which already links every module in this workspace together for exactly this
kind of local cross-module development, per `roadmap.md`'s own stated purpose
for it). Zero duplication, zero extra buf setup.

**Option B — copy `depot`'s `.proto` file into `ordering`'s own folder and
generate a private local copy** of the client bindings there, never importing
`depot`'s module at all. This is the literal scenario ADL 0004 anticipated and
reserved space for ("`.proto` files... promoted to the same tier as
`foundation/`... once adopted") — it keeps `ordering` decoupled from `depot`'s
module the same way `baskets` refuses to import `stores`' internal packages for
event consumption.

I'm going to recommend **Option A** for this step, specifically because
`go.work` exists in this system for exactly this reason, and a generated RPC
client stub is a wire-format artifact, not domain logic — it's a different
category from the "don't import another service's implementation" rule ADL 0004
is really protecting against. But this is a real trade-off (a silent,
compiler-enforced coupling to `depot`'s module going forward), so tell me if
you'd rather go Option B — it's a bit more ceremony (you'd need to set up
`buf.yaml`/`buf.gen.yaml` for `ordering` for the first time) but keeps the
isolation guarantee airtight.

Assuming Option A, here's Step 1:

### 1. `foundation/config.go` — add depot's URL

```go
// foundation/config.go
```

```go
// foundation/config.go
```

### 2. `ordering/internal/domain/depot_client.go` — the port

```go
// ordering/internal/domain/depot_client.go
```

### 3. `ordering/internal/depotclient/client.go` — the adapter

```go
// ordering/internal/depotclient/client.go
```

**What's happening here:**

The `depotclient.Client` never appears in `ordering`'s `application` layer
directly — `CreateOrder` (Phase 4's Step 3) will depend on `domain.DepotClient`,
the interface, not this concrete type. That mirrors exactly how `CreateOrder`
already depends on `domain.OrderRepository` rather than
`*postgres.OrderRepository`. The payoff is concrete for Step 4: a test can hand
`CreateOrder` a fake `DepotClient` that returns canned IDs and never touches the
network, the same way existing application tests presumably could fake a
repository if they needed to (though in practice this codebase has favored
testing against real Postgres/NATS — this is the first synchronous
_cross-service_ dependency, so a fake is much more attractive here than a
repository fake ever was).

Note `Client.CreateShoppingList` returns a plain `string`, not a
`*depotpb.ShoppingList` — the translation from wire type to something
`ordering`'s domain layer can use happens right here, at the boundary, the same
discipline `toProtoOrder`/`toProtoStore`-style functions apply everywhere else
in this system, just running in the opposite direction (protobuf response →
plain Go value) since we're now a client instead of a server.

`Config` here is deliberately just `{URL string}` — a minimal package-owned
config struct, populated by whoever constructs the client (Step 2), mirroring
`postgres.Config`/`foundationnats.Config`'s own shape: each infra package owns a
tiny config type, and the caller assembles it by copying fields off the one
shared `foundationconfig.Config` that actually reads the environment.

One thing to flag: `foundation/config.go` is shared code every service imports.
This edit is purely additive (a new optional field with a default, nothing
existing changes shape), so `stores`, `depot`, and `baskets` are unaffected —
but it's still a `foundation/` edit, the same category of shared-surface change
ADL 0007's addendum and ADL 0015 both were.

Go ahead and create these three files (plus the `Depot` field/`DepotConfig`
addition to your existing `foundation/config.go`), then run `go build ./...`
from the workspace root. This won't run anything yet — there's no caller wired
up — but it should compile cleanly, and a successful build is actually the real
test here: it confirms `go.work` correctly resolves `ordering`'s import of
`depot`'s generated package across module boundaries. Let me know what you get,
and let me know your call on the Option A/B question too.

### Step 2 `ordering/model.go` — imports

```go

```

### `ordering/ordering.go` — inside `New`, after the repository is initialized

```go

```

### `ordering/ordering.go` — in the returned `&Module{...}` literal

```go

```

**What's happening here:**

`Module.DepotClient` is typed as `domain.DepotClient` — the interface — not
`*depotclient.Client`, the concrete type `depotclient.New` actually returns.
This is the same substitution `StoreRepo domain.StoreRepository` already does in
`stores-module.go` (`storeRepo := postgres.NewStoreRepository(pool)` assigned
into a field typed as the interface). Go allows this implicitly because
`*depotclient.Client` satisfies `domain.DepotClient`'s method set — nothing
forces the assignment to be explicit, but keeping the _field_ typed as the
interface is what actually lets `CreateOrder` (Step 3) depend on the abstraction
rather than the concrete adapter, even though `Module` is the thing constructing
the concrete value.

`depotclient.New` only needs `cfg.Depot.URL` — a single string — so unlike
Postgres or NATS there's no connection to open, no error to handle, no
`defer`/close lifecycle at all. A ConnectRPC client is just a configured HTTP
client wrapper; the "connection" is per-request, not persistent. That's why this
block is much shorter than the Postgres/NATS sections above it, and why
`Module.Close()` doesn't need a new line for it.

One thing I intentionally left out:
`CreateOrderUC := application.NewCreateOrder(orderRepo, publisher, logger)` a
few lines further down is untouched for now — it doesn't yet receive
`depotClient`. That's correct for this step (nothing calls it yet, so passing it
early would just be dead wiring), but flag it if you'd rather do it in one
motion now instead of splitting the constructor-signature change into Step 3.

Go ahead and make these edits, then `go build ./...` again from the workspace
root — should still compile clean, same "does it resolve" check as Step 1, just
now proving `Module` itself constructs correctly with the new field. Let me know
how it goes.

### changes to depot

Good — small and contained, exactly the size we agreed on. Two files change on
`depot`'s side, both mechanical once you see them side by side with what's
already there.

### 1. `depot/internal/domain/shopping_list_repository.go` — new interface method

```go
// depot/internal/domain/shopping_list_repository.go
```

### 2. `depot/internal/postgres/shopping_list_repository.go` — the implementation

```go
// depot/internal/postgres/shopping_list_repository.go
```

### 3. `depot/internal/application/create_shopping_list.go` — the idempotency check

```go
// depot/internal/application/create_shopping_list.go
```

```go
// depot/internal/application/create_shopping_list.go
```

**What's happening here:**

`FindByOrderID` is copy-shaped off `FindByID`/`FindByIDForUpdate`'s existing
`find` pattern in the same file, just filtering on a different column and
skipping the `FOR UPDATE` variant entirely — nothing in this design needs row
locking here, since we're not mutating on this path, only reading to decide
whether to skip creation.

The `LIMIT 1` is doing real work, not just being defensive: nothing in the
schema stops two rows from sharing an `order_id` (no unique constraint), and if
you'd already hit the duplicate-creation bug once before this fix landed, there
may already be more than one row out there for the same order. `LIMIT 1` means
this always resolves deterministically to _some_ row rather than erroring on
`pgx`'s "multiple rows returned" behavior — it won't clean up a pre-existing
duplicate, but it also won't crash on one.

`Execute`'s new shape is a direct mirror of `ordering.CreateOrder.Execute`'s own
idempotency check — `FindByOrderID` first, return the existing aggregate on a
hit, fall through to real construction only on `ErrShoppingListNotFound`. Same
pattern, one layer down, which is exactly the point: `depot` is now idempotent
on `order_id` the same way `ordering` is idempotent on basket ID, closing the
symmetry that was missing before.

One thing to be honest about: the `id` parameter `Execute` receives
(server-generated fresh on every RPC call, per `connectapi`'s
`CreateShoppingList` handler) is simply discarded when an existing list is found
— a wasted UUID generation on every retry, harmless but slightly wasteful. Not
worth fixing; `ordering`'s own `CreateOrder.Execute` already accepts the same
minor waste for the same reason.

For testing: this only closes Problem A (duplicate creation) — `AddItem` is
still unguarded, so calling it in a loop from Phase 4's current
`orchestrateShoppingList` would still double-count on any retry, until we do the
`GetShoppingList`-delta piece next. So a clean test here is narrow and
deliberate: call `depot`'s `CreateShoppingList` twice in a row with the **same**
`order_id` (via `curl` against your running stack, or a small Go test) and
confirm the second call returns the _same_ `shoppingList.id` as the first,
rather than a new one. Don't chain it into a full `AddItem` sequence yet —
that's the next piece, and testing it now would just confirm the half-fixed
state you already understand.

Go ahead and make these three edits, rebuild `depot`, and try the
double-`CreateShoppingList` check. Let me know what you get.

That confirms both branches cleanly: the fresh `order_id` hit
`publish_event.go`'s real publish path once (`seq=84`), and the identical second
call skipped straight to the `create_shopping_list.go:41` log line with no new
publish — exactly the miss-then-hit sequence needed. Problem A is closed and
verified, not just assumed.

Problem B now. `orchestrateShoppingList` needs to know what's already in the
list _before_ deciding how many `AddItem` calls to make, and `depot`'s existing
`GetShoppingList` RPC already returns exactly that — we just need to flatten its
response into something the loop can subtract against.

### 1. `ordering/internal/domain/depot_client.go` — new interface method

```go
// ordering/internal/domain/depot_client.go
```

### 2. `ordering/internal/depotclient/client.go` — the adapter

```go
// ordering/internal/depotclient/client.go
```

### 3. `ordering/internal/application/create_order.go` — delta-aware loop

```go
// ordering/internal/application/create_order.go
```

**What's happening here:**

The map is keyed by `productID` alone, not `(storeID, productID)`. That's safe
specifically because a product belongs to exactly one store in `stores`' own
data model — `stores.Product.StoreID` is fixed at creation, never reassigned —
so `item.ProductID` alone is already unambiguous as a lookup key across every
stop `depot` groups the list into. If products could ever move between stores,
this key would need to widen; they can't, so it doesn't.

`existingQuantities[item.ProductID]` on a Go map returns the zero value (`0`)
when the key isn't present — so a first-time run (fresh shopping list, empty
`GetShoppingList` response) computes `remaining := item.Quantity - 0`, i.e. the
full quantity, identical to the old unconditional loop. The delta logic is a
strict generalization of the original behavior, not a parallel code path —
first-time orchestration and resumed orchestration now go through the exact same
arithmetic.

Notice `GetShoppingList`'s own failure is treated as fatal —
`orchestrateShoppingList` returns immediately rather than falling back to "just
add everything anyway." That's a deliberate fail-closed choice: if we can't
determine what's already there, guessing and adding blindly is precisely the
double-counting bug we're trying to eliminate, so an unreadable state has to
halt the orchestration rather than risk silently reintroducing Problem B under a
different trigger.

One thing this doesn't handle, worth flagging before you test rather than after:
if `AddItem` itself fails _midway_ through a single unit-by-unit loop for one
product (say 2 of 3 units succeed, the 3rd fails), the function returns
immediately on that error — it doesn't finish adding the remaining items for
_other_ products in `order.Items` first. On the next retry, `GetShoppingList`
will correctly see that partial 2-of-3 progress and only add the 1 remaining
unit for that product, so nothing double-counts — but any products listed
_after_ the failed one in `order.Items` won't get touched until that retry
either, even though nothing was stopping them individually. That's a minor
completeness-ordering quirk, not a correctness bug — everything still converges
correctly on retry — but it's worth knowing the loop isn't parallel or
independent per-item.

For testing: this is harder to verify with a bare `curl` sequence than Problem A
was, since simulating "an `AddItem` call fails partway" from the outside isn't
something you can trigger on demand without either stopping `depot` mid-sequence
or adding a deliberate failure point. Do you want to test this by manually
inserting a partial state directly into `depot`'s `shopping_list_items` table
(simulating "attempt 1 got interrupted here") and then confirming a fresh
orchestration call only adds the difference — or would you rather do a real
end-to-end trigger (an actual basket checkout through `baskets`) and just
confirm normal-path behavior first, saving the partial-failure simulation for
after?

## Phase 5

### Proposed Phase 5 Step Plan

**Step 1 — `CompleteOrder` application use case.** A new
`application.CompleteOrder` type, structurally parallel to `CreateOrder`: takes
a `shoppingListID`, looks up the `Order` via the repository's
`FindByShoppingListID` (new lookup, per Phase 1's deferral — this is where it
finally gets built), calls `order.MarkReadied()`, saves, and publishes
`OrderReadied`. We'll need to decide together how it should behave if the order
is already `readied` (idempotency under redelivery, same discipline as
`CreateOrder`).

**Step 2 — Repository: `FindByShoppingListID`.** Add this method to
`domain.OrderRepository`'s interface and implement it in
`postgres.OrderRepository` — a straightforward indexed lookup, mirroring
`depot`'s own `FindByOrderID` we just saw added in Phase 4/CONTRACT.

**Step 3 — `depot` event consumer package.** A new `internal/depot` package
(mirroring `internal/baskets`'s shape) with its own local envelope/payload types
for `ShoppingListCompleted` (per ADL 0004 — no importing `depot`'s internal
packages), and a `Handler` function wired to `CompleteOrder`.

**Step 4 — Second `Subscriber` + `Module`/`main.go` wiring.** Extend
`ordering.go`'s `Module` with a second `foundationnats.Subscriber` against the
`depot` stream, and extend `main.go`'s `foundationshutdown.Run(...)` call to a
third concurrent listener (HTTP + baskets subscriber + depot subscriber) — the
multi-listener pattern from ADL 0012, now at its widest point in this system.

**Step 5 — Live verification / smoke test.** Once wired, we'll talk through what
a manual end-to-end check looks like (trigger a real basket checkout through to
a completed shopping list, confirm the `Order` in Postgres flips to `readied`).

A couple of open questions before we start Step 1: should `CompleteOrder` treat
"order not found for this shopping list ID" as a hard error (log and drop) or
dead-letter it, given this is a correlation lookup rather than a straightforward
not-found on a primary key? And for idempotency — same pattern as `CreateOrder`
(check current state, skip re-work if already `readied`), or did you want to
discuss alternatives?

## Step 1-CompleteOrder\` application use case

Step 1 is the `CompleteOrder` use case. It needs `FindByShoppingListID` on the
repository interface, but that method is Step 2. To keep `go build ./...` clean
in between, we'll add the method to the interface now, with a temporary stub in
the Postgres repository (the placeholder pattern).

**1a. Interface addition**

```go
// ordering/internal/domain/order_repository.go (added FindByShoppingListID)
```

**1b. Temporary stub**

```go

```

**1c. The use case**

```go
// ordering/internal/application/complete_order.go (new file)
```

**What's happening here**

The structure mirrors the command use cases you've already built for `stores`
and `depot`: a locked transaction that finds, mutates, and saves the aggregate,
followed by a publish outside the transaction. The interesting part is the
two-step lookup inside `WithLock`. The event we consume only carries the
shopping list's ID, so we first have to resolve which `Order` it belongs to, and
that is `FindByShoppingListID`. That lookup is a plain `SELECT` with no
`FOR UPDATE`, following the same convention as `FindByID`. It therefore can't be
what serializes concurrent redeliveries. So once we know the order's ID, we call
`FindByIDForUpdate`, which takes the row lock and reloads the state as it is
now. From that point on, every decision (is it already readied? is it canceled?)
is made against locked, fresh data. That is the concurrency reason Phase 1 Step
4 chose row-level locking for `Order`: two near-simultaneous deliveries of the
same event hit the same row. The second one blocks at `FindByIDForUpdate` until
the first commits, then sees `readied` and takes the idempotent path.

The idempotency check follows the `CreateOrder` pattern of inspecting state
first and skipping the work if it is already done. It differs in one way.
`MarkReadied` would itself return `ErrOrderAlreadyReadied`, but for a redelivery
that is a success case, not a failure, and returning the error would push a
harmless duplicate through five backoff retries into the dead letter. So we
check `Status` ourselves before calling it. The `alreadyReadied` flag is needed
because returning `nil` from the callback commits an empty transaction. We then
have to remember, after `WithLock` returns, that there is no event to publish.
The canceled case is deliberately not special-cased. `MarkReadied` returns
`ErrOrderAlreadyCanceled`, which surfaces as a hard error. That is consistent
with your decision to treat unexpected correlation states as hard errors for now
and see what the tests tell us.

One consequence to be aware of, which is the same gap `CreateOrder` has. If the
save commits but `publishDomainEvent` exhausts its retries, the message gets
redelivered, sees `readied`, and skips the publish, so `OrderReadied` is never
sent. That is exactly the gap ADL 0007's deferred outbox names. We're inheriting
it here, not introducing it.

**To run:** from the workspace root, `go build ./...` and then
`go vet ./ordering/...`. Both should be clean with no output. Nothing calls
`CompleteOrder` yet, so there's no runtime behavior to observe.

Two things I noticed while reading the files you uploaded. First, the
`ordering-module.go` I have looks like the Phase 3 version. It calls
`NewCreateOrder(orderRepo, publisher, logger)` with three arguments, while
`create_order.go` takes four (with `depotClient`), and it has no `DepotClient`
field. I'm assuming your real one is already updated per Phase 4, but tell me if
the upload was stale, since we'll be editing that file in Step 4. Second, the
`domain.DepotClient` and `depotclient` files weren't in the upload, which is
fine because we don't need them for this phase.

Give the build a run and let me know how it goes. Any questions on the two-step
lookup or the `alreadyReadied` flag before we move on to Step 2, the real
`FindByShoppingListID` query?

## Phase 6

**Phase 6 goal:** give `ordering` its own ConnectRPC surface — `GetOrder` and
`CancelOrder` — following the exact same shape `depot`/`baskets` already
established (`connectapi/server.go`, `order.go`, `proto.go`, `errors.go`, an
`orderingpb/api.proto`, wired into `ordering.go` and `main.go`).

Here's the Step breakdown I'd propose:

**Step 1 — Design decision: what does `CancelOrder` actually do?** This is the
one genuinely open question Phase 5 flagged, and it's worth settling before
writing any code, the same way ADL 0016 settled the orchestration question
before Phase 3 started. The core fork: does canceling an `Order` reach
synchronously into `depot` to cancel the linked shopping list too, or are the
two lifecycles allowed to diverge (an `ordering`-side cancellation that leaves
`depot`'s shopping list to complete on its own)? I have a recommendation, but
I'd rather walk through the tradeoff with you first since it's a real design
call, not mechanical work — this'll be a discussion step, not a code step.

**Step 2 — `orderingpb/api.proto`** Define `Order` (mirroring the domain struct:
id, status, items, shopping_list_id), `GetOrderRequest/Response`,
`CancelOrderRequest/Response`. You'll run `buf generate` locally same as before.

**Step 3 — `connectapi` package** `server.go`, `order.go` (the two handlers),
`proto.go` (`toProtoOrder`), `errors.go` (`mapError` for `ordering`'s own
sentinel errors — some already exist like
`ErrOrderAlreadyReadied`/`ErrOrderAlreadyCanceled`).

**Step 4 — `CancelOrder` use case** Shape depends on Step 1's decision — either
a straightforward `application.CancelOrder` calling `order.Cancel()`, or one
that also calls into `depotClient` first.

**Step 5 — Wiring** `ordering.go`'s `Module` gains
`GetOrderUC`/`CancelOrderUC`/`ConnectServer`; `main.go` registers the Connect
handler path, same pattern as `stores`/`depot`/`baskets`.

**Step 6 — Live verification** A curl script (`ordering-http-connect.sh`-style)
exercising the full loop end-to-end: checkout → order created → shopping list
orchestrated → `GetOrder` mid-flight → complete → `GetOrder` shows `readied` → a
fresh order → `CancelOrder` → verify terminal-state guards
(cancel-after-readied, cancel-after-already-canceled, get/cancel against unknown
IDs).

This also finally gives us a live way to test the gap Phase 5 named
(`ShoppingListCompleted` arriving for an already-canceled order) if you want to
fold that into Step 6.

### Step 1

Good — let's dig into Step 1.

Here's the fork, stated plainly: when someone calls `ordering.CancelOrder`,
should that handler _also_ place a synchronous RPC call to
`depot.CancelShoppingList`, or should it only touch `ordering`'s own `Order` row
and let `depot`'s shopping list keep going on its own?

Let me lay out what I see as the real considerations, then tell you where I'd
lean.

**The case for reaching into `depot` (symmetric with `CreateOrder`'s
orchestration):** `ordering` already treats itself as the orchestrator — Design
A in ADL 0016 was explicitly "`ordering` drives `depot`, not the other way
around." If `Order` can command `depot` to _build_ a shopping list, it's at
least consistent for it to also command `depot` to _tear one down_ on
cancellation. Without this, you'd get a real, visible inconsistency: a customer
cancels their order in `ordering`, but somewhere in a warehouse, `depot`'s
shopping list is still `active`, someone finishes shopping for it, publishes
`ShoppingListCompleted`, and `ordering`'s `depot` subscriber picks that up and
tries to call `MarkReadied()` on an order that's already `canceled`. That's
exactly the untested interaction Phase 5's gap section named.

**The case against it:** it's a new failure mode, and a nastier one than
`CreateOrder`'s. `CreateOrder`'s orchestration only ever needs to succeed
_eventually_ — retries and idempotent calls handle that fine, nothing else is
racing against it. `CancelOrder` is different: it's racing against `depot`'s own
independent state machine. By the time `ordering.CancelOrder` fires its RPC, the
shopping list might already be `completed` in `depot` (someone finished shopping
before the cancellation reached it), and `depot.CancelShoppingList` will
legitimately reject that with `ErrShoppingListAlreadyCompleted`. So `ordering`
would need to treat that specific rejection as an acceptable, expected outcome
("we tried to cancel, but it already finished — that's fine, not an error")
rather than a failure to propagate to the caller. That's real branching logic,
not a mechanical RPC call.

**Where I'd lean, and why:** call into `depot`, but make the failure handling
narrow and deliberate rather than a blanket "ignore any depot error."
Specifically: call `CancelShoppingList`, and treat
`ErrShoppingListAlreadyCompleted`/`CodeInvalidArgument`-with-that-specific-cause
as a benign no-op (log it, proceed with canceling the `Order` anyway), but let
any other `depot` failure (unreachable, `CodeInternal`, etc.) propagate as a
real error — same "distinguish rather than swallow" discipline this project's
used everywhere else (ADL 0007's return-the-error-with-the-aggregate pattern,
`mapError`'s explicit code mapping).

One more wrinkle this surfaces: what should happen if `Order` is already
`readied`? `Cancel()` already guards against that (`ErrOrderAlreadyReadied`) —
meaning if `depot` already completed the list and `ordering` already advanced to
`readied`, `CancelOrder` never even reaches the `depot` call at all, since the
domain guard fires first. So the race window is narrower than it might first
sound: it's specifically the gap between "`depot` completes the list" and
"`ordering`'s subscriber processes `ShoppingListCompleted`" — not the whole
order lifecycle.

What's your read — does reaching into `depot` with that narrow exception feel
right to you, or would you rather keep `CancelOrder` local to `ordering` and
treat the divergent-state case as an accepted, named gap (the way several other
things in this project have been)?

Good — that's the design we'll build. One quick note before I write anything:
the way we'll actually detect "the shopping list was already in a terminal
state" without violating ADL 0004 (no importing `depot`'s internal error types)
is by checking the returned `connect.Error`'s **code**, not its message text.
Looking at `depot`'s own `mapError`, both `ErrShoppingListAlreadyCompleted` and
`ErrShoppingListAlreadyCanceled` map to `CodeInvalidArgument` — and those are
the only two ways `CancelShoppingList` can reject a request that already has a
valid ID. So the rule becomes: `CodeInvalidArgument` from that specific call →
benign, the list already reached a terminal state, proceed with canceling the
`Order` anyway. Anything else (`CodeNotFound`, `CodeInternal`, network failure)
→ a real error, propagate it. That keeps the distinction code-based and
contract-safe rather than parsing error strings.

## Step 2

Here's Step 2 — extending the `DepotClient` port and its concrete
implementation.

First, the interface gets one more method, in the same style as its three
siblings:

```go
// ordering/internal/domain/depot_client.go
```

And the thin implementation, added to `depotclient.Client` alongside
`AddItem`/`GetShoppingList`:

```go
// ordering/internal/depotclient/client.go — add this method
```

What's happening here: this method is deliberately dumb. It doesn't inspect the
error at all — it wraps whatever `connect` gives it with `%w` and hands it back.
That `%w` (not `%v`) is the one detail that matters for Step 3 to work: it
preserves the underlying `*connect.Error` inside the wrapped error, so the
caller can later use `errors.As` to pull the original `connect.Error` back out
and inspect its `.Code()` — the judgment call we talked about lives entirely in
`application.CancelOrder`, not here. This keeps all four `DepotClient` methods
symmetric: every one of them is "make the call, wrap the error, return" — no
branching, no knowledge of what any particular error code _means_ to the caller.
That knowledge belongs to whoever has the context to decide it's benign, which
is `CancelOrder`, not a generic RPC client.

One thing to double check on your end before we move on: does `depot/depotpb`
already export a `CancelShoppingListRequest` type with a `ShoppingListId` field?
Looking at the `depot-api.proto`/`depot-api.pb.go` you gave me earlier,
`CancelShoppingListRequest` does exist with exactly that field — so this should
compile against what's already generated for `depot`, no proto regeneration
needed on `depot`'s side for this step.

Go ahead and add these two changes. Once you've confirmed it builds
(`go build ./ordering/...` or however you usually check), let me know and we'll
move to Step 3: the `application.CancelOrder` use case itself, where the actual
`connect.Code` inspection happens.

### Step 3

Here's Step 3 — wait, let me keep the numbering loose the way this session's
already drifted (DepotClient extension, now the use case) rather than force it
back to the original Step 2/3/4 labels. This is `application.CancelOrder`.

```go
// ordering/internal/application/cancel_order.go

```

What's happening here, and a few things worth flagging:

The `WithLock`/`FindByIDForUpdate` pair here is actually the _first_
correctly-scoped use of row locking in `ordering`'s use cases. Both
`CreateOrder` and `CompleteOrder` have a known, named gap (Phase 4/5's
"check-then-act race") because they each need a non-locking lookup first
(`FindByID`, `FindByShoppingListID`) before they can even get an ID to lock on.
`CancelOrder` doesn't have that problem — it's handed the order ID directly by
the caller, so it can go straight into `WithLock` → `FindByIDForUpdate` with no
unlocked window beforehand. Worth noting as a small positive, not something to
fix elsewhere right now.

The `depot` call sits deliberately _outside_ the transaction, using `ctx` rather
than `txCtx` — same placement `publishDomainEvent` already uses everywhere else
in this codebase, for the same reason: the local `Order` row is already durably
committed by this point, so there's nothing left to roll back regardless of what
happens next.

The `errors.As(err, &connectErr)` line is the payoff from keeping
`depotclient.Client.CancelShoppingList` a thin `%w`-wrapping passthrough.
`connect.Error` is a concrete type connect-go returns from any RPC call, and
wrapping with `%w` at each layer preserves it deep enough in the chain for
`errors.As` to find it. If the code is `CodeInvalidArgument`, we treat it as
"the shopping list already reached a terminal state on its own" and let
cancellation succeed anyway. Anything else — the depot service being
unreachable, a `CodeInternal`, `CodeNotFound` on the shopping list ID somehow —
falls through to the `else` and gets returned as a real error, with `order`
still handed back non-nil, matching this whole codebase's established pattern of
"here's the aggregate, and here's what didn't finish."

One thing I did _not_ add: a distinction between `depot` returning "already
completed" versus "already canceled" — both are `CodeInvalidArgument` from
`depot`'s own `mapError`, and for our purposes here they mean the same thing
("nothing left for you to cancel"), so we don't need to tell them apart.

This use case has no wiring yet (no `Module` field, nothing calling `Execute`) —
same as Phase 1's domain/postgres layers, it should just compile standalone
right now. Go ahead and run `go build ./ordering/...` and let me know if it's
clean. Any questions on the locking placement or the error-code check before we
move to the proto definitions?

### Step 4

Now the proto definitions — this is the first proto file `ordering` has needed,
since everything built so far consumed other services' RPCs rather than exposing
its own.

```protobuf
// ordering/orderingpb/api.proto
syntax = "proto3";

package ordering.v1;

option go_package = "github.com/dupakarovsky/eda-storefront/ordering/orderingpb;orderingpb";


// ==== Data types ============================================================

message Item {
  string product_id = 1;
  string store_id = 2;
  int32 quantity = 3;
}

message Order {
  string id = 1;
  string status = 2;
  repeated Item items = 3;
  string shopping_list_id = 4;
}

// ==== Messages ============================================================

// === GetOrder ===
message GetOrderRequest {
  string order_id = 1;
}
message GetOrderResponse {
  Order order = 1;
}

// === CancelOrder ===
message CancelOrderRequest {
  string order_id = 1;
}
message CancelOrderResponse {
  Order order = 1;
}

// ==== Services ============================================================

service OrderingService {
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc CancelOrder(CancelOrderRequest) returns (CancelOrderResponse);
}
```

A few choices worth explaining:

`Item` here carries `store_id`, unlike `depot`'s wire-format `Item` (which only
has `product_id`/`quantity`, since `depot` groups by `Stop` instead). This
mirrors the domain: `ordering.internal.domain.Item` already carries `StoreID`
for exactly the reason journey_progress.md's Phase 1 notes call out — `ordering`
is the one service that needs to hand `store_id` to `depot.AddItem` per unit, so
it has to own that field itself rather than deriving it from a grouping
structure.

`Order.items` is a flat `repeated Item`, matching `domain.Order.Items []Item`
directly — no `Stops`-style grouping the way `depot`'s wire format has, because
`ordering` never needed that grouping internally.

Only two RPCs on the service, matching what Phase 6 actually needs — no
`CreateOrder` exposed here, since order creation only ever happens via the
`baskets` subscriber, never as a direct synchronous call from an external
caller. That's a deliberate asymmetry worth noticing: `depot`/`baskets`/`stores`
all exposed full CRUD-shaped RPC surfaces because they're driven directly by
callers, but `ordering`'s creation path is event-driven only — its synchronous
surface exists purely for querying and canceling something that already exists.

Go ahead and drop this at `ordering/orderingpb/api.proto` and run `buf generate`
the way you did for `depot`/`baskets`. Once that's produced `api.pb.go` and
`api.connect.go` under `orderingpb`/`orderingpb/orderingpbconnect`, let me know
and we'll move to the `connectapi` package — `proto.go`, `errors.go`,
`server.go`, and `order.go` — which is where those generated types actually get
used.

### Step 5

Before the `connectapi` package, we're missing one small piece: a `GetOrder`
query use case. Everything built so far either mutates
(`CreateOrder`/`CompleteOrder`/`CancelOrder`) or reads via a repository method
directly inside another use case — there's no standalone read-only use case yet,
the kind ADL 0008 requires sitting between `connectapi` and the repository even
for a plain lookup.

```go
// ordering/internal/application/get_order.go
```

This is deliberately as thin as `depot.GetShoppingList`/`baskets.GetBasket` — no
logic beyond the repository call, per ADL 0008's own reasoning: the point isn't
that this use case does anything yet, it's that `connectapi.Server` is never
allowed to call `OrderRepository` directly, read or write, so every RPC needs a
real `application` type standing between them regardless of how little work it
does today.

Now the `connectapi` package — four files, all structurally identical to
`depot`'s equivalents.

```go
// ordering/internal/connectapi/proto.go
```

```go
// ordering/internal/connectapi/errors.go
```

```go
// ordering/internal/connectapi/server.go
```

```go
// ordering/internal/connectapi/order.go
```

A couple of things worth flagging, since this is `ordering`'s first time on this
side of `mapError`:

`errors.go`'s error list only maps `Order`'s own domain sentinels — it doesn't
need to know anything about `depot`'s errors, even though `CancelOrder`'s use
case now talks to `depot`. That's because `CancelOrder.Execute` already resolved
the `depot`-side ambiguity itself (the `connect.CodeInvalidArgument` check we
wrote last step) before returning — by the time an error reaches `mapError`
here, it's either a genuine `ordering`-domain error or an opaque
`depot`-communication failure that correctly falls through to the
`default: CodeInternal` case. `mapError` staying ignorant of `depot`'s specific
errors is the payoff of having done that filtering one layer down, in
`application`, rather than pushing it all the way up to the API boundary.

Notice `ErrOrderNotFound` is the _only_ thing mapped to `CodeNotFound` here —
unlike `depot`'s `mapError`, which has two sentinels sharing that code
(`ErrStoreNotFound`, `ErrProductNotFound`). `ordering` only has one aggregate
exposed through this API, so there's only one way to get a 404.

Go ahead and try `go build ./ordering/...` — this should compile clean against
what `buf generate` produced, but this is the first real point where a mismatch
between the proto field names I guessed (`OrderId`, `ShoppingListId`, etc.) and
what `buf` actually generated would surface. Let me know what happens.

### Step 6

Now we wire this into `ordering.go` (the `Module`) and `cmd/service/main.go` —
the same two-file pattern every prior service used (see
`stores-module.go`/`stores-main.go`, `depot-module.go`/`depot-main.go`).

```go
// ordering/ordering.go — add these imports
```

```go
// ordering/ordering.go — Module struct gains these fields
```

```go
// ordering/ordering.go — inside New, after createOrderUC/completeOrderUC are constructed
```

```go
// ordering/ordering.go — the returned &Module{...} literal gains these fields
```

What's happening here: this follows `depot.go`'s exact construction order —
build every use case first, then hand the ones the RPC layer needs into
`connectapi.NewServer`, then place everything into the `Module` literal at the
end. `cancelOrderUC` needs `depotClient` and `publisher`/`logger` (matching its
constructor signature from two steps ago), while `getOrderUC` only needs
`orderRepo`, same asymmetry `depot`'s own `getStoresUC`/mutating-use-case split
already has.

Now `main.go`:

```go
// ordering/cmd/service/main.go — add this import
```

```go
// ordering/cmd/service/main.go — after
// mod, err := ordering.New(...) and before router.Use(...)
```

```go
// ordering/cmd/service/main.go
//replace the existing router.Handle("GET /health", ...) line with both routes
```

One thing to double check as you place that: looking at your current
`ordering-main.go`, `router.Handle("GET /health", ...)` is called _before_
`handler := router.Build()`, same as every other service's `main.go` — make sure
the new `router.Handle(path, connectHandler.ServeHTTP)` line goes in that same
block, before `Build()` is called, not after. `stores-main.go` and
`depot-main.go` both register their Connect handler path right alongside the
health route for exactly this reason — `Build()` finalizes the mux, so anything
registered after it is silently never reachable.

Also worth a quick sanity check on your end: does `docker-compose.yaml`
currently publish a host port for `ordering` (like `8080`→stores,
`8081`→baskets, `8082`→depot)? I don't see one in the compose file you gave me —
if it's still missing, we'll need to add one before the curl verification step,
something like `127.0.0.1:8083:8080` matching the `ORDERING-HTTP` service name
already used in your shutdown wiring.

Go ahead and make these changes, run `go build ./ordering/...`, and let me know
how it goes — and whether `ordering` already has a compose port mapping or if we
need to add one.
