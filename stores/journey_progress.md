# 🟢 Stores Service — Project Plan

## Project Concept

The first live domain service in the Event-Driven Storefront system, and the
root of the whole dependency graph (ADL 0001). Owns two aggregates — `Store`
(name, location, participation flag) and `Product` (a store's catalog item:
name, description, SKU, price) — and is the single source of truth for both.
Publishes domain events onto NATS JetStream for every downstream consumer
(`depot`, `baskets`, later `search`) to react to, and exposes a synchronous API
via ConnectRPC for the storefront and any other synchronous caller.

Adapted from `Chapter12/stores` in the source book, with two deliberate
departures: current-state persistence instead of event sourcing (ADL 0002), and
ConnectRPC instead of the book's real-gRPC-server-plus-grpc-gateway pairing —
chosen because foundation's hand-rolled `Router` already commits us to serving
HTTP directly, and Connect's generated handlers are plain `http.Handler`s that
mount onto it with no second port or proxy hop.

This is the first project since `foundation` was split out (ADL 0005) to build
on top of it rather than build it.

---

## Concepts We'll Cover

**Domain modeling** Aggregate design with current-state persistence (no event
sourcing) — domain events still fired as an integration side effect, not as the
aggregate's own source of truth

**Persistence** Postgres schema/migration via `golang-migrate`, repository
pattern with `Reconstitute` rehydration constructors and current-state upserts,
per-aggregate concurrency control (row lock vs optimistic version-column CAS,
decided by write pattern rather than defaulted system-wide)

**Messaging** Publishing real domain events over NATS JetStream using
`foundation/nats`'s `Publisher`/`EnsureStream`/`Subject` helpers; subject naming
scheme for the `stores` stream

**API layer** Protobuf service definition, `buf` codegen, and wiring ConnectRPC
handlers as `http.Handler`s onto foundation's existing `Router` (command + query
methods)

**System discipline** Composition root pattern (`module.go`), extending the
shared `docker-compose.yml`/`Makefile`, and producing this project's
`CONTRACT.md` as a real deliverable (ADL 0004)

---

## Folder Structure

```txt
01-stores/
├── go.mod
├── journey_progress.md
├── CONTRACT.md
├── module.go                    # composition root
├── cmd/
│   └── service/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── store.go              # Store aggregate + behavior
│   │   ├── product.go            # Product aggregate + behavior
│   │   ├── events.go             # domain event payload types
│   │   ├── store_repository.go   # repository port/interface
│   │   └── product_repository.go
│   ├── postgres/
│   │   ├── store_repository.go   # Postgres adapter
│   │   └── product_repository.go
│   ├── connect/
│   │   └── server.go             # ConnectRPC handler implementation
│   └── events/
│       └── publisher.go          # domain event → NATS subject/payload mapping
├── storespb/                     # buf-generated: api.proto → Go + Connect code
│   └── api.proto
└── migrations/
    └── 0001_init.sql
```

---

## Phase Plan

**Phase 1 — Domain Model** Build `Store` and `Product` as current-state
aggregates: fields, constructors, behavior methods (`Rebrand`,
`EnableParticipation`/ `DisableParticipation`, `IncreasePrice`/`DecreasePrice`,
`Remove`, etc.), validation, and the event payload types they raise. No
persistence, no API yet — testable in isolation with a throwaway script.

**Phase 2 — Postgres Persistence** Schema + migration for `stores` and
`products` tables, repository interfaces, and Postgres adapters with
`Reconstitute` rehydration and current-state upserts. This is where row-lock vs
optimistic-version concurrency gets decided per aggregate.

**Phase 3 — Event Publishing** Wire aggregate actions to real NATS JetStream
publishes using `foundation/nats`, including defining the `stores` stream and
subject scheme. By the end, a throwaway script can create a store and we can
watch the event land on NATS for real.

**Phase 4 — ConnectRPC API** Define `storespb/api.proto`, run `buf` codegen,
implement the Connect handlers, and mount them on foundation's `Router`. Covers
both commands (create/rebrand/add product/etc.) and queries (get store, get
catalog, etc.).

