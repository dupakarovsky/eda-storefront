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

## Phase 2. **Shared groundwork** — `foundation/nats` gets a `Subscriber`

Good — I've read `roadmap.md`, all the ADLs through 0012, `CONTRACT.md`,
`system-state.md`, and `journey_progress.md`, plus the `foundation` source
you've shared (shutdown/service.go, http_service.go, nats/_, postgres/_,
config.go, router.go, logging.go, timeout.go, recovery.go) and the
compose/Makefile/Postgres hardening files. I also pulled up the
`learning-journey` skill to run this the way you've set it up.

Quick recap so we're aligned before touching code:

Given the scope, here's how I'd like to break Phase 2 into Steps:

- **Step 1** — Design and implement `foundation/nats.Subscriber`: the durable
  pull-consumer wiring (`ConsumeContext`), and crucially satisfying the
  `shutdown.Service` interface from ADL 0012 (`Shutdown` → `Drain()`, `Close` →
  `Stop()`, as already decided in that ADL).
- **Step 2** — The ack/redelivery strategy discussion + any config knobs needed
  (durable name, max deliver, ack wait) — this is a real design decision point,
  not just typing code, so I'd like to walk through the tradeoffs with you
  before locking it in.
- **Step 3** — Scaffold `baskets/go.mod` and `depot/go.mod`, wire both into
  `go.work`.
- **Step 4** — Empty service skeletons (`main.go` stubs that start up, connect
  to NATS/Postgres, and shut down cleanly) wired into `docker-compose.yml` —
  proving the plumbing before either service has any domain logic.

Does that Step breakdown look right to you, or would you rather fold 1 and 2
together since they're tightly related? Once you confirm, I'll start on Step 1.

### Step 1 — Design and implement `foundation/nats.Subscriber`

Let's build the `Subscriber`. Before the code, a couple of things I looked up to
get this right rather than guess, since it matters for correctness:

`jetstream.ConsumeContext` (the thing `Consumer.Consume(handler)` returns) has
three methods, not two: `Stop()` (abandon immediately, discard buffered
messages), `Drain()` (stop pulling new messages but let buffered ones finish
processing), and `Closed() <-chan struct{}` — a channel that closes only once
draining/stopping has _actually completed_. ADL 0012 already decided `Shutdown`
→ `Drain()` and `Close` → `Stop()`, but it didn't specify how `Shutdown` knows
drain is _done_ — without reading `Closed()`, `Shutdown(ctx)` would call
`Drain()` and return immediately, which defeats the whole "wait for in-flight
work" point of `shutdown.Run`'s concurrent-broadcast design from Phase 1. So
`Shutdown` needs to block on `Closed()` (respecting the passed-in `ctx`
deadline).

One more real-world wrinkle worth guarding against: there's a known nats.go
issue where calling `Stop()`/`Drain()` more than once, or racing them, can panic
internally (closing an already-closed channel). Our own `Shutdown`→`Close`
fallback path in `shutdown.Run` calls them sequentially in the same goroutine so
that specific race shouldn't hit us, but I'll still guard our own `done` channel
close with a `sync.Once` defensively, since a channel close panic is exactly the
kind of failure mode that shouldn't be possible by construction.

Here's `foundation/nats/subscribe.go`:

```go
// foundation/nats/subscribe.go
```

What's happening here, and why it's shaped this way:

`Run()` follows the exact same "block until told to stop" contract
`httpService.Run()` already established — `ListenAndServe()` blocks there; here,
`<-s.done` blocks instead, since `Consume()` itself is non-blocking (it spins up
its own delivery goroutine and returns immediately). Without that final blocking
receive, `Run()` would return the instant `Consume` was called, `shutdown.Run`
would see it as "exited," and the whole `Service` contract from ADL 0012 would
be violated for this listener specifically.

`onMessage` is the thin wrapper that turns our `MessageHandler`'s `error` return
into JetStream's ack/nak vocabulary — `nil` error acks, any error naks
(triggering redelivery per the consumer's `MaxDeliver`/`AckWait`, which we'll
pin down as an explicit decision in Step 2). I deliberately didn't give the
handler direct access to `msg.Ack()`/`msg.Nak()`/`msg.Term()` itself — keeping
`foundation` as a thin mechanism layer, with disposition-nuance (like
poison-message handling) left as a decision for whichever service actually needs
it, not baked in here speculatively.

