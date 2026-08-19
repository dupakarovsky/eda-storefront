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

______________________________________________________________________

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

______________________________________________________________________

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

______________________________________________________________________

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

______________________________________________________________________

## Phase 1 — Domain Model ✅

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

______________________________________________________________________

## Phase 2 — Postgres Persistence ✅

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