**Phase 5 — Wiring, Compose, and Verification** `module.go` composition root,
`main.go`, extending `docker-compose.yml`/ `Makefile` with the `stores` service
and its own Postgres database, then real `curl`/integration verification against
live containers.

**Phase 6 — Contract and Close** Finalize `CONTRACT.md` with real event/API
shapes (superseding the `system-state.md` placeholders), update
`system-state.md`, and write the closing Phase summary.

---

## Phase 1 — Domain Model (COMPLETED)

**What was built:** `Store` and `Product` aggregates in `internal/domain/`, as
current-state structs (ADL 0002) with behavior methods returning
`(Event, error)` directly — no internal event accumulation, no event-sourced
rehydration.

- `event.go` — the `Event` interface (`AggregateID() string`,
  `EventName() string`, `OccurredAt() time.Time`) plus an unexported `eventBase`
  struct providing the two bookkeeping fields, embedded by every concrete event
  type to avoid repeating them.
- `events.go` — every concrete event type: `StoreCreated`,
  `StoreParticipationEnabled`, `StoreParticipationDisabled`, `StoreRebranded`,
  `ProductAdded`, `ProductRebranded`, `ProductPriceIncreased`,
  `ProductPriceDecreased`, `ProductRemoved`. Each event that could plausibly
  represent more than one outcome (participation toggled, price changed) got
  split into two distinct concrete types instead of one type with a variable
  event-name field, to keep every struct either fully exported (payload) or
  fully internal (bookkeeping), never mixed.
- `store.go` — `Store` struct + `NewStore`, `EnableParticipation`,
  `DisableParticipation`, `Rebrand`.
- `product.go` — `Product` struct + `NewProduct`, `Rebrand`, `IncreasePrice`,
  `DecreasePrice`, `Remove`.
- `store_test.go`, `product_test.go` — full behavior + error-path coverage for
  every method, following `order-fulfillment`/`court-reservation`'s testing
  convention (shared `newTestX` helper, `t.Run` subtests per case), adapted to
  this project's direct-return design rather than a `recordEvent`/`PullEvents`
  pattern.

**Issues found and how they were diagnosed:** `IncreasePrice`/`DecreasePrice`
originally allowed a target price equal to the current price to succeed (only
`<`/`>` were checked, not `<=`/`>=`) — caught by a direct question before
writing tests, not by a failing test. Traced to the book itself having the same
gap. Fixed by switching to `<=`/`>=`, then covered with explicit "equal price
fails" subtests for both methods to lock the behavior in.

**Key concepts reinforced:**

- Current-state aggregates still raise integration events as a side effect,
  without those events being the source of truth for state (ADL 0002)
- Keeping struct field visibility uninformed by convenience — every field either
  fully exported or fully internal, never mixed on principle, even when it costs
  a couple more types
- Testing error paths for _non_-mutation, not just for the right error type —
  several tests assert state is unchanged after a rejected operation, not only
  that an error was returned

**Deviations from the book, and why:**

- `Rebrand` (both `Store` and `Product`) validates a blank name; the book's
  didn't.
- `IncreasePrice`/`DecreasePrice` reject a negative target price outright, and
  reject an _equal_ price as neither an increase nor a decrease (`<=`/`>=`
  rather than `<`/`>`).
- `Product.Price` is set directly to the new absolute value rather than
  recomputed via `p.Price + delta` — the book's delta-reapplication only exists
  because of event-sourced rehydration, which this system doesn't have.
- Event types are one-name-per-concrete-type (`court-reservation` convention),
  not the book's single generic `ddd.Event` wrapper around any payload.

**State at end of Phase 1:** `internal/domain/` package complete and fully
tested for both aggregates. No persistence, no event publishing, no API layer
yet — those are Phases 2–4.

**Carried into Phase 2:** `Store` and `Product` as-is, to be wrapped in Postgres
repositories with `Reconstitute` rehydration and current-state upserts.
Concurrency control strategy (row lock vs optimistic version column) still to be
decided per aggregate based on write pattern.

---

## Phase 2 — Postgres Persistence (COMPLETED)

**What was built:** Role-based database access, a per-service database, schema
migrations, repository interfaces, and Postgres adapters for both aggregates —
with row-level locking wired into both, and a reusable credential-safe
smoke-test pattern established for the rest of the project.

