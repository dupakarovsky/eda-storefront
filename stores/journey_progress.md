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
