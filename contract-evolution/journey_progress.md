<!-- contract-evolution/journey_progress.md -->

# 🟢 Contract Evolution — Project Plan

## Project Concept

Before `payments` can be built, the contracts between the live services need to
carry the data `payments` depends on. Today price, customer identity and order
totals are not threaded through the flow: `BasketCheckedOut` has no price or
customer, `Order` has no customer or total, and `OrderReadied` and
`OrderCanceled` carry `{}`. This project implements ADL 0020 across `stores`,
`baskets`, `depot` and `ordering`, so that `payments` can be built against real
contracts instead of stubs. Tranche 3 (payment identifiers, confirm-payment,
invoices) is deliberately not built here: it originates in `payments` and is
built inside that project.

## Concepts We'll Cover

- Evolving published event contracts additively (optional fields, tolerant
  consumers) without breaking running consumers
- Absolute-state versus delta projections, and why a delta projection can skip
  or corrupt state under redelivery and reordering
- The `last_applied_seq` guard and sentinel errors that must be the same value
  across package boundaries
- Money as `float64` with rounding at the boundary, validated at the domain
  edge, stored as `NUMERIC(10,2)`
- Safe schema migrations on a live database (pre-check query, `USING round()`,
  `CHECK` constraints, real down migrations)
- Freezing a price at checkout, inside the transaction that already resolves
  `store_id`
- Threading a new identifier through an aggregate, its events, its RPC and its
  tests (a breaking RPC change)
- Synchronous cross-service calls and failure classification by Connect error
  code (retry on `Unavailable`, `DeadlineExceeded`, `Internal`; permanent
  otherwise)
- A new terminal aggregate state (`rejected`) with its own event
- Proving a property by breaking the code and watching the test fail first

## Folder Structure

```
contract-evolution/
└── journey_progress.md        (no go.mod: this project owns no service)

Services modified (source lives in their own folders):
stores/                        domain, events, price use cases, mapError, proto
depot-baskets/baskets/         catalog projection, checkout, customer_id, migrations
depot-baskets/depot/           catalog projection, migrations
ordering/                      Item.Price, totals, CustomerID, customersclient,
                               rejected status, wiring
docs/                          system-state.md, roadmap.md, ADL updates, contracts
```

## Phase Plan

**Phase 0 — Housekeeping.** Restructure `system-state.md` so it can be updated
by appending, amend `roadmap.md` for the real build order (ADL 0019), add a
short ADL 0007 addendum for the current retry budget, and run
`ordering-http-connect-2.sh` against the live stack as the regression we have
not yet seen since the NATS reconnect change.

**Phase 1 — Tranche 1a: price in the catalog.** Fix the catalog sentinel-error
bug in `baskets` and `depot` with tests that fail first. Add the pre-check
query, the `stores` two-decimal rule, an absolute `price` on
`ProductPriceIncreased` and `ProductPriceDecreased`, absolute-price projections
guarded by `last_applied_seq` that tolerate old events without `price`, the
`000004` migrations, and a delta-reorder test.

**Phase 2 — Tranche 1b: price frozen at checkout.** `baskets` resolves price at
`Checkout` and publishes it on `BasketCheckedOut`. `ordering` gains
`Item.Price`, `order_items.price`, a rounded `GetTotal()`, `price` on
`OrderCreated` items and `total` on `OrderReadied`. Four contracts updated.

**Phase 3 — Tranche 2a: customer identity in `baskets`.** `customer_id` on
`StartBasket` (breaking), the domain, migration `000005`, `BasketStarted` and
`BasketCheckedOut`, and every affected test and script.

**Phase 4 — Tranche 2b: customer identity in `ordering`.** Settle the two open
questions in ADL 0020 first. Then `Order.CustomerID`, `customersclient`,
authorization with the retry classification, the `rejected` status and
`OrderRejected`, plus `Dockerfile`, `go.mod`, compose and config wiring.

**Phase 5 — Close-out.** Contracts, `system-state.md`, ADL 0020 status, and the
new ADLs this project produced.

## Phase 0 Summary — COMPLETE

**Status:** Complete. Documentation matches reality and the live stack is
verified healthy before any contract changes.

### What Was Built

- `docs/system-state.md` restructured: a narrow services table pointing at each
  `CONTRACT.md`, and infrastructure notes as short bullets per component, with
  the 2026-10-10 `customers` changes folded in. `edadb` is now documented as
  the administration database.
- `docs/roadmap.md` amended: an "Amendments to the original sequence" section,
  a narrow project table with Order and Ref columns (Ref keeps the original
  project numbers that ADLs refer to), per-project prose moved out of the
  table, and a corrected workspace tree.
- ADL 0007 addendum (stale "three retries" wording corrected; five local copies
  of the publish helper recorded) and ADL 0004 addendum (modifying projects
  attach full source of the services they change; `customerspb` as a bounded
  exception).
- Typo fixes: `07-customers-db.sh` (`cusotmersdb`), the `Makefile`
  (`cusotmers`, `.PHONY`), the compose mount path for the customers init
  script. The dead `foundation/smoke` recipe was removed.

### Issues Found & Resolved

- `customers` was down: `docker compose kill` is a manual stop, so
  `restart: unless-stopped` never applied, including after a daemon restart.
  Started manually. Crash recovery (process dying without Docker's involvement)
  remains untested.
- Fixing the compose mount path made `docker compose up` recreate the Postgres
  container, as ADL 0006 predicts for mount changes. Data survived in the
  `pg-data` volume and the four other services reconnected without a restart.
- Regression run of `ordering-http-connect-2.sh` after the NATS
  `MaxReconnects` default change: all steps and status codes as designed, no
  errors, dead-letters or exhausted retries in the logs.
- Found while reading, deferred to Phase 1: the catalog sentinel-error bug
  (`postgres.ErrPriceAdjustmentSkipped` is not `catalog.ErrPriceAdjustmentSkipped`,
  so the "already applied" branch cannot fire), in both `baskets` and `depot`.

### State at End of Phase 0

All five app services, Postgres and NATS are up and healthy. `baskets`
`000004` has not been applied to `basketsdb`; `depot` has no `000004`. There are
no tests for the `baskets` or `depot` catalog packages.

### Carried Into Phase 1

- Edit `baskets` `000004` in place (never applied): add `CHECK (price >= 0)`
  and a real down migration. Create `depot` `000004`.
- Write catalog tests before fixing the sentinel bug, and watch them fail first.
- The Tranche 2 open questions in ADL 0020 are settled at the start of Phase 4,
  with `ordering`'s source in hand.
- `stores`' `toProtoProduct` omits `Description` (visible in the regression
  output: `AddProduct` was sent a description and the response has none).
  Fix it when Phase 1 touches `stores`.
