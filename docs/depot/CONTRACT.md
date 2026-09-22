# Contract — `depot`

**Status:** Live
**Project folder:** `depot-baskets/depot/`
**Last updated:** 2026-09-16

Owns and is the source of truth for shopping list fulfillment — a manifest of
items to be gathered, grouped by the store each item belongs to, from
creation through completion or cancellation.

## Published events

All events publish onto the `depot` JetStream stream (subject
`depot.shopping_list.*`), at-least-once delivery, no ordering guarantee
across different aggregate IDs. Publish uses inline retry with backoff (five
attempts, ~10.5s worst-case budget, same schedule as `stores`/`baskets` per
ADL 0007), and attaches a per-call `Nats-Msg-Id` (the same ID reused across
every retry of one logical publish), relying on the stream's `Duplicate
Window` (2 minutes, JetStream's default, unconfigured) to collapse a
buffered/late-arriving retry into the original stored message rather than a
second one (ADL 0015), applied from this service's first publish call rather
than retrofitted. A publish that exhausts all five retries still leaves the
aggregate saved in Postgres with no corresponding event ever published — the
same known, currently unresolved gap ADL 0007 already names for `stores` and
`baskets`.

### `ShoppingListCreated`

- **Subject:** `depot.shopping_list.ShoppingListCreated`
- **Published when:** a new shopping list is created via `CreateShoppingList`
- **Payload:**

```json
{ "order_id": "string" }
```

### `ItemAdded`

- **Subject:** `depot.shopping_list.ItemAdded`
- **Published when:** a product is added to a list via `AddItem`
- **Payload:**

```json
{
    "store_id": "string",
    "product_id": "string",
    "quantity": "integer (running total after this add, not a delta)"
}
```

### `ItemRemoved`

- **Subject:** `depot.shopping_list.ItemRemoved`
- **Published when:** a product is removed (or decremented) via `RemoveItem`
- **Payload:**

```json
{ "store_id": "string", "product_id": "string" }
```

- Carries no quantity — a consumer cannot tell from this event alone whether
  the item was fully removed or merely decremented, only that a removal
  occurred against this product ID under this store.

### `ShoppingListInitiated`

- **Subject:** `depot.shopping_list.ShoppingListInitiated`
- **Published when:** `InitiateShoppingList` succeeds
- **Payload:** `{}`
- Marks the point at which the list's contents are frozen — no further
  `AddItem`/`RemoveItem` calls are accepted against this list once this
  event fires.

### `ShoppingListCompleted`

- **Subject:** `depot.shopping_list.ShoppingListCompleted`
- **Published when:** `CompleteShoppingList` succeeds
- **Payload:** `{}`
- **Known limitation:** carries no item list or stop breakdown. A downstream
  consumer needing the completed list's contents (e.g. `ordering`, per the
  roadmap's dependency graph) cannot get them from this event alone as
  currently defined — the same open question `baskets_CONTRACT.md` already
  flags for `BasketCheckedOut`, unresolved here for the identical reason
  (no real consumer exists yet to make the requirement concrete).

### `ShoppingListCanceled`

- **Subject:** `depot.shopping_list.ShoppingListCanceled`
- **Published when:** `CancelShoppingList` succeeds
- **Payload:** `{}`

## Synchronous API surface

ConnectRPC, served at `/depot.v1.DepotService/<Method>`. All seven methods
below are live and verified — both via unit/integration tests against real
Postgres/NATS and via a live curl script against the full containerized
stack — see `depot/depotpb/api.proto` for exact message shapes. Shopping
list IDs are server-generated (UUID) on creation, never client-supplied
(ADL 0009).

| Method                 | Request → Response                                        | Notes                                                                                                                                                             |
| ---------------------- | --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CreateShoppingList`   | `{order_id}` → `{shoppingList}`                           | shopping list id is server-generated; `order_id` is required                                                                                                      |
| `AddItem`              | `{shoppingListId, storeId, productId}` → `{shoppingList}` | validates both `storeId` and `productId` exist in the local catalog cache before mutating; only legal while `status` is `pending`                                 |
| `RemoveItem`           | `{shoppingListId, storeId, productId}` → `{shoppingList}` | decrements quantity, or removes the item entirely once quantity reaches zero; deletes the stop entirely once its last item is removed; only legal while `pending` |
| `InitiateShoppingList` | `{shoppingListId}` → `{shoppingList}`                     | moves `pending` → `active`; freezes `AddItem`/`RemoveItem` from this point forward                                                                                |
| `CompleteShoppingList` | `{shoppingListId}` → `{shoppingList}`                     | moves `active` → `completed`; errors if the list has no stops                                                                                                     |
| `CancelShoppingList`   | `{shoppingListId}` → `{shoppingList}`                     | legal from `pending` or `active`; errors if already completed or already canceled (two distinct error cases)                                                      |
| `GetShoppingList`      | `{shoppingListId}` → `{shoppingList}`                     | read-only, no repository write, no event published                                                                                                                |

