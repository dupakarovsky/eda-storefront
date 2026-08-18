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

### Step 2: StoreParticipation Enabled, Disable and Rebrand Events

Agreed — that's consistent with the `StoreCreated`/`StoreRebranded` shape, and
avoids the mixed-visibility problem entirely by trading one type with a variable
name for two types with fixed names. Here's the full `events.go` addition and
the corrected `store.go` methods:

```go
// internal/domain/events.go
```

`Rebrand`'s blank-name check is back in, using the same `ErrStoreNameIsBlank`
sentinel `NewStore` already uses — one error, reused wherever "a store needs a
non-blank name" is being enforced, rather than a second near-identical sentinel.

Build it and let me know how it goes. Once that's clean, we'll move to `Product`
— same pattern (constructor + behaviors + events), but with a couple of new
things: it references a `StoreID` it doesn't own, and price changes
(`IncreasePrice`/`DecreasePrice`) need to decide what happens on invalid input
(negative resulting price) that `Store`'s methods so far haven't had to
consider.

### Step 3: Product Aggragate

`Product` follows the same shape as `Store`, with a bit more happening in the
price methods. Here's the full aggregate — constructor, `Rebrand`,
`IncreasePrice`/`DecreasePrice`, and `Remove` — plus its events.

```go
// internal/domain/events.go — additions
```

Three deviations from the book worth flagging explicitly:

1. **`Rebrand` validates blank name**, same reasoning as `Store.Rebrand` — the
   book didn't check it there either, and we already decided that gap was worth
   closing.
1. **`IncreasePrice`/`DecreasePrice` reject negative prices**, which the book's
   version doesn't do (only `InitProduct` checks `price < 0`). Since these take
   an absolute target price, not a delta, nothing stops a negative one from
   slipping through post-creation without this.
1. **Price is stored and set directly** (`p.Price = price`) rather than via
   delta arithmetic. The book computes `Delta` and later reapplies it with
   `p.Price = p.Price + payload.Delta` inside `ApplyEvent` — a pattern that
   exists because their `ApplyEvent` is the _only_ path that mutates state
   (event-sourced rehydration). We don't have that separate path — the method
   itself is the one and only place `Price` changes — so recomputing the same
   value via addition would just be redundant arithmetic for the same result.
   The event's `Delta` field is kept as-is, since a consumer (`depot`'s cache,
   say) may still want "price went up by $2" rather than just the new absolute
   price.

`Remove()` always succeeds — no invariant currently blocks removing a product —
so it returns `(Event, error)` only for signature consistency with the other
methods, not because an error is actually reachable. If a real precondition
shows up later (e.g. "can't remove a product with pending orders"), the
signature's already there to support it.

Build this and let me know how it goes — then we'll write `product_test.go` the
same way we did for `Store`.

#### Test Files

`product_test.go`, following the same shape as `store_test.go` — a shared constructor helper, then one `t.Run` subtest per behavior and its error paths, including the equal-price case for both `IncreasePrice` and `DecreasePrice`.

```go
// internal/domain/product_test.go
```

One thing to note on the negative-price checks for `IncreasePrice`/`DecreasePrice`: since both already reject any target price on the wrong side of the current price via `<=`/`>=`, and `NewProduct` already guarantees `Price >= 0` at creation, a negative target price for `IncreasePrice` would already be caught by `price <= p.Price` in virtually every real case (you can't increase into negative territory from a non-negative starting price) — the explicit `price < 0` check is mostly a defense-in-depth/clarity thing rather than something reachable through normal use. Worth keeping for a clear error message either way, but wanted to flag that the two checks overlap in coverage rather than being fully independent guards.

Run `go test ./01-stores/internal/domain/... -v` and let me know how it goes.
