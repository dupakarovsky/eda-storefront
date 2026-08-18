# 🟢 Stores Service — Project Plan

## Project Concept

The first live domain service in the Event-Driven Storefront system, and the
root of the whole dependency graph (ADL 0001). Owns two aggregates — `Store`
(name, location, participation flag) and `Product` (a store's catalog item:
name, description, SKU, price) — and is the single source of truth for both.
Publishes domain events onto NATS JetStream for every downstream consumer
(`depot`, `baskets`, later `search`) to react to, and exposes a synchronous
API via ConnectRPC for the storefront and any other synchronous caller.

Adapted from `Chapter12/stores` in the source book, with two deliberate
departures: current-state persistence instead of event sourcing (ADL 0002),
and ConnectRPC instead of the book's real-gRPC-server-plus-grpc-gateway
pairing — chosen because foundation's hand-rolled `Router` already commits us
to serving HTTP directly, and Connect's generated handlers are plain
`http.Handler`s that mount onto it with no second port or proxy hop.

This is the first project since `foundation` was split out (ADL 0005) to
build on top of it rather than build it.

---

## Concepts We'll Cover

**Domain modeling** Aggregate design with current-state persistence (no event
sourcing) — domain events still fired as an integration side effect, not as
the aggregate's own source of truth

**Persistence** Postgres schema/migration via `golang-migrate`, repository
pattern with `Reconstitute` rehydration constructors and current-state
upserts, per-aggregate concurrency control (row lock vs optimistic
version-column CAS, decided by write pattern rather than defaulted
system-wide)

**Messaging** Publishing real domain events over NATS JetStream using
`foundation/nats`'s `Publisher`/`EnsureStream`/`Subject` helpers; subject
naming scheme for the `stores` stream

**API layer** Protobuf service definition, `buf` codegen, and wiring
ConnectRPC handlers as `http.Handler`s onto foundation's existing `Router`
(command + query methods)

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

**Phase 1 — Domain Model**
Build `Store` and `Product` as current-state aggregates: fields,
constructors, behavior methods (`Rebrand`, `EnableParticipation`/
`DisableParticipation`, `IncreasePrice`/`DecreasePrice`, `Remove`, etc.),
validation, and the event payload types they raise. No persistence, no API
yet — testable in isolation with a throwaway script.

**Phase 2 — Postgres Persistence**
Schema + migration for `stores` and `products` tables, repository
interfaces, and Postgres adapters with `Reconstitute` rehydration and
current-state upserts. This is where row-lock vs optimistic-version
concurrency gets decided per aggregate.

**Phase 3 — Event Publishing**
Wire aggregate actions to real NATS JetStream publishes using
`foundation/nats`, including defining the `stores` stream and subject
scheme. By the end, a throwaway script can create a store and we can watch
the event land on NATS for real.

**Phase 4 — ConnectRPC API**
Define `storespb/api.proto`, run `buf` codegen, implement the Connect
handlers, and mount them on foundation's `Router`. Covers both commands
(create/rebrand/add product/etc.) and queries (get store, get catalog,
etc.).

**Phase 5 — Wiring, Compose, and Verification**
`module.go` composition root, `main.go`, extending `docker-compose.yml`/
`Makefile` with the `stores` service and its own Postgres database, then
real `curl`/integration verification against live containers.

**Phase 6 — Contract and Close**
Finalize `CONTRACT.md` with real event/API shapes (superseding the
`system-state.md` placeholders), update `system-state.md`, and write the
closing Phase summary.

---

## Phase 1 — Domain Model ✅

**What was built:**
`Store` and `Product` aggregates in `internal/domain/`, as current-state
structs (ADL 0002) with behavior methods returning `(Event, error)` directly
— no internal event accumulation, no event-sourced rehydration.

- `event.go` — the `Event` interface (`AggregateID() string`,
  `EventName() string`, `OccurredAt() time.Time`) plus an unexported
  `eventBase` struct providing the two bookkeeping fields, embedded by every
  concrete event type to avoid repeating them.
- `events.go` — every concrete event type: `StoreCreated`,
  `StoreParticipationEnabled`, `StoreParticipationDisabled`,
  `StoreRebranded`, `ProductAdded`, `ProductRebranded`,
  `ProductPriceIncreased`, `ProductPriceDecreased`, `ProductRemoved`. Each
  event that could plausibly represent more than one outcome (participation
  toggled, price changed) got split into two distinct concrete types
  instead of one type with a variable event-name field, to keep every
  struct either fully exported (payload) or fully internal (bookkeeping),
  never mixed.
- `store.go` — `Store` struct + `NewStore`, `EnableParticipation`,
  `DisableParticipation`, `Rebrand`.
- `product.go` — `Product` struct + `NewProduct`, `Rebrand`,
  `IncreasePrice`, `DecreasePrice`, `Remove`.
- `store_test.go`, `product_test.go` — full behavior + error-path coverage
  for every method, following `order-fulfillment`/`court-reservation`'s
  testing convention (shared `newTestX` helper, `t.Run` subtests per case),
  adapted to this project's direct-return design rather than a
  `recordEvent`/`PullEvents` pattern.

**Issues found and how they were diagnosed:**
`IncreasePrice`/`DecreasePrice` originally allowed a target price equal to
the current price to succeed (only `<`/`>` were checked, not `<=`/`>=`) —
caught by a direct question before writing tests, not by a failing test.
Traced to the book itself having the same gap. Fixed by switching to
`<=`/`>=`, then covered with explicit "equal price fails" subtests for both
methods to lock the behavior in.

**Key concepts reinforced:**

- Current-state aggregates still raise integration events as a side effect,
  without those events being the source of truth for state (ADL 0002)
- Keeping struct field visibility uninformed by convenience — every field
  either fully exported or fully internal, never mixed on principle, even
  when it costs a couple more types
- Testing error paths for _non_-mutation, not just for the right error type
  — several tests assert state is unchanged after a rejected operation, not
  only that an error was returned

**Deviations from the book, and why:**

- `Rebrand` (both `Store` and `Product`) validates a blank name; the book's
  didn't.
- `IncreasePrice`/`DecreasePrice` reject a negative target price outright,
  and reject an _equal_ price as neither an increase nor a decrease
  (`<=`/`>=` rather than `<`/`>`).
- `Product.Price` is set directly to the new absolute value rather than
  recomputed via `p.Price + delta` — the book's delta-reapplication only
  exists because of event-sourced rehydration, which this system doesn't
  have.
- Event types are one-name-per-concrete-type (`court-reservation`
  convention), not the book's single generic `ddd.Event` wrapper around any
  payload.

**State at end of Phase 1:**
`internal/domain/` package complete and fully tested for both aggregates.
No persistence, no event publishing, no API layer yet — those are Phases
2–4.

**Carried into Phase 2:**
`Store` and `Product` as-is, to be wrapped in Postgres repositories with
`Reconstitute` rehydration and current-state upserts. Concurrency control
strategy (row lock vs optimistic version column) still to be decided per
aggregate based on write pattern.
