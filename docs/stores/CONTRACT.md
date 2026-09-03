# Contract — `stores`

**Status:** Live
**Project folder:** `stores/`
**Last updated:** 2026-08-30

Owns and is the source of truth for store and product catalog data — store
identity/location/participation status, and each store's products
(name/description/SKU/price).

## Published events

All events publish onto the `stores` JetStream stream (subjects
`stores.store.*` / `stores.product.*`), at-least-once delivery, no ordering
guarantee across different aggregate IDs. Publish uses inline retry with
backoff (five attempts, ~10.5s worst-case budget) — see ADL 0007. A
publish that exhausts all retries leaves the aggregate saved in Postgres
with no corresponding event ever published; this is a known, currently
unresolved gap (ADL 0007), not something a consumer should assume can't
happen.

### `StoreCreated`

- **Subject:** `stores.store.StoreCreated`
- **Published when:** a new store is created via `CreateStore`
- **Payload:**

```json
{
    "name": "string",
    "location": "string"
}
```

- **Delivery guarantees:** at-least-once via JetStream

### `StoreRebranded`

- **Subject:** `stores.store.StoreRebranded`
- **Published when:** a store's name is changed via `RebrandStore`
- **Payload:**

```json
{ "name": "string" }
```

- **Delivery guarantees:** at-least-once via JetStream

### `StoreParticipationEnabled`

- **Subject:** `stores.store.StoreParticipationEnabled`
- **Published when:** `EnableStoreParticipation` succeeds
- **Payload:**

```json
{ "participating": true }
```

- **Delivery guarantees:** at-least-once via JetStream

### `StoreParticipationDisabled`

- **Subject:** `stores.store.StoreParticipationDisabled`
- **Published when:** `DisableStoreParticipation` succeeds
- **Payload:**

```json
{ "participating": false }
```

- **Delivery guarantees:** at-least-once via JetStream

### `ProductAdded`

- **Subject:** `stores.product.ProductAdded`
- **Published when:** a product is added to a store via `AddProduct`
- **Payload:**

```json
{
    "store_id": "string",
    "name": "string",
    "description": "string",
    "sku": "string",
    "price": "number"
}
```

- **Delivery guarantees:** at-least-once via JetStream

### `ProductRebranded`

- **Subject:** `stores.product.ProductRebranded`
- **Published when:** a product's name/description changes via
  `RebrandProduct`
- **Payload:**

```json
{ "name": "string", "description": "string" }
```

- **Delivery guarantees:** at-least-once via JetStream

### `ProductPriceIncreased`

- **Subject:** `stores.product.ProductPriceIncreased`
- **Published when:** `IncreaseProductPrice` succeeds
- **Payload:**

```json
{ "delta": "number (positive)" }
```

- **Delivery guarantees:** at-least-once via JetStream

### `ProductPriceDecreased`

- **Subject:** `stores.product.ProductPriceDecreased`
- **Published when:** `DecreaseProductPrice` succeeds
- **Payload:**

```json
{ "delta": "number (negative)" }
```

- **Delivery guarantees:** at-least-once via JetStream

### `ProductRemoved`

- **Subject:** `stores.product.ProductRemoved`
- **Published when:** a product is removed via `RemoveProduct`
- **Payload:**

```json
{}
```

- **Delivery guarantees:** at-least-once via JetStream

## Synchronous API surface

ConnectRPC, served at `/stores.v1.StoresService/<Method>`, supporting both
real gRPC and Connect's own HTTP+JSON protocol (the latter requires a
`Connect-Protocol-Version: 1` header). All eleven methods below are live and
verified — see `01-stores/storespb/api.proto` for exact message shapes.
IDs are server-generated (UUID) on creation, never client-supplied.

| Method                      | Request → Response                                        | Notes                                                                                                                                                                                                                                          |
| --------------------------- | --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CreateStore`               | `{name, location}` → `{store}`                            | store id is server-generated                                                                                                                                                                                                                   |
| `RebrandStore`              | `{store_id, name}` → `{store}`                            |                                                                                                                                                                                                                                                |
| `EnableStoreParticipation`  | `{store_id}` → `{store}`                                  | errors if already enabled                                                                                                                                                                                                                      |
| `DisableStoreParticipation` | `{store_id}` → `{store}`                                  | errors if already disabled                                                                                                                                                                                                                     |
| `AddProduct`                | `{store_id, name, description, sku, price}` → `{product}` | product id is server-generated                                                                                                                                                                                                                 |
| `RebrandProduct`            | `{product_id, name, description}` → `{product}`           |                                                                                                                                                                                                                                                |
| `IncreaseProductPrice`      | `{product_id, price}` → `{product}`                       | `price` is the absolute new price, not a delta; errors if not strictly greater                                                                                                                                                                 |
| `DecreaseProductPrice`      | `{product_id, price}` → `{product}`                       | `price` is the absolute new price, not a delta; errors if not strictly lower                                                                                                                                                                   |
| `RemoveProduct`             | `{product_id}` → `{}`                                     | hard delete, no domain-level rejection possible                                                                                                                                                                                                |
| `GetStore`                  | `{store_id}` → `{store}`                                  |                                                                                                                                                                                                                                                |
| `ListProducts`              | `{store_id}` → `{products}`                               | empty result is `[]` at the Go/application level; the field is entirely absent from the JSON response when empty, per protobuf JSON's default zero-value omission — not a bug, a client should treat a missing `products` key as an empty list |

A caller invoking any of the above against an unknown ID receives HTTP 404
(`CodeNotFound`); a domain-rule rejection (blank name, non-increase/decrease
price, already-enabled/disabled) receives HTTP 400 (`CodeInvalidArgument`).

## Data this service owns

`stores` is the source of truth for store identity/location/participation
status and for product existence, naming, SKU, and pricing. Consumers may
cache a read-only projection of this data but must never write it back —
all mutation happens through the API surface above or not at all.

## Explicit non-goals of this contract

- No bulk/batch create, update, or delete for either aggregate — every
  mutation is single-aggregate.
- No store deletion — a store can only be disabled from participation
  (`DisableStoreParticipation`), never removed. `RemoveProduct` is the one
  asymmetric exception: products can be hard-deleted, stores cannot.
- No historical event replay beyond JetStream's own retention window — this
  service does not separately archive or re-expose past events.
- No synchronous "does X exist" check beyond `GetStore`/`ListProducts`
  themselves — there is no lightweight existence-only endpoint.
- No pagination on `ListProducts` — it returns every product for a store in
  one response. Fine at this system's current scale; a future consumer with
  a store carrying a very large catalog would need this revisited.
- No guaranteed event delivery across a publish failure that exhausts all
  retries (ADL 0007) — a consumer cannot assume every state change in
  Postgres has a corresponding event that reached NATS.
- Read RPCs (`GetStore`/`ListProducts`) currently share the same
  ~12–13s worst-case request timeout as write RPCs, even though nothing
  about a read needs anywhere near that long (ADL 0007 addendum) — a slow
  Postgres would let a read hang far longer than ideal before failing.
