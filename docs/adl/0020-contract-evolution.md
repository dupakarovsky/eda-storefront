# 20. Contract evolution: price, customer identity, and totals

Date: 2026-10-10

## Status

Accepted. Design only; no tranche is implemented yet.

## Context

A comparison against the book's `Chapter12/` showed that the book threads
customer, payment, and price through the whole flow. Ours threads none of them:
`BasketCheckedOut` carries no price or customer; `Order` has no customer,
payment, or total; `OrderReadied` and `OrderCanceled` carry `{}`. `payments`
cannot be built against today's contracts: the book creates an invoice from
`OrderReadied`'s `total` and `payment_id`.

Not everything can be done now. The payment identifier originates in `payments`
(its `AuthorizePayment` returns it), and requiring it earlier would mean
stubbing a service that does not exist. The work is therefore split.

## Decision

**Tranche 1: price and money (no new service dependencies).**

1. `float64` stays as the money type. Order totals are rounded to cents at the
   order boundary. Revisit if `payments` needs exact arithmetic.
2. `stores` rejects prices with more than two decimal places at the domain
   boundary, with a new sentinel mapped to `InvalidArgument`. The check must not
   be a naive float equality.
3. `ProductPriceIncreased` and `ProductPriceDecreased` gain an absolute `price`
   field (additive; `delta` stays). The `baskets` and `depot` catalog
   projections set the absolute price, guarded by the existing
   `last_applied_seq` check, instead of adding deltas. A delta projection can
   permanently skip a redelivered older event after a newer one applied, and
   float noise (`0.3 - 0.1 = 0.19999999999999998`) can accumulate in an
   unconstrained `NUMERIC`. Events already in the stream carry no `price`;
   the payload field must be optional and fall back to the delta path.
4. `catalog_products.price` becomes `NUMERIC(10,2)` in `baskets` and `depot`
   (migration `000004`, with `CHECK (price >= 0)` mirroring `stores`). Run a
   pre-check first: `SELECT id, price FROM catalog_products WHERE price <>
round(price, 2);`. The cast rounds drifted rows.
5. Price is frozen at checkout, not at add-time. `Checkout` resolves price from
   `baskets`' catalog cache in the same transaction that already resolves
   `store_id`. `BasketCheckedOut` items gain `price`. This partly reverses
   `baskets-CONTRACT.md`'s "no price snapshotting" non-goal: a basket's items
   still carry no price until checkout.
6. `ordering`: `Item` gains `Price`; `order_items.price` is `NUMERIC(10,2)`;
   `Order.GetTotal()` rounds to cents; `OrderCreated` items gain `price`;
   `OrderReadied` gains `total`.
7. Legacy rows default to price 0. The affected data is development data.

**Tranche 2: customer identity (requires `customers`, live).**

1. `StartBasket` requires `customer_id` (breaking RPC change). `baskets` treats
   it as an opaque, non-blank string and does not validate it against
   `customers`: that would add a dependency ADL 0001 does not have.
   `BasketStarted` and `BasketCheckedOut` carry `customer_id`.
2. `Order` gains `CustomerID`, which `NewOrder` requires. `OrderCreated`,
   `OrderReadied`, and `OrderCanceled` carry `customer_id`.
3. `ordering` authorizes the customer by calling `customers.AuthorizeCustomer`
   synchronously, the same pattern as its `depot` calls (ADL 0016).
4. Failure classification, applied by `ordering`: retry only on `Unavailable`,
   `DeadlineExceeded`, and `Internal`; treat every other code as permanent. A
   permanent failure (a disabled customer is `PermissionDenied`, an unknown one
   `NotFound`) rejects the order and acknowledges the message. An allowlist of
   permanent codes is rejected as the design: it would retry a request that can
   never succeed five times before dead-lettering it.
5. A new `rejected` order status and an `OrderRejected` event.
6. Legacy rows get `customer_id = ''`. A legacy basket checked out after this
   change fails `NewOrder` and dead-letters. Acceptable for development data.

**Tranche 3: payment** (`payment_id` at checkout and on the order, the
confirm-payment step, the invoice flow) is built inside the `payments` project.

## Alternatives considered

**Freeze price at add-time, as the book does.** Fairer to the customer, but
needs a `basket_items.price` column and a price on `ItemAdded`. Rejected for the
smaller footprint.

**Integer minor units for money.** Correct, but touches `stores`, the protos,
and about forty test call sites. Deferred with the tripwire above.

**Cap the catalog `NUMERIC` only, without absolute prices.** Rounding hides
float noise but not a skipped or reordered delta: it would produce a tidy wrong
price. Both changes are needed.

## Consequences

- Four live services change: `stores`, `baskets`, `depot`, `ordering`, plus
  their contracts, migrations, tests, and curl scripts. Tranche 1 is additive
  and safe in any rollout order if consumers treat new fields as optional.
  Tranche 2 is breaking (`StartBasket`).
- `ordering` becomes a real code dependency on `customers`: it imports
  `customerspb`, so its Dockerfile must copy `customers/`, its `go.mod` needs a
  `replace`, `docker-compose.yaml` adds `customers` to its `depends_on`, and the
  shared config gains a customers URL alongside `DEPOT_URL`.
- Open, to be settled at the start of Tranche 2: whether `ordering` authorizes
  before saving the order or after (create, then reject), and what an
  idempotent redelivery of a rejected order does.
- The outbox decision (ADL 0007) is separate and happens before `payments`.