A caller invoking any of the above against an unknown shopping list ID, an
`AddItem` call against a store ID absent from the catalog cache, or an
`AddItem` call against a product ID absent from the catalog cache all
receive HTTP 404 (`CodeNotFound`) — note store and product validation are
two separate checks, store first, so a request invalid on both counts
always reports the store failure. A domain-rule rejection (blank
store/product ID, wrong lifecycle state, empty-list completion,
product-not-in-stop) receives HTTP 400 (`CodeInvalidArgument`).

## Data this service owns

`depot` is the source of truth for shopping list contents (grouped by store
into `stops`) and lifecycle status. It is explicitly **not** the source of
truth for product or store data — `catalog_stores`/`catalog_products` are a
local, read-only, eventually-consistent projection of `stores`' own data,
populated entirely by consuming `stores`' published events, the same
pattern `baskets` uses. A consumer of this contract must never treat
`depot`'s catalog cache as authoritative for product existence, pricing, or
store participation — only `stores`' own `CONTRACT.md` is authoritative for
that data.

## Explicit non-goals of this contract

- **`CreateShoppingList` now requires `order_id` (ADL 0016).** This was a
  breaking change to an already-live method, applied when `ordering`
  (Project 3) became the first real caller — existing rows created before
  this change carry `order_id = ''` (the column's `DEFAULT ''`), not a
  real order reference. `order_id` is immutable once set: `Save` never
  updates it on subsequent calls for the same shopping list.
- **No verified proof of idempotent-publish behavior under a real NATS
  outage**, unlike `baskets`' `TestStartBasket_Execute_SucceedsAfterNATSRestart`.
  `depot`'s publish path (`publishWithRetry`/`publishDomainEvent`) is
  structurally identical code to `baskets`', so this was a deliberate,
  reasoned omission rather than an oversight — but it means this specific
  service has not itself been proven correct under that failure mode, only
  inferred to be correct by code identity.
- **No bots, no bot assignment, no `AssignedBotID`, no routing/pathfinding
  of any kind.** `Stops` groups items by store for manifest/checklist
  purposes only; nothing about the choice or order in which stops are
  visited is modeled or enforced by this service.
- **No verified sequencing relationship with `ordering`.** This project has
  assumed a completed shopping list is a precondition for order creation,
  not the reverse, based on the roadmap's description of `ordering`'s
  fan-in dependency on both `baskets` and `depot`. This has not been
  checked against any actual `ordering` design, since that project does not
  exist yet.
- **No bulk/batch operations, no shopping list deletion, no historical event
  replay beyond JetStream's own retention window, no pagination** — the
  same non-goals `stores_CONTRACT.md`/`baskets_CONTRACT.md` already name for
  their own aggregates, holding identically here.
- **Read RPCs share the same request timeout ceiling as write RPCs**, the
  same known, unresolved gap ADL 0007's addendum names for `stores` and
  inherited unchanged by every later service via `foundation/config`'s
  defaults.
- **`Stops` array ordering is not stable across calls.** `toProtoShoppingList`
  converts the domain's `map[string]*Stop` into a wire-format slice; Go's
  randomized map iteration means two `GetShoppingList` calls for the same
  list can return `stops` in different orders. A consumer must not depend
  on stop order being consistent between calls.