- **Role provisioning** (`postgres/init/02-roles.sh`) — two new roles, `eda_dev`
  (schema owner, `CREATE` on `public`, used for migrations) and `eda_user`
  (application runtime, DML only via `ALTER DEFAULT PRIVILEGES`, no DDL). This
  closed a gap carried over unnoticed from the `foundation` project, where the
  smoke test and `.env` both connected as the Postgres superuser (`edasuper`) —
  acceptable for a connectivity check, not for real application traffic.
- **Per-service database** (`postgres/init/03-stores-db.sh`) — `storesdb`, owned
  by `eda_dev`, with its own copy of the `eda_user` default-privilege grants
  (these are scoped per-database, not global, so `edadb`'s grants don't extend
  to a new database). Confirms `system-state.md`'s stated intent of one database
  per service rather than a shared `edadb` going forward.
- **Schema migrations**
  (`01-stores/migrations/0001_create_stores_and_products.up/down.sql`, via
  `golang-migrate`) — `stores` and `products` tables. `id`/`store_id` as
  `TEXT PRIMARY KEY`/`TEXT` (matching the domain's string IDs directly, no UUID
  conversion layer), `price` as `NUMERIC(10,2)` with a `CHECK (price >= 0)`
  constraint, `products.store_id` as a `REFERENCES stores(id) ON DELETE CASCADE`
  foreign key, and `created_at`/`updated_at TIMESTAMPTZ` audit columns on both.
  No `version` column — deferred until/unless optimistic locking is actually
  chosen for some future aggregate.
- **Repository interfaces** (`internal/domain/store_repository.go`,
  `product_repository.go`) — `Save`/`FindByID` per aggregate, with dedicated
  `ErrStoreNotFound`/`ErrProductNotFound` sentinels. Deliberately kept to this
  shape rather than expanded to broader CRUD or split into CQRS read/write
  repositories (see Deviations).
- **`Reconstitute` constructors** (`internal/domain/store.go`, `product.go`) —
  skip validation and emit no domain event, rebuilding an aggregate already
  known-valid from a database row rather than constructing a new one.
- **Postgres adapters** (`internal/postgres/store_repository.go`,
  `product_repository.go`) — `Save` as a genuine
  `INSERT ... ON CONFLICT DO UPDATE` upsert (one method covers both creation and
  update, matching the interface), `FindByID` issuing `SELECT ... FOR UPDATE`. A
  shared `Executor` interface (`Exec`/`QueryRow`) lets the same adapter code
  accept either a bare `*pgxpool.Pool` or a `pgx.Tx` — the lock is real but
  dormant until Phase 4 wires an actual transaction spanning load→mutate→save.
- **`cmd/repo-smoketest/`** — a kept (gitignored, reusable) throwaway that
  exercises both repositories end-to-end against live Postgres, using
  `foundation/config.Load()` rather than a hardcoded connection string.
- **`.gitignore`** — added `**/cmd/*smoketest*/`, workspace-wide, so any future
  throwaway smoke test is unreachable by `git add` by construction rather than
  by discipline.
- **`hardening.conf`** — added `timezone = 'UTC'` as a stated decision rather
  than an inherited image default.

**Issues found and how they were diagnosed:** `03-stores-db.sh`'s
`ALTER DEFAULT PRIVILEGES` statements logged as successful during container init
(`docker compose logs postgres` showed both commands completing without error),
but `eda_user` still got `permission denied` on a table `eda_dev` created
afterward. `\ddp` inside `storesdb` showed zero registered default-privilege
rules, despite the identical statement pattern being verifiably registered in
`edadb`. Bypassing psql's `\ddp` meta-command and querying `pg_default_acl`
directly confirmed the rows genuinely never landed — not a display artifact.
Root cause in the script itself was never conclusively identified (the file was
replaced rather than diffed against the broken version), but recreating the
volume with a corrected script resolved it, reverified this time against the raw
catalog table rather than trusting the init log alone.

**Key concepts reinforced:**

- `TIMESTAMPTZ` always stores an absolute UTC instant internally; what looked
  like a wrong timestamp (`+00` in the container vs. `-03` on a local Postgres
  instance) was actually two sessions with different `TimeZone` GUCs rendering
  the same instant differently — not a data bug.
