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

______________________________________________________________________

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

## Phase 4 — ConnectRPC API (COMPLETED)

**What was built:**

- `storespb/api.proto` — the full `StoresService` contract: eleven RPCs covering
  every domain command from Phases 1–3 (`CreateStore`, `RebrandStore`,
  `EnableStoreParticipation`, `DisableStoreParticipation`, `AddProduct`,
  `RebrandProduct`, `IncreaseProductPrice`, `DecreaseProductPrice`,
  `RemoveProduct`) plus the project's first two queries (`GetStore`,
  `ListProducts`). One dedicated request/response message pair per RPC rather
  than shared shapes, mirroring Phase 1's one-concrete-type-per-event
  discipline.
- `buf.yaml`/`buf.gen.yaml` — `buf` codegen wiring (`protoc-gen-go`,
  `protoc-gen-connect-go`), producing `storespb/api.pb.go` and
  `storespb/storespbconnect/api.connect.go`.
- `internal/connectapi/server.go` — the `Server` type implementing
  `storespbconnect.StoresServiceHandler` in full, each handler translating wire
  messages to/from `application`-layer calls; `errors.go`'s `mapError`
  translating domain sentinel errors (`ErrStoreNotFound`,
  `ErrStoreIsAlreadyParticipating`, etc.) into the right Connect error codes
  (`CodeNotFound`/`CodeInvalidArgument`/`CodeInternal`), confirmed reaching real
  HTTP status codes (404/400) through the actual wire, not just asserted via
  `errors.Is` in a unit test.
- Five new `application`-layer use cases
  (`RebrandStore`/`EnableStoreParticipation`/`DisableStoreParticipation`/
  `AddProduct`/`RebrandProduct`/`IncreaseProductPrice`/
  `DecreaseProductPrice`/`RemoveProduct` — eight total alongside Phase 3's
  `CreateStore`) plus two read-only use cases (`GetStore`, `ListProducts`), the
  latter deliberately kept in the `application` layer rather than letting
  `connectapi.Server` call repositories directly (ADL 0008).
- `internal/application/publish_event.go`'s `publishDomainEvent` extracted as a
  shared helper once a second, third, and fourth use case needed the same
  envelope→marshal→subject→publish-with-retry tail Phase 3 wrote once for
  `CreateStore`.
- `ProductRepository.Delete`/`ListByStoreID`, `StoreRepository`/
  `ProductRepository.FindByID` split from their locking `FindByIDForUpdate`
  counterparts (Step 6) — resolving the "row-locking wired but dormant" gap
  Phase 2 had explicitly flagged and deferred.
- `internal/postgres/txcontext.go`'s `ctxWithTx`/`executorFrom` pair, making
  `WithLock` actually thread a real `pgx.Tx` through to every repository method
  called inside its callback — the concrete fix for that same dormant-lock gap
  (ADL 0010), proven not just by re-passing existing tests but by a dedicated
  concurrency test measuring genuine blocked time under a held lock
  (`TestStoreRepository_WithLock_SerializesConcurrentAccess`).
- Server-generated UUIDs for every new aggregate (ADL 0009), decided before
  writing `api.proto` and implemented in `connectapi.Server`'s
  `CreateStore`/`AddProduct` handlers.
- Twenty-plus tests across `internal/application/*_test.go` — one
  success/rejection pair per command and query use case, following the same
  real-infrastructure discipline established in Phase 3
  (`newTestModule`/`uniqueID` shared test helpers, extracted once three files
  needed the identical boot sequence).
- `01-stores/Dockerfile` — two-stage build (`golang:1.25` → `alpine:3.20`), the
  project's first containerized deployment path.
- `docker-compose.yml` extended with a real `stores` service block
  (network-internal Postgres/NATS addressing via `environment:` overrides
  layered on `env_file`), plus healthchecks added to the `postgres` block;
  `nats` deliberately left without one after confirming `nats:2.12`'s official
  image has no shell or utilities capable of running one — the startup-ordering
  gap that would have covered is instead closed via `foundation/nats`'s new
  `RetryOnFailedConnect(true)` option, a genuine improvement to `foundation`
  itself.
- `foundation/logging.New` updated to support an empty `logFilePath`
  (console-only output), driven by a new `LOG_FILE_PATH` config value — closing
  the "container's log file vanishes on rebuild" gap identified while writing
  the Dockerfile, rather than carrying it forward silently.
- `Makefile` reorganized into deployment-vs-development sections, plus a new
  `build` target.
- Four new ADL entries (0008–0010, plus an addendum to 0007) capturing the
  query-layering decision, server-side ID generation, the transaction- locking
  pattern choice (and the `TxManager` alternative considered and set aside), and
  the request-timeout/retry-budget reconciliation.

**Issues found and how they were diagnosed:**

- The original `WithLock` implementation began a real `pgx.Tx` but never
  threaded it into the calls made inside its own callback — every repository
  method still ran against the bare pool regardless. This wasn't caught by any
  of Steps 4–6's tests, since none of them were designed to distinguish "a lock
  was held" from "no lock exists" — a single-threaded call sequence produces
  identical final database state either way. Only a dedicated concurrency test,
  measuring actual wall-clock blocked time on a second goroutine while the first
  genuinely held the row, could prove the difference — and did, failing clearly
  against the dormant version before the context-propagation fix.
- An initial `Executor` interface addition (`BeginTx(ctx, pgx.TxOptions)`) would
  have made `pgx.Tx` itself fail to satisfy `Executor`, since `pgx.Tx`'s real
  interface only has `Begin` (for nested transactions/savepoints), not `BeginTx`
  — that method only exists on `*pgxpool.Pool`/`*pgx.Conn`. Caught by inspection
  before a build was attempted, once the interface was reviewed against what
  `pgx.Tx` actually exposes, rather than surfacing as a confusing compile error
  later.
