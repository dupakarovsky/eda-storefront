# EVENT DRIVE ARCHITECTURE STOREFRONT IN GO: STORES

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

## **Phase 1 — Domain Model**

### Step 1: Store and Product

**Setup:** create `01-stores/` at the workspace root, and inside it run
`go mod init eda-storefront/stores` (adjust the module path to match whatever
prefix `foundation`'s `go.mod` uses, so imports stay consistent). Then add it to
the root `go.work`:

```
// go.work
go 1.25.0

use (
    ./foundation
    ./foundation-smoketest/
    ./01-stores
)
```

Now the aggregate. Two files: `internal/domain/events.go` for the shared event
wrapper type, and `internal/domain/store.go` for `Store` itself.

```go
// internal/domain/events.go
```

```go
// internal/domain/store.go
```

What's happening here: `DomainEvent` is a deliberately minimal replacement for
the book's `ddd.Event`/`registry.Registerable` machinery — ADL 0002 already
ruled out porting `internal/es`, and since we're not event-sourcing, we don't
need an event type that knows how to be replayed, only one that knows its name
and payload well enough to hand to a publisher later (Phase 3).
`Name`/`Payload any` is enough for that.

Notice `NewStore` takes `id` as a parameter rather than generating it
internally. That's on purpose — ID generation (UUID or otherwise) is an
application-layer decision, not a domain one, so the constructor stays pure and
trivially testable: same inputs, same outputs, no hidden side effect.
`Participating` is left at its zero value (`false`) on creation, matching the
book — a new store doesn't participate until someone explicitly enables it,
which we'll build in the next step.

One more thing worth flagging: the book wraps errors with
`github.com/stackus/errors` and an `ErrBadRequest` sentinel, presumably so the
gRPC layer can map domain errors to status codes generically. We're using plain
`errors.New` for now — simpler, and we don't yet know how ConnectRPC's
error-code mapping wants to work (Phase 4). If we need richer error
classification once we get there, we can revisit this without it being a big
change.

Go ahead and create those two files, then run `go build ./...` from inside
`01-stores/` to confirm it compiles cleanly — there's no `main` yet, so this is
just a compile check. Let me know how that goes, and if you have questions about
the `DomainEvent` design before we move to Step 2 (`Rebrand`,
`EnableParticipation`, `DisableParticipation`).