- "The log shows no errors" is not proof a script did what it was supposed to;
  checking actual catalog state (`pg_default_acl`) caught a failure the init
  log's successful command echoes did not.
- `ALTER DEFAULT PRIVILEGES` is scoped per-database, not global or
  per-role-across-databases — a new database needs its own copy of the grants
  even when the granting role (`eda_dev`) already exists elsewhere.
- Mount-path/config changes under `docker-entrypoint-initdb.d` or `include_dir`
  require full container recreation (`docker compose down -v && up -d`), not a
  restart or reload — consistent with ADL 0006's existing finding, now hit a
  second time for a different file.

**Deviations from the book, and why:**

- The book's `StoreRepository`/`ProductRepository` are also `Load`/`Save`-only,
  but that shallowness is load-bearing there — event-sourced write-side
  repositories, backed by a separate `CatalogRepository`/`MallRepository`
  read-model pair returning flat DTOs from denormalized projections, since an
  event stream can't itself answer read queries. We don't have that constraint:
  ADL 0002's current-state tables are directly queryable, so `Load`/`Save`
  (renamed `FindByID`/`Save` here) is a deliberate choice for the write path,
  not a structural necessity — a read/query surface remains genuinely
  unaddressed and is deferred to whenever Phase 4's API layer defines what
  queries are actually needed, rather than spec'd speculatively now.
- Both aggregates use row-level locking (`SELECT ... FOR UPDATE`) rather than
  optimistic version-column CAS, decided per ADL 0002's per-aggregate mandate —
  `Store`'s writes are low-contention (single owner), `Product`'s include a
  plausible concurrent-edit case (price changes), but row-locking was judged
  adequate for both rather than splitting strategies without a concrete reason
  to.
- Went further than the book on privilege separation: the book's persistence
  layer isn't shown connecting as anything other than a single configured user;
  this project explicitly split schema-owner (`eda_dev`) from runtime
  (`eda_user`) roles, motivated by ADL 0006's own hardening posture rather than
  anything in the source material.

**State at end of Phase 2:** `storesdb` live with `stores`/`products` tables,
empty (scratch rows cleared). Both repositories and adapters built, tested
against real containers, row-locking wired but dormant. `01-stores/.env` points
at `eda_user`; migrations run via `eda_dev` through the `Makefile`'s
`migrate-up`/`migrate-down` targets. No application/command layer, no event
publishing, no API yet — those are Phases 3–4.

**Carried into Phase 3:** `StoreRepository`/`ProductRepository` and their
Postgres adapters as-is, ready to be called from real event-publishing code.
`foundation/nats`'s `Publisher`/`EnsureStream`/`Subject` helpers (already proven
in `foundation-smoketest`) still unused by `01-stores` — Phase 3 is where a
store/product action actually reaches NATS for the first time in this service.

## Phase 3 — Event Publishing (COMPLETED)

**What was built:**

- `internal/events/stream.go` — the `stores` stream definition, registered via
  `EnsureStream` in `module.go`'s boot sequence, using
  `stores.store.*`/`stores.product.*` as its two subjects (Option A from the
  earlier subject-scheme discussion).
- `domain/events.go` — corrected event-name constants (a typo, an inconsistent
  suffix, and a redundant prefix, all fixed before they became a live wire
  contract).
- `internal/events/envelope.go` and `payloads.go` — the `Envelope` wire format
  (`aggregate_id`, `event_name`, `occurred_at`, `data`) plus one dedicated,
  JSON-tagged payload type per domain event, converted via an exhaustive
  `toPayload` switch with an error-returning default case, so no future event
  type can silently reach the wire unmapped.
- `internal/events/subject.go` — `SubjectFor`, resolving a domain event to its
  NATS subject via the same exhaustive-switch discipline.
- `internal/application/create_store.go` — the first real use case,
  `CreateStore.Execute`: construct → save → envelope → publish, wired into
  `module.go` as `mod.CreateStoreUC`.
- `internal/application/publish_retry.go` — `publishWithRetry`, five attempts
  with exponential backoff (200/400/800/1600ms) and a per-attempt 1.5s timeout,
  giving a measured ~10.5-second worst-case budget.
