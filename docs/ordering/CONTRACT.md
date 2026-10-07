# Contract — `ordering`

**Status:** Live
**Project folder:** `ordering/`
**Last updated:** 2026-09-30

Owns and is the source of truth for the fan-in orchestration between a
checked-out basket and a completed shopping list — creating an `Order` from
`baskets`' checkout, synchronously driving `depot` to build and populate the
matching fulfillment list, and advancing the order to `readied` once that
list completes, or to `canceled` on request.

## Published events

All events publish onto the `ordering` JetStream stream (subject
`ordering.order.*`), at-least-once delivery, no ordering guarantee across
different aggregate IDs. Publish uses inline retry with backoff (five
attempts, ~10.5s worst-case budget, same schedule as every other service per
ADL 0007), and attaches a per-call `Nats-Msg-Id` (ADL 0015). An `Order`'s ID
is always identical to the `baskets` `Basket` ID that produced it — a
deliberate modeling commitment, not an incidental detail a consumer should
rely on changing.

### `OrderCreated`

- **Subject:** `ordering.order.OrderCreated`
- **Published when:** `baskets`' `BasketCheckedOut` is consumed and a new
  `Order` is constructed
- **Payload:**

```json
{
    "items": [
        {
            "product_id": "string",
            "store_id": "string",
            "quantity": "integer"
        }
    ]
}
```

### `OrderReadied`

- **Subject:** `ordering.order.OrderReadied`
- **Published when:** `depot`'s `ShoppingListCompleted` is consumed and
  correlates to this order's linked shopping list
- **Payload:** `{}`
- Carries no item data — a consumer needing the order's contents must
  correlate back to this same order's own `OrderCreated` event by aggregate
  ID, not expect this event to repeat them.

### `OrderCanceled`

- **Subject:** `ordering.order.OrderCanceled`
- **Published when:** `CancelOrder` succeeds
- **Payload:** `{}`
- **Side effect not visible in this event's payload:** if the order had a
  linked shopping list, `ordering` also synchronously calls `depot`'s
  `CancelShoppingList` as part of producing this event (see "Synchronous
  dependency on `depot`" below). A consumer cannot tell from this event alone
  whether that call succeeded, was skipped because the list had already
  reached a terminal state on its own, or — in a known, unverified race
  window — raced against `depot` completing the list first. See Known
  Limitations.

## Synchronous API surface

ConnectRPC, served at `/ordering.v1.OrderingService/<Method>`. Both methods
below are live and verified via a curl script against the full containerized
stack — see `ordering/orderingpb/ordering.proto` for exact message shapes.
Unlike `stores`/`depot`/`baskets`, `ordering` exposes no creation RPC —
`Order` creation is event-driven only, triggered exclusively by consuming
`baskets`' `BasketCheckedOut`, never by a direct synchronous caller.

| Method        | Request → Response       | Notes                                                                                                                                                                                              |
| ------------- | ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GetOrder`    | `{order_id}` → `{order}` | read-only, no repository write, no event published                                                                                                                                                 |
| `CancelOrder` | `{order_id}` → `{order}` | legal from `created`; errors if already `readied` or already `canceled` (two distinct error cases). If the order has a linked shopping list, also synchronously cancels it in `depot` — see below. |

A caller invoking either method against an unknown order ID receives HTTP 404
(`CodeNotFound`). A domain-rule rejection (already-readied,
already-canceled) receives HTTP 400 (`CodeInvalidArgument`).

## Synchronous dependency on `depot`

`ordering` is a live ConnectRPC client of `depot`, not just an event
consumer — this is the one asymmetry in this system's otherwise
event-driven dependency graph (ADL 0016's Design A). Two call paths:

- **Order creation** (`CreateOrder`'s internal orchestration): calls
  `depot.CreateShoppingList` then `depot.AddItem` once per unit, storing the
  returned shopping list ID on the `Order`. Idempotent under redelivery —
  `depot.CreateShoppingList` is itself idempotent keyed on `order_id`, and
  `orchestrateShoppingList` computes per-product deltas via
  `depot.GetShoppingList` before adding, so a retried orchestration never
  double-counts.
- **Order cancellation** (`CancelOrder`): calls `depot.CancelShoppingList`.
  A `CodeInvalidArgument` response is treated as benign (the shopping list
  already reached a terminal state — completed or canceled — on its own) and
  cancellation proceeds anyway; any other response (unreachable, `CodeInternal`,
  etc.) is treated as a real failure and returned to the caller, with the
  `Order` already canceled in `ordering`'s own database regardless.

A consumer of this contract relying on `ordering`'s event stream alone (not
calling `ordering`'s own API) never sees these `depot` calls directly — only
their consequence, if any, via `depot`'s own published events
(`ShoppingListCanceled`, etc.), which such a consumer would need its own
subscription to `depot`'s stream to observe.

## Data this service owns

`ordering` is the source of truth for `Order` lifecycle and contents
(`orders`/`order_items` tables) — status (`created`/`readied`/`canceled`),
items (`product_id`/`store_id`/`quantity`), and the linked `shopping_list_id`
once orchestration completes. It is explicitly **not** the source of truth
for product, store, or shopping-list contents — those remain owned by
`stores` and `depot` respectively, per their own `CONTRACT.md` files.

## Explicit non-goals and known limitations of this contract

- **No creation RPC.** `CreateOrder` is reachable only by consuming
  `baskets`' `BasketCheckedOut`; there is no synchronous way to construct an
  `Order` directly, unlike every other aggregate in this system.
- **The `depot`-completes-during-cancellation race is reasoned through but
  not verified under an actual race.** If `depot` completes a shopping list
  at the exact moment `ordering.CancelOrder` is mid-flight, `CancelShoppingList`
  correctly resolves to "benign, already terminal" and cancellation proceeds
  — but this specific timing window has only been validated by code review,
  not by forcing the race live.
- **The check-then-act race named in `CreateOrder`/`CompleteOrder` (Phases
  4/5) remains open.** Neither use case's initial, unlocked lookup
  (`FindByID`/`FindByShoppingListID`) is protected by a row lock before the
  subsequent locked mutation — a narrow window where two genuinely
  concurrent redeliveries of the same event could both pass the existence
  check before either takes the lock. `CancelOrder` does not have this gap,
  since it receives its order ID directly from a caller and locks
  immediately.
- **No automated test coverage for `GetOrder`/`CancelOrder`/`connectapi`, nor
  for the rollback-context fix (ADL 0018).** All of Phase 6's and Phase 7's
  new surface was verified live, end-to-end, against the full containerized
  stack, not via dedicated unit/integration tests. A future consumer should
  not assume test coverage here matches the depth `stores`/`depot`/`baskets`
  have for their own equivalent methods.
- **No bulk/batch operations, no order deletion, no historical event replay
  beyond JetStream's own retention window, no pagination** — the same
  non-goals every other service's `CONTRACT.md` already names for its own
  aggregate, holding identically here.
- **Read RPCs (`GetOrder`) share the same request timeout ceiling as write
  RPCs** — the same known, unresolved gap ADL 0007's addendum names for
  every other service, inherited unchanged via `foundation/config`'s
  defaults.
