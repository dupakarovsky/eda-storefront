# EVENT DRIVE ARCHITECTURE STOREFRONT IN GO: DEPOT + BASKETS

## Project Concept

`depot` and `baskets`, each pairing a **projector** (an independent NATS
JetStream consumer maintaining a local read-only cache of `stores`' catalog
data) with an **aggregate owner** — `baskets` owns `Basket`, `depot` owns a
lean `ShoppingList` (`Create → Initiate → Complete`/`Cancel`, Bot/Stop
sub-domain deliberately cut — confirmed unreferenced anywhere else in the
book's own Chapter12 source, including its own public API, before cutting
it). First project to build the _consuming_ side of NATS; Project 1 only
ever published.

## Concepts We'll Cover

NATS JetStream pull consumers (`foundation/nats` `Subscriber`), idempotent
cache-projection upserts, two independent durable consumer groups against
one stream, cache-miss fallback strategy, multi-listener graceful shutdown
(see ADL 0012), `Basket`/`ShoppingList` as current-state aggregates (ADL 0002) reusing ADL 0008/0009 unchanged.

## Folder Structure

```txt
02-depot-baskets/
├── journey_progress.md
├── baskets/
│   ├── cmd/service/main.go
│   ├── internal/{domain,application,postgres,nats,connectapi}/
│   ├── basketspb/api.proto
│   ├── migrations/
│   └── go.mod
├── depot/
│   ├── cmd/service/main.go
│   ├── internal/{domain,application,postgres,nats,connectapi}/
│   ├── depotpb/api.proto
│   ├── migrations/
│   └── go.mod
├── baskets/CONTRACT.md
└── depot/CONTRACT.md
```

## Phase Plan

Phase 1. Multi-listener shutdown — generalize foundation/shutdown to the Service
interface, concurrent shutdown broadcast, per-service Shutdown→Close fallback;
verify with a real test (SIGTERM mid-request and mid-message-handling), not just
final-state assertions.

Phase 2. **Shared groundwork** — `foundation/nats` gets a `Subscriber` (mirrors
`Publisher`), decide durable-consumer/ack strategy; scaffold both `go.mod`s and
wire empty service skeletons into `docker-compose.yml`.

Phase 3. **`baskets`** — `Basket`/`Item` domain, `Store`/`Product` cache
projector consuming `stores`' events, ConnectRPC (`StartBasket`, `AddItem`,
`RemoveItem`, `Checkout`, `CancelBasket`, `GetBasket`), publish
`BasketCheckedOut` (+ lifecycle events).

Phase 4. **`depot`** — lean `ShoppingList` domain
(`Create → Initiate → Complete`/`Cancel`), same cache-projector pattern reused,
ConnectRPC, publish `ShoppingListCompleted` (+ lifecycle events).

Phase 5. **Integration & close-out** — full `docker-compose` verification with
`stores` genuinely live, end-to-end event flow checked (create a store/product →
confirm both caches pick it up independently), `CONTRACT.md` for each service,
`system-state.md` update, `journey_progress.md` close-out.

---

## Phase 1 Summary — COMPLETE

**Status:** Single-step phase. Step 1 complete and verified. Phase 1
closed out.

### What Was Built

**Step 1 — `Service` interface, `httpService` adapter, `Run` rewrite.**
`foundation/shutdown/service.go` (the `Service` interface),
`foundation/shutdown/http_service.go` (`httpService`/`NewHTTPService`
wrapping `*http.Server`), and `shutdown.go`'s `Run` generalized from a
single `*http.Server` parameter to `services ...Service`, broadcasting
`Shutdown(ctx)` concurrently against one shared deadline with a per-service
`Close()` fallback. Full reasoning and the alternative considered (the
book's `errgroup`/`WaitFunc` shape) recorded in ADL 0012. `01-stores`' call
site updated to the new signature — one line changed, per ADL 0012's
surface-area goal.

### Issues Found & Resolved

Three real bugs surfaced during review, all invisible with only one
registered service (`stores`), all documented in full in ADL 0012's
Consequences:

1. Non-deferred `wg.Done()` — `Run` returned before shutdown had actually
   completed.
2. Duplicate `Close()` call on the failure path.
3. The `errs`-branch `return`ing immediately instead of falling through to
   the shared shutdown sequence — would have left sibling services never
   notified of a peer's failure.

A fourth, in the test harness itself: calling `t.Errorf`/`t.Fatalf` from a
background goroutine after the main test goroutine had already exited via
an earlier `t.Fatalf`, causing a nil-pointer panic instead of a clean
failure — fixed by reporting results back over a channel and asserting
only from the main test goroutine.

### Key Concepts Reinforced

**Proving timing, not just final state.** `TestRun_WaitsForInFlightRequestBeforeReturning`
deliberately races a slow in-flight HTTP request against a triggered
shutdown, and was confirmed to fail (cleanly, on the exact property it
exists to check) against a deliberately-reintroduced non-deferred
`wg.Done()`, then confirmed to pass once restored — same discipline as
ADL 0010's `TestStoreRepository_WithLock_SerializesConcurrentAccess`.

**Concurrent vs. sequential shutdown broadcast.** Shutting services down
one after another inside one shared timeout silently starves later
services of budget — the same class of mistake as ADL 0007's addendum
(retry budget vs. global HTTP timeout). Broadcasting concurrently against
one shared deadline is the only version where the configured timeout means
what it says for every service, not just the first.

**Go 1.22+ per-iteration loop variables.** Confirmed the `svc := svc`
capture-guard pattern common in older Go code (and used throughout the
book, pinned to `go 1.18`) is unnecessary on our current toolchain — each
loop iteration already gets its own variable copy natively.

### State at End of Phase 1

- `foundation/shutdown` now generalizes over any number of `Service`s,
  proven correct via a real concurrency/timing test (both positive and
  negative), not just a manual single-service smoke check.
- `01-stores` runs against the new `Run` signature, manually verified:
  clean startup, clean SIGTERM shutdown sequence, correct log ordering.
- **Known limitation, deliberately deferred** (ADL 0012): concurrent
  `Run()` failures beyond the first are silently absorbed, not logged.
  Worth closing once `depot`/`baskets` both being live makes this concrete
  rather than hypothetical — same treatment as ADL 0007's outbox
  deferral.

### Carried Into Phase 2

- `Service` is the contract the NATS `Subscriber` must be built against
  from the start — `Shutdown` → `ConsumeContext.Drain()`,
  `Close` → `Stop()`.
- The shared `*nats.Conn` is explicitly **not** a `Service` — closed once,
  directly, by `main()` after `Run()` returns, to avoid a `Subscriber`
  yanking the connection out from under a `Publisher` mid-retry.