- `cmd/createstore-smoketest/main.go` — the real, full-boot-path smoke test
  proving the happy path against live Postgres and NATS.
- `internal/application/create_store_test.go` — Docker-orchestrating test
  helpers (`stopNATSContainer`, `restartNATSContainerAfter`, `loadRealEnv`) plus
  `TestCreateStore_Execute_SucceedsAfterNATSRestart`, run twice against
  genuinely different outage lengths to prove both the success and
  exhausted-retry paths for real.
- `docs/adl/0007-inline-retry-transactional-outbox-deferred.md` — recording the
  retry-vs-outbox decision, including the empirically-measured retry budget.

**Key concepts exercised:**

- Marshaling boundaries: why embedding an interface's underlying struct directly
  leaks unexported-field gaps and Go-native casing, and why a dedicated
  wire-contract type (not the domain type) is the right fix — tying back to ADL
  0004's contract-boundary philosophy a layer deeper than originally scoped.
- Exhaustive type switches with an error-returning default as a structural
  guardrail against silent omission, applied twice (`toPayload`, `aggregateFor`)
  rather than once.
- The real distinction between "the NATS client's own reconnect buffering" and
  "our own retry logic" — and why proving the latter requires deliberately
  outlasting the former, not just watching a single test pass.
- `context.WithTimeout` as a way to bound worst-case latency per attempt, and
  why skipping it let an ambiguous connection state block for several times
  longer than the deliberate backoff alone would suggest.
- Why `t.Errorf`, not `t.Fatalf`, is required from a background goroutine in a
  Go test — and the coordination cost (a padded sleep, in this case) of not yet
  having sunk to a `sync.WaitGroup` or completion channel.

**Deviations from the original plan, and why:**

- Step 1 grew substantially beyond "define the stream" — the composition root
  (`module.go`, `cmd/service/main.go`, `health.go`) didn't exist yet, so it was
  built here rather than assumed, effectively absorbing part of what the
  original roadmap implied Phase 4 would do.
- Failure-mode handling (Step 5) landed on inline retry-with-backoff rather than
  a transactional outbox, deliberately deferred per ADL 0007 — the
  correct-but-heavier answer, left as a concrete open item to revisit once
  `depot`/`baskets` exist and the gap becomes real rather than hypothetical.
- Verification leaned on real `docker compose stop`/`start` orchestration from
  within Go tests, rather than mocking NATS — slower and more elaborate than a
  unit test, but it's what caught the context-timeout gap a mock never would
  have surfaced.

**State at end of Phase 3:** `01-stores` publishes real `StoreCreated` events to
a live `stores` JetStream stream on store creation, with a tested, bounded retry
path for transient NATS outages and an honest, documented gap for outages that
outlast it. `CreateProduct` and the remaining seven event-publishing paths are
not yet wired into an application-layer use case — only `CreateStore` was built,
as the proof-of-pattern this phase needed.

**Carried into Phase 4:** The `application` package pattern (`CreateStore` as
the template), `publishWithRetry` as reusable infrastructure for any future use
case, and the full `events` package (envelope, payloads, subject resolution)
ready to be called from the remaining seven command handlers Phase 4's
ConnectRPC layer will need — plus the standing to-do, per ADL 0007, of
documenting the exact wire payload shapes (including the
`ProductPriceDecreased.delta` negative-sign convention) into `CONTRACT.md` once
the API layer's scope is fully known.

I'll append this to `journey_progress.md` now, alongside the finalized ADL 0007.

## Phase 4 — ConnectRPC API (COMPLETE)

**Status:** All 8 Steps complete and verified against live containers, including
a live, scripted, end-to-end curl walkthrough covering the full
`pending → active → completed` lifecycle and its dead ends.

### What Was Built

