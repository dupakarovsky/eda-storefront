# Contract — `baskets`

**Status:** Live **Project folder:** `depot-baskets/baskets/` **Last updated:** 2026-09-12

Owns and is the source of truth for a customer's active shopping basket — its
contents (product/quantity pairs) and lifecycle status (open, checked out,
canceled) — from the moment it's started until checkout or cancellation.

## Published events

All events publish onto the `baskets` JetStream stream (subject
`baskets.basket.*`), at-least-once delivery, no ordering guarantee across
different aggregate IDs. Publish uses inline retry with backoff (five
attempts, ~10.5s worst-case budget, same schedule as `stores` per ADL 0007),
**and** attaches a per-call `Nats-Msg-Id` (the same ID reused across every
retry of one logical publish, never regenerated per attempt), relying on the
stream's `Duplicate Window` (2 minutes, JetStream's default, unconfigured by
either service) to collapse a buffered/late-arriving retry into the original
stored message rather than a second one (ADL 0015). A publish that exhausts
all five retries still leaves the aggregate saved in Postgres with no
corresponding event ever published — the same known, currently unresolved
gap ADL 0007 already names for `stores`.

### `BasketStarted`

- **Subject:** `baskets.basket.BasketStarted`
- **Published when:** a new basket is started via `StartBasket`
- **Payload:** `{}`

### `ItemAdded`

- **Subject:** `baskets.basket.ItemAdded`
- **Published when:** a product is added to a basket via `AddItem`
- **Payload:**

```json
{
    "product_id": "string",
    "quantity": "integer (running total after this add, not a delta)"
}
```

### `ItemRemoved`

- **Subject:** `baskets.basket.ItemRemoved`
- **Published when:** a product is removed (or decremented) via `RemoveItem`
- **Payload:**

```json
{ "product_id": "string" }
```

- Carries no quantity — a consumer cannot tell from this event alone whether
  the item was fully removed or merely decremented; only that a removal
  occurred against this product ID.

### `BasketCheckedOut`

- **Subject:** `baskets.basket.BasketCheckedOut`
- **Published when:** `Checkout` succeeds
- **Payload:** `{}`
- **Known limitation:** carries no item list or total. A downstream consumer
  needing the basket's contents at checkout time (e.g. `ordering`) cannot get
  them from this event alone as currently defined — flagged during Phase 3
  as a design decision to revisit once a real consumer's needs are known,
  not yet resolved.

### `BasketCanceled`

- **Subject:** `baskets.basket.BasketCanceled`
- **Published when:** `Cancel` succeeds
- **Payload:** `{}`

## Synchronous API surface

ConnectRPC, served at `/baskets.v1.BasketsService/<Method>`. All six methods
below are live and verified against real Postgres/NATS/`stores` — see
`baskets/basketspb/api.proto` for exact message shapes. Basket IDs are
server-generated (UUID) on creation, never client-supplied (ADL 0009).

| Method         | Request → Response                     | Notes                                                                        |
| -------------- | -------------------------------------- | ---------------------------------------------------------------------------- |
| `StartBasket`  | `{}` → `{basket}`                      | basket id is server-generated; no input fields at all                        |
| `AddItem`      | `{basket_id, product_id}` → `{basket}` | validates `product_id` exists in the local catalog cache before mutating     |
| `RemoveItem`   | `{basket_id, product_id}` → `{basket}` | decrements quantity, or removes the line entirely once quantity reaches zero |
| `Checkout`     | `{basket_id}` → `{basket}`             | errors if the basket has no items                                            |
| `CancelBasket` | `{basket_id}` → `{basket}`             | errors if already checked out or already canceled (two distinct error cases) |
| `GetBasket`    | `{basket_id}` → `{basket}`             | read-only, no repository write, no event published                           |

A caller invoking any of the above against an unknown basket ID, or an
`AddItem` call against a product ID absent from the catalog cache, receives
HTTP 404 (`CodeNotFound`). A domain-rule rejection (blank product ID, wrong
basket lifecycle state, empty-basket checkout) receives HTTP 400
(`CodeInvalidArgument`).

## Data this service owns

`baskets` is the source of truth for basket contents and lifecycle status
(`baskets`/`basket_items` tables). It is explicitly **not** the source of
truth for product or store data — `catalog_stores`/`catalog_products` are a
local, read-only, eventually-consistent projection of `stores`' own data,
populated entirely by consuming `stores`' published events. A consumer of
this contract must never treat `baskets`' catalog cache as authoritative for
product existence, pricing, or store participation — only `stores`' own
`CONTRACT.md` is authoritative for that data.

## Explicit non-goals of this contract

- No bulk/batch operations — every mutation is single-basket, single-item.
- No basket deletion — a basket can only be canceled (`CancelBasket`), never
  removed, mirroring `stores`' "no aggregate deletion" pattern for `Store`.
- No historical event replay beyond JetStream's own retention window.
- No price snapshotting at add-time — `ItemAdded`'s payload (and
  `basket_items`' own schema) records only `product_id`/`quantity`, never a
  price. A basket's items carry no price information of their own; any price
  a future consumer needs must come from `stores`' own data (directly or via
  a cache), not from `baskets`.
- No participation-status check on `AddItem` — a product from a store whose
  participation has since been disabled can still be added to a basket
  today, since `AddItem`'s catalog validation only checks product
  _existence_, not the owning store's `participating` flag. Named as a real,
  currently-accepted gap, not an oversight.
- Consuming `stores`' events relies on matching event-name strings written
  as local literals inside `catalog/project.go` (`"StoreCreated"`,
  `"ProductAdded"`, etc.) — there is no compiler-enforced link between these
  strings and `stores`' own event-name constants, since ADL 0004 prohibits
  importing `stores`' internal packages. A silent rename on `stores`' side
  would not fail a `baskets` build; it would silently stop matching and log
  a warning per unrecognized event. `CONTRACT.md` is the only synchronization
  mechanism for this today.
