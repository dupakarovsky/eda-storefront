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
`foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout,
orderingHTTPService, mod.Subscriber)`.
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