**Step 1 — `ShoppingList` domain aggregate.** A deliberate redesign, not a port:
the book's `OrderID`/`AssignedBotID`/`Assign()`/`Stops`-for-routing shape was
reworked from first principles once we established `ordering` doesn't exist yet
(ADL 0004) and the `Bot` domain isn't being built at all. Landed on a flat
`pending → active → completed`/`canceled` lifecycle
(`Create → Initiate → Complete/Cancel`, matching `journey_progress.md`'s
original Phase 4 line), with `Stops` kept as a real, store-grouped structure — a
deliberate point of differentiation from `Basket`, per your call that a shopping
list needs to read as a manifest, not a flat cart. `RemoveItem` takes both
`storeID` and `productID` to avoid cross-store ambiguity on a
coincidentally-shared product ID; `Complete()` rejects an empty `Stops` map,
mirroring `Basket.Checkout()`.

**Step 2 — Domain test suite.** 24 tests, full guard/emission coverage, plus two
tests with no `Basket` equivalent:
`TestShoppingList_AddItem_BlockedAfterInitiate` (proving the
freeze-on-`Initiate` behavior) and
`TestShoppingList_RemoveItem_SameProductDifferentStores` (proving the `storeID`
disambiguation actually works, not just compiles).

**Step 3 — `events` package.** Structurally identical to `baskets`', four files.
`ItemAdded`/`ItemRemoved` payloads carry `StoreID` alongside `ProductID` — a
direct consequence of `Stops` being store-keyed.