`Shutdown(ctx)` is the one piece that goes a bit beyond ADL 0012's original
wording, for the reason above: it calls `Drain()`, then genuinely waits — either
`cc.Closed()` fires (drain actually finished) or the caller's `ctx` deadline
expires first, in which case it returns `ctx.Err()` so `shutdown.Run` correctly
treats it as a failed shutdown and falls through to `Close()`.

Give this a compile check on its own — it doesn't need the rest of the service
wired up yet, but Go will catch signature mismatches against the real
`jetstream` package immediately. A `go build ./foundation/...` (or `go vet` if
you'd rather not produce a binary) from the workspace root should be enough to
confirm it type-checks against your actual `nats.go` version. Let me know what
you get, or paste the error if `ConsumeContext`'s method set doesn't match what
I described above — that's exactly the kind of thing worth catching here rather
than three files downstream.

### NATS internals

Good questions — let's go through them one at a time.

**`jetstream.Consumer`** — yes, that's it. It's the handle to an already-created
(or created-via-`CreateOrUpdateConsumer`) durable or ephemeral consumer on a
stream, and it's the type you call `Consume()`, `Fetch()`, or `Messages()` on to
actually pull data out.

**`Consume()`** — your reading of the doc comment is exactly right. Under the
hood it manages a continuous cycle of pull requests against the server on your
behalf and invokes your callback (`onMessage` in our case) once per message as
they arrive. You never call "fetch again" yourself — that looping is what
`Consume()` is doing for you.

**The return value is `jetstream.ConsumeContext`** (small correction — not
`consumer.Context`, no relation to Go's `context.Context`; it's just named
similarly to that pattern). It's the handle you use to later stop that
background pulling.

**Why the mutex around assigning `s.cc`:** because it's written in one goroutine
and read in a different one. `Run()` executes inside the goroutine
`shutdown.Run` spawned for this service
(`go func() { errs <- services[i].Run() }()`). `Shutdown()`/`Close()` execute
later, inside separate goroutines `shutdown.Run` spawns specifically for the
shutdown broadcast. So there's a real cross-goroutine read/write on the same
field, with no ordering guarantee between "Run's goroutine finishes assigning
`s.cc`" and "Shutdown's goroutine reads `s.cc`" unless something synchronizes
them. Go's memory model doesn't guarantee the reading goroutine ever sees the
write at all without that synchronization — and since an interface value like
`jetstream.ConsumeContext` is actually two machine words internally (type
pointer + data pointer), an unsynchronized concurrent read/write can even
observe a torn value, not just a stale one. The mutex is the cheapest fix for
exactly that hazard — same category of bug ADL 0010 and ADL 0012 both already
caught elsewhere in this project.

**Does `Run()` staying blocked keep `cc` receiving?** Almost — but the mechanism
is slightly different from what you described. The actual message pulling
happens on background goroutine(s) that `Consume()` itself spun up internally,
completely independent of whether our `Run()` function's stack frame is blocked
or not. If `Run()` returned right after calling `Consume()`, messages would
_still_ keep arriving — `Consume`'s internal loop doesn't care about our
function's control flow at all. The `<-s.done` block exists purely to satisfy
the `shutdown.Service` **contract**: `shutdown.Run` uses "has `Run()` returned?"
as its signal for "has this service stopped," so if our `Run()` returned early,
`shutdown.Run` would think the subscriber had already exited (or crashed) and
could kick off the whole shutdown sequence prematurely. So: message delivery
keeps happening regardless, but blocking `Run()` is what keeps the _bookkeeping_
honest.

**`Stop()` vs `Drain()`** — your read is correct. `Stop()` cancels immediately
and throws away anything already sitting in the client-side buffer, unprocessed.
`Drain()` stops asking the server for _new_ messages but lets whatever's already
buffered client-side finish running through your callback (and get acked/nak'd)
before it actually finishes closing.

**The `Shutdown` flow** — also correct. `Drain()` itself returns immediately —
it just flips a switch. The actual waiting happens in the `select`: either
`cc.Closed()` fires once draining has genuinely finished, or `ctx.Done()` fires
first if the caller's shutdown deadline runs out before drain completes. Either
way we call `markDone()` to unblock `Run()` — the difference is just what error
(if any) `Shutdown` reports back to `shutdown.Run` about whether it finished
cleanly.

Does that clear it up? Happy to go a level deeper on how `Consume`'s internal
pull loop actually works against the NATS server if that's useful context, or we
can move on to compiling this and then Step 2 (the durable/ack-policy decision).
