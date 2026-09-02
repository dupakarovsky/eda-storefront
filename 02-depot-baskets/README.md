# EVENT DRIVE ARCHITECTURE STOREFRONT IN GO: DEPOT + BASKETS

## Project concept

`depot` and `baskets` are each two things: a **projector** (an independent NATS
consumer that reads `stores`' catalog events and maintains a local read-only
cache of stores/products, so lookups don't require a live synchronous call to
`stores`) and an **aggregate owner** (`baskets` owns `Basket`; `depot` owns
`ShoppingList`, lean version). Same pairing pattern as `stores`' domain logic,
but this is the project's first time consuming events rather than only
publishing them.

## Concepts we'll cover

- **NATS JetStream consumers** — first time building the _subscribe_ side
  (Project 1 only built `Publisher`). We'll extend `foundation/nats` with a
  `Subscriber`, deciding durable-consumer naming and push-vs-pull deliberately
  rather than defaulting.
- **Idempotent projection** — cache upserts (`ON CONFLICT DO NOTHING`/`UPDATE`)
  so at-least-once delivery (ADL 0007) can't corrupt the cache on redelivery.
- **Two independent consumer groups on one stream** — `depot` and `baskets` each
  get their own durable consumer name against `stores`' stream; same events, two
  unrelated projections.
- **Cache-miss fallback** — the book's pattern is a synchronous gRPC call to
  `stores` when the cache hasn't caught up yet; worth deciding deliberately
  whether we keep that or accept eventual consistency.
- **Two aggregates, same persistence discipline** — `Basket` and `ShoppingList`
  both as current-state rows (ADL 0002), reusing ADL 0009 (server-generated IDs)
  and ADL 0008 (query-through-application) unchanged.

## Folder structure

```
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
├── baskets/CONTRACT.md   (produced at close, per ADL 0004)
└── depot/CONTRACT.md
```

## Phase plan

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

### Phase 1, Step 1 -- Service interface and concurrent Shutdown

Three files: the `Service` contract, an `http.Server` adapter satisfying it, and
the rewritten `Run`.

```go
// foundation/shutdown/service.go
```

```go
// foundation/shutdown/http_service.go
```

```go
// foundation/shutdown/shutdown.go
```

The trigger for shutdown is now whichever fires first — a signal, or any
service's `Run()` returning an error — and either way every service gets
`Shutdown(ctx)` broadcast concurrently against one shared deadline, with
`Close()` as its own per-service fallback if that deadline is blown (points 2
and 3 from before). `runErr` is only non-nil when a real service failure (not a
signal) triggered the sequence, so `main` can still distinguish "asked to stop"
from "something broke."

At `01-stores`' call site, this is a one-line change:
`shutdown.Run(logger, srv, cfg.Server.ShutdownTimeout)` becomes
`shutdown.Run(logger, cfg.Server.ShutdownTimeout, shutdown.HTTPService(logger, srv))`.

Go ahead and drop these in — want to review before you wire up the call site and
test it, or run it as-is first?

#### Testing the new Shutdown

The manual run looks correct, but it doesn't actually prove the fix works — no
request was in-flight when SIGTERM landed (the health check three minutes
earlier had long since completed), so the ~150µs gap between "shutdown signal
received" and "shutdown complete" is just as consistent with the old, broken
(non-deferred `wg.Done()`) version as with the fixed one. This is exactly the
ADL 0010 lesson repeating itself: a lock (or here, a wait) that never engaged
and one that engaged correctly can look identical under a test that never
actually exercises concurrent/in-flight state. We need a real timing-based test,
not another log read.

```go
// foundation/shutdown/shutdown_test.go
```

The mechanics: `trigger`'s `Run()` deliberately fails at 100ms, forcing `Run`
into its shutdown path while the `/slow` request (which takes 400ms) is still
being handled. If `wg.Done()` isn't deferred — the exact bug you just fixed —
`Run` returns almost immediately after the shutdown loop starts, well before the
request finishes, and the `runDoneAt.Before(reqDoneAt)` check fails loudly.
Against the fixed code, `srv.Shutdown(ctx)` genuinely blocks until the handler
finishes, so `Run` can only return after.

Run
`go test ./foundation/shutdown/... -run TestRun_WaitsForInFlightRequestBeforeReturning -v`
and paste the result. If you want to see it actually catch the bug (worth doing
once, for confidence in the test itself, same as ADL 0010's "confirmed necessary
by testing against the dormant version too"), temporarily remove the `defer`
from `wg.Done()`, confirm this test fails, then put `defer` back before moving
on.