**Step 4 — `catalog` package.** Same narrow per-event `Repository` shape
`baskets` arrived at only after a mid-Phase-3 redesign (Issue #2 there) —
`depot` gets it from the start. One real addition: `FindStoreByID`, which
`baskets`' catalog never needed, required by `AddItem`'s two-lookup design.

**Step 5 — `postgres` package.** Two migration pairs
(`shopping_lists`/`shopping_list_stops`/`shopping_list_items`;
`catalog_stores`/`catalog_products`, the latter with `last_applied_seq` built in
from its first version rather than bolted on mid-Step the way `baskets`' was).
`ShoppingListRepository.Save` deletes-and-reinserts two tables per save (items,
then stops), one level deeper than `baskets`' single-table pattern.
`initiated_at`/`completed_at` are set via a `CASE`/`COALESCE` pair keyed off
which status is being persisted — the domain layer carries no timestamp fields
at all, resolving the question raised (and deliberately deferred) earlier in the
Phase.

**Step 6 — `application` layer.** Seven use cases (one more than `baskets`' six,
for the standalone `InitiateShoppingList` step). `AddItem` does two catalog
lookups per call (store, then product) inside its `WithLock` transaction, versus
`baskets.AddItem`'s one — the direct cost of `Stops` needing store data the
aggregate itself never fetches. `module.go`'s Phase 2 placeholder
(`-- TEMPORARY --` stream constants, `placeholderHandler`) was fully replaced
with real wiring.

**Step 7 — `connectapi`.** `depotpb/api.proto` (seven RPCs;
`AddItem`/`RemoveItem` both carry `store_id` alongside the usual IDs).
`toProtoShoppingList` converts the domain's `map[string]*Stop` into a
wire-format `[]*Stop` slice, pulling `storeID` back out of the map key — the one
piece of this Step with no `baskets` file to adapt from.

**Step 8 — Integration verification.** Full seven-use-case `application` suite
(16 tests) run against real Postgres/NATS, including both `AddItem`
catalog-failure modes and the full lifecycle chain. A live, scripted, end-to-end
curl verification against real `stores` + `depot` together — multi-store `Stops`
grouping, quantity increment/decrement, stop deletion on last-item removal, the
`Initiate`-freeze guard, and five distinct negative cases.

### Issues Found & Resolved

1. **`depot.Module` didn't expose `CatalogRepo` as a public field**, unlike
   `baskets.Module` — caught only because writing the application test suite
   required seeding catalog data directly the same way your `baskets` tests do
   (`mod.CatalogRepo.CreateProduct(...)`). Fixed by adding the field and
   populating it in `New`, before any test could be written against it. A small
   thing, but a reminder that a test suite can surface a structural gap a
   compile-clean build never would.

### Key Concepts Reinforced

**A structural redesign done at the whiteboard stage, before any code exists, is
cheaper than discovering the same gap mid-implementation.** `Stops`' final
shape, `AddItem`'s two-lookup requirement, and the `OrderID` deferral were all
settled in conversation before Step 1's first line was written — unlike
`baskets`' `catalog.Repository` redesign (Phase 3 Issue #2), which happened
after an initial shape was already built and had to be un-built. Same outcome,
cheaper path to it.

**Removing a mechanism (bots) can force a real re-derivation of a domain's
purpose, not just a smaller version of the same thing.** `Stops` under
bot-routing and `Stops` as a fulfillment manifest are structurally similar but
conceptually different — the redesign conversation (what job does `depot`
actually do, now that nothing physically routes anywhere) mattered as much as
the resulting code.

**Map iteration order is a real, visible consequence, not just a Go trivia
fact.** The live script's own output demonstrated `Stops`' non-deterministic
ordering directly — `toProtoShoppingList`'s map-to-slice conversion means no two
`GetShoppingList` calls are guaranteed to return stops in the same order,
confirmed empirically rather than left as a caveat in this document alone.

**Validation order still determines which error surfaces, same lesson as
`baskets`' Issue #5 — now proven twice, not once.** `AddItem`'s
store-lookup-then-product-lookup ordering means a request that's wrong on both
counts always reports the store failure, never the product failure. The live
script's two separate negative cases (`store depot never cached` vs.
`valid store, unknown product`) were written specifically to prove both branches
independently, rather than trusting the ordering by inspection alone.

### Deliberately Deferred, Carried Forward Rather Than Forgotten

- **`OrderID` on `ShoppingList`/`CreateShoppingList`.** Deferred per ADL 0004's
  no-stubbing rule — `ordering` (Project 3) doesn't exist yet. When it's built,
  adding `OrderID` back is a breaking signature change to `CreateShoppingList`
  (zero args → requires an order reference), not a quiet addition — every
  existing test and the live curl script will need updating at that point, the
  same class of ripple ADL 0007's addendum and ADL 0015 each caused elsewhere.
  Whoever builds `ordering` should start here, not assume `depot`'s API is
  already order-aware.
- **No NATS-outage retry test for `depot`**, unlike `baskets`'
  `TestStartBasket_Execute_SucceedsAfterNATSRestart`. Deliberate, discussed
  choice — `depot`'s `publishWithRetry`/`publishDomainEvent` are structurally
  identical code to `baskets`', so rerunning the same infrastructure-level proof
  was judged unlikely to surface anything `depot`-specific. Worth revisiting
  only if something about `depot`'s actual publish path is ever suspected to
  differ.
- **`ShoppingList`'s sequencing assumption relative to `ordering`** — we assumed
  a completed shopping list is a precondition for order creation, not the
  reverse. Reasonable given the roadmap's fan-in description, but unconfirmed
  against any real `ordering` design yet; worth double-checking once Project 3
  actually starts.

### State at End of Phase 4

- `depot` is fully live: `domain`, `events`, `catalog`, `postgres`,
  `application`, `connectapi` all built, unit-tested (`domain`, 24 tests),
  integration-tested against real Postgres/NATS (`application`, 16 tests), and
  verified end-to-end via live HTTP/ConnectRPC calls against a running `stores`.
- The catalog projector is real, consuming `stores`' full event set via the same
  narrow per-event method pattern `baskets` uses.
- Both `depot-baskets` services (`baskets`, `depot`) are now genuinely live and
  independently verified — the project's original two-service scope is complete.

### Carried Into Phase 5 (Integration & Close-Out)

- Per the original Phase Plan in this file, Phase 5 is full `docker-compose`
  verification with both services genuinely live together, `CONTRACT.md` for
  `depot` (mirroring `stores_CONTRACT.md`/`baskets_CONTRACT.md`'s structure),
  and a `system-state.md` update reflecting `depot` as fully live rather than
  "Not started."
- The three deferred items above (`OrderID`, the skipped NATS-restart test, the
  `ordering`-sequencing assumption) should be restated in `depot`'s own
  `CONTRACT.md` under an "Explicit non-goals" section, the same way
  `baskets_CONTRACT.md` names its own known gaps — not just left in this
  progress log alone.

---

Copy this into `journey_progress.md`, appended after Phase 3's summary. Once
that's done, are you ready to move into Phase 5 — `CONTRACT.md` and
`system-state.md` — or would you like to pause here first?