- `nats:2.12`'s official image has no shell, `wget`, `curl`, or `ls` — confirmed
  by direct experimentation (`docker run --entrypoint sh` and `--entrypoint ls`
  both failing with "executable file not found"), not assumed. This made a
  `CMD-SHELL`-style Docker healthcheck structurally impossible against this
  image, regardless of which command was tried inside it — resolved by moving
  the readiness guarantee to the NATS client's own `RetryOnFailedConnect` option
  instead of the container orchestration layer.
- The global `SRV_REQUEST_TIMEOUT`/`SRV_WRITE_TIMEOUT` values (3s/10s) were
  shorter than ADL 0007's measured ~10.5s worst-case retry budget — a real gap
  invisible until an actual HTTP-layer request had a reason to exercise the full
  retry path, which nothing had prior to this phase (Phase 3's own outage test
  called `Execute` directly against an undeadlined `context.Background()`).
  Resolved by raising both defaults in `foundation/config` past the measured
  worst case (12s/13s), accepting read RPCs inheriting the same ceiling as a
  documented, named trade-off (ADL 0007 addendum) rather than solving it with a
  bespoke per-route timeout mid-phase.

**Key concepts reinforced:**

- The difference between a data-correctness test and a concurrency- correctness
  test — the former confirms an operation's final effect, the latter must
  confirm something about _timing_ under real contention, since a broken lock
  and a working lock can produce identical final state in a single-threaded
  scenario.
- Go's structural interface-to-interface assignability check (`pgx.Tx` failing
  to satisfy an `Executor` that declared a method `pgx.Tx` doesn't have) as a
  real, if narrow, category of compile error worth understanding rather than
  discovering by trial and error.
- `go mod tidy`'s single-module scope versus `go build`/`go run`/`go test`'s
  workspace-aware scope — carried over from Phase 3's `go.work` discoveries,
  revisited here as background context for why `01-stores/ go.mod`'s `replace`
  directive (not `go.work`) is what makes the Dockerfile able to build without
  any workspace file present in the image at all.
- Container image minimalism as a real operational constraint, not just a
  security nicety — `nats:2.12`'s missing shell forced a genuine architectural
  choice (client-side retry vs. container-side healthcheck) rather than a
  cosmetic one.
- Protobuf JSON's default zero-value omission applying identically to scalar
  fields (`participating: false`, first seen in Phase 3/Step 3) and empty
  repeated fields (`products: []` rendering as an absent key entirely) — the
  same underlying behavior, confirmed at the wire level for the first time in
  Step 8 after being designed for at the Go level back in Step 6.

**Deviations from the original Phase 4 plan, and why:**

- The original plan (see this file's Phase Plan section) scoped Phase 4 as five
  steps ending in "Wiring, Compose, and Verification" as Phase 5. In practice,
  verifying the API layer surfaced two genuine, unplanned pieces of work — the
  dormant-lock fix and the timeout/retry reconciliation — each substantial
  enough to warrant its own Step (7 and part of 8) and ADL entry rather than
  being folded silently into what was meant to be a wiring pass. Containerizing
  `stores` (Step 9) was likewise not in the original five-step plan, added once
  it became clear `system-state.md` marking `stores` "Live" needed to mean what
  it already means for `foundation` — a real running container, not a process on
  the developer's machine — before `02-depot-baskets` could honestly depend on
  it.
- `RemoveProduct` has no domain-level rejection case (`Product.Remove()` cannot
  fail) — its test suite's "rejection" case is instead "called against an ID
  that doesn't exist," the only real failure mode left once the domain method
  itself can't reject.
- The originally-considered `domain.TxManager` pattern (explicit tx-scoped
  repositories passed as callback arguments) was fully prototyped — all seven
  affected use cases rewritten, a `postgres.TxManager` implementation written —
  before being set aside in favor of the context-scoped `Executor` approach,
  specifically to avoid `TxManager.WithTx` forcing every callback to receive
  both `StoreRepository` and `ProductRepository` regardless of which aggregate a
  given use case actually concerns (ADL 0010). Worth remembering this was a
  real, working alternative that was deliberately not chosen, not an approach
  that was tried and failed.

**State at end of Phase 4:** `01-stores` exposes all eleven RPCs from
`api.proto` through ConnectRPC, backed by real `application`-layer use cases for
every command and query, verified three independent ways — direct `Execute`
calls in Go tests, real HTTP requests against `go run` (Step 3/8), and real HTTP
requests against the actual container `02-depot-baskets` will depend on (Step
9). The transactional row-locking gap Phase 2 left open is closed and proven
under real concurrency. `docker-compose.yml`/`Makefile` now produce a genuinely
running `stores` container, matching every other service's deployment shape for
the first time.

**Carried into Phase 5 (`02-depot-baskets`):** `01-stores`'s `CONTRACT.md`
(finalized alongside this entry) is what `depot`/`baskets` will actually consume
— not this file, not `01-stores`' source tree, per ADL 0004. The
`foundation/nats` `RetryOnFailedConnect` addition and `foundation/logging`'s
empty-path console-only mode are both real `foundation` improvements every
future service inherits automatically. The still-open items explicitly not
resolved in this phase — per-procedure timeouts via a Connect interceptor, the
transactional outbox deferred by ADL 0007, and the `TxManager` alternative noted
in ADL 0010 as worth revisiting once a service needs a genuine cross-aggregate
transaction — remain standing items for whichever future phase first has
concrete pressure to resolve them, not closed questions.
