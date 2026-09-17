# 16. `ordering` orchestrates shopping list creation via synchronous call, not choreography

Date: 2026-09-17

## Status

Accepted

## Context

`ordering` (Project 3) is the first service in this system with two live
upstream event dependencies (`baskets`' `BasketCheckedOut`, `depot`'s
`ShoppingListCompleted` — ADL 0001), and the first whose aggregate creation
requires a genuine side effect in another already-live service:
`depot-CONTRACT.md`'s own non-goals section already named this, stating
`CreateShoppingList`'s book-native design "is created from an Order" and
flagging its current zero-argument signature as provisional pending this
project.

This raised two separable questions, addressed together here since the
second answer depends on the first.

**Where does `Order`'s item data come from?** Neither `BasketCheckedOut` nor
`ShoppingListCompleted` carries item data today — both `baskets-CONTRACT.md`
and `depot-CONTRACT.md` flag this explicitly as a decision deferred until a
real consumer existed to make the requirement concrete. Two options: enrich
the events themselves, or have `ordering` call back synchronously
(`GetBasket`/`GetShoppingList`, both already-live read-only RPCs) the moment
it consumes the bare notification event.

**How does `ordering` get `depot` to build the fulfillment list?** Two
options: **orchestration** — `ordering` calls `depot.CreateShoppingList`
directly as a synchronous command, the moment it creates an `Order`; or
**choreography** — `ordering` publishes its own `OrderCreated` event, and
`depot` gains a second `Subscriber` (alongside its existing `stores`-catalog
projector) that reacts to it by creating the list itself, publishing
`ShoppingListCreated` (carrying `order_id`) back for `ordering` to complete
the correlation.

## Decision

**Item data: enrich the events, not synchronous read-back.** `baskets`'
`BasketCheckedOut` payload gains an item list (`product_id`, `store_id`,
`quantity` — `store_id` resolved from `baskets`' own catalog cache at
checkout time, since `Basket.Item` itself carries no store reference).
`depot`'s `ShoppingListCompleted` needs **no** payload change — see
Consequences.

**Orchestration, not choreography (Design A).** `ordering` calls
`depot.CreateShoppingList(order_id)` synchronously the instant it creates an
`Order` from `BasketCheckedOut`, followed by one `AddItem` call per unit
(not per line-item-with-quantity — see Consequences on `AddItem`), storing
the returned `shopping_list_id` on its own `Order` row. `depot` gains no new
subscriber and no new upstream dependency; its only change is accepting
`order_id` on an already-planned-to-change method.

The deciding factor was minimizing rewiring of two already-shipped, already-
running services, given this system was built sequentially rather than
designed as a whole up front. Choreography is the more architecturally
"pure" event-driven shape, and was explicitly acknowledged as such — but it
would have required `depot` to gain a second live upstream dependency
(`ordering`) that ADL 0001's dependency graph never anticipated for it, a
new `Subscriber`/consumer/dead-letter path, and a full extra async round
trip before a shopping list exists, all to save avoiding one synchronous
call that only ever needs to fire once, immediately, as a direct
consequence of an action `ordering` itself just took.

## Alternatives considered

**Synchronous read-back (`GetBasket`/`GetShoppingList`) instead of enriching
events.** Rejected — this would have been a workaround for a payload gap
rather than a fix for it: enriching the event removes the need for the call
entirely, whereas read-back leaves `ordering`'s event processing dependent
on `baskets` being reachable at consumption time for data the event could
have carried itself. Also assessed as working against this project's
event-driven intent more directly than the orchestration call below, which
is triggering a real action, not compensating for missing data.

**Choreography (Design B).** See Decision — not rejected on correctness
grounds, genuinely the more "pure" shape, but rejected on cost/benefit given
this system's sequential build history. Worth reconsidering if a future
service needs the same create-on-behalf-of pattern more than once, at which
point the "second subscriber + dead-letter path" cost gets amortized across
more than one caller.

**`baskets` calling `depot.CreateShoppingList` directly, with `ordering`
reduced to a pure read-model.** Rejected — would require `baskets` to know
about `depot` and about the concept of an `Order`, neither of which is its
responsibility, and directly contradicts `docs/roadmap.md`'s own framing of
`ordering` as "the real event-driven analogue of `order-fulfillment`'s
in-process `PayOrder`/`Inventory` coordination" — i.e. coordination is
`ordering`'s job, not `baskets`'.

**Adding `quantity` to `depot`'s `AddItem`.** Considered as a natural
counterpart to the `order_id` change, rejected specifically to keep the
footprint on `depot` as small as possible: `AddItem` already adds one unit
and increments on repeat calls for the same product, which is exactly the
shape `ordering` needs if it calls it once per unit rather than once per
line-item-with-a-quantity. Slightly more chatty (N calls instead of one),
but zero signature changes to an already-live method.

## Consequences

- `ShoppingListCompleted`'s item-list gap, flagged as unresolved in
  `depot-CONTRACT.md`, turns out not to need resolving at all under Design
  A: `ordering` already possesses the order's contents (it supplied them to
  `depot` in step one), and only needs to know _which_ order is now ready —
  already answered by the event's existing aggregate ID
  (`shopping_list_id`), which `ordering` stores on its own `Order` row the
  moment `CreateShoppingList` returns. A contract gap two prior projects
  correctly left open, rather than guessing at, turns out to resolve itself
  once a real consumer's actual needs became concrete — exactly the
  deferral discipline ADL 0007 and this system generally have practiced
  throughout.
- The actual contract surface touched by this project is now confined to
  exactly two changes, both already anticipated by name in the relevant
  `CONTRACT.md` files before this ADL was written: `baskets`'
  `BasketCheckedOut` payload, and `depot`'s `CreateShoppingList` signature.
  Nothing else in `baskets`/`depot`/`stores` changes.
- `ordering` is the first service in this system to be both an event
  consumer and a synchronous ConnectRPC client of another live service.
  This introduces a new failure mode this system hasn't had to handle yet —
  a downstream synchronous command failing or timing out mid-orchestration,
  distinct from ADL 0007's publish-failure gap — to be resolved explicitly
  in Phase 4 rather than assumed away.
- `depot`'s own dependency graph (ADL 0001: depends only on `stores`) is
  unchanged by this project. If a future service needs the same
  create-on-behalf-of relationship with `depot` more than once, Design B's
  cost/benefit calculus should be re-run, not dismissed on this ADL's
  say-so alone — this decision was made for `ordering` specifically, under
  this system's specific sequential-build constraint, not as a general
  rule that orchestration always beats choreography.
