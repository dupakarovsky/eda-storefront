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

### Step 2 — The ack/redelivery strategy discussion

Nice — Subscriber's locked in structurally. On to **Step 2: durable-consumer and
ack strategy**. This is one of those "worth deciding deliberately and writing
down" moments (same spirit as ADL 0007's outbox deferral) rather than picking
defaults quietly, so let's walk through the actual choices.

**1. How many durable consumers, and named how?**

Both `depot` and `baskets` need to independently read the _same_ `stores` stream
(`stores.store.*` / `stores.product.*`) to build their own separate local
caches. This is exactly the "two independent durable consumer groups against one
stream" item your `journey_progress.md` flagged as a concept to cover.
Concretely: two distinct `Durable` names — something like
`depot-stores-projector` and `baskets-stores-projector` — each maintaining its
own separate delivery cursor on the same stream. Neither sees the other's ack
state; a message redelivered to `depot` because its handler failed has no effect
on what `baskets` has already acked.

**2. Filter subject — narrow or take everything?**

Per ADL 0001, both `depot` and `baskets` maintain "a local product/store cache"
— i.e., they need _both_ store events and product events, not a subset. So I'd
leave `FilterSubject` empty on both consumers (consume the whole stream) rather
than filtering to `stores.store.*` or `stores.product.*` alone. If that
assumption's wrong — e.g. if `depot` genuinely only ever needs product events —
that'd change this.

**3. Ack policy — this one's not really a choice.** `Consume()` with a
manual-disposition callback (what our `Subscriber` does) requires
`AckExplicitPolicy`. The alternatives (`AckNonePolicy`, `AckAllPolicy`) don't
fit a "the handler decides ack/nak per message" model at all, so this is
effectively forced by the design already committed to in Step 1.

**4. Redelivery — `AckWait` and `MaxDeliver`, and whether to set `BackOff`**

This is the one with real tradeoffs. Three sub-decisions:

- **`AckWait`** — how long JetStream waits for an ack before considering a
  message eligible for redelivery. Default is 30s. Since our handlers are meant
  to be fast idempotent Postgres upserts (not the kind of slow work `AckWait` is
  usually tuned around), I'd lean toward leaving this at the default rather than
  inventing a number without a measured reason to change it — consistent with
  how `pool.go`'s comment treats unmeasured tuning as a known bug pattern to
  avoid repeating.
- **`MaxDeliver`** — a bounded number of redelivery attempts before JetStream
  gives up on a message (server-side; not the same as ADL 0007's _inline_ retry
  loop, which was about the publish side, not consumption). Unbounded (`-1`)
  risks a poison message looping forever if a handler has a genuine,
  non-transient bug. I'd propose a bounded value, e.g. `5`.
- **`BackOff`** — an optional `[]time.Duration` schedule for redelivery spacing
  (first retry after X, second after Y, etc.), instead of every retry waiting
  the flat `AckWait` duration. Purely optional — omitting it just means every
  redelivery waits `AckWait` uniformly.

**The real question for you:** once `MaxDeliver` is exhausted, JetStream just
stops redelivering — there's no automatic dead-letter queue in this library
version, and our current `Subscriber` doesn't expose `Term()` for a handler to
explicitly give up early either. That means a persistently-failing message (say,
`depot`'s DB is down for an extended stretch) will silently stop being retried
after 5 attempts, with nothing but a log line (once we add one) marking it.
That's a real gap, structurally similar to ADL 0007's "publish retry exhausted,
nobody notices" gap.

Given that parallel, I'd suggest we **name this gap explicitly and defer it**
the same way ADL 0007 did, rather than solve dead-lettering now as groundwork
scope creep. Does that match your instinct, or would you rather we build a
minimal dead-letter subject now while we're already in this code?

#### Retries on Failed to Deliver Messages

**A real problem with the current `onMessage`:** `msg.Nak()` (no arguments)
**does not respect `AckWait` or `BackOff` at all** — it triggers an _immediate_
redelivery, full stop. Those settings only govern the case where a message is
delivered and the client _never responds_ (crash, hang) — the server's own
timeout-based redelivery. Since our `onMessage` explicitly calls `Nak()` on
every handler error, the `BackOff` schedule we're about to configure would
silently never engage on the failure path we actually care about — every retry
would fire back-to-back as fast as the client can process them, hammering
whatever's down (e.g. a stalled Postgres) in a tight loop bounded only by
`MaxDeliver`. That's the opposite of what you asked for. To get real backoff, we
need to switch to `NakWithDelay(delay)`, computing `delay` ourselves from the
message's own delivery count and our `BackOff` schedule.

That same code path is also the natural place to solve your dead-letter
requirement — since we're already inspecting delivery count to pick a backoff
delay, we already know exactly when we're on the _last_ attempt.

**Revised `onMessage`, plus the pieces it needs:**

```go
// foundation/nats/subscribe.go
```

A few things worth being explicit about:

- **`msg.Term()` on the last attempt, rather than trusting `MaxDeliver` alone to
  stop things.** `Term()` unconditionally tells the server to stop redelivering
  this message, regardless of `MaxDeliver`. Deciding to give up is now an
  explicit act in our own code, made once, at the point we've already decided to
  dead-letter — not an implicit consequence of a number matching up correctly.
  Same "make the boundary legible" instinct as ADL 0006's explicit `reject`
  rules.
- **`DeadLetterFunc` is a caller-supplied callback, not something `Subscriber`
  hardcodes.** `foundation` stays a thin mechanism layer; what "dead lettering"
  actually _means_ (publish somewhere, log, page someone) is a decision for
  whoever wires up a concrete `Subscriber`. That said, since every future
  consumer will want _something_ here, I'd suggest a small default helper
  alongside it:

```go
// foundation/nats/deadletter.go
```

This reuses the `Publisher` we already have rather than inventing a second
publish path — a dead letter is just a message, persisted onto its own subject
on a JetStream stream, inspectable and replayable later. It doesn't retry itself
if the dead-letter publish fails (unlike ADL 0007's `publishWithRetry`) — that's
a real gap, but this is already a last-resort path for an already-failed
message, and I'd rather flag that honestly than add a second layer of retry
logic to groundwork code before either service exists.

**The concrete values, given what you agreed to:**

```go
jetstream.ConsumerConfig{
    Durable:   "depot-stores-projector",   // "baskets-stores-projector" for the other
    AckPolicy: jetstream.AckExplicitPolicy,
    AckWait:   5 * time.Second,
    MaxDeliver: 5,
    BackOff: []time.Duration{
        5 * time.Second,
        15 * time.Second,
        30 * time.Second,
        60 * time.Second,
    },
    // FilterSubject left empty — consumes the whole stores stream
}
```

`len(BackOff)` is 4, `MaxDeliver` is 5 — that's deliberate: attempt 1
(immediate) + 4 backed-off retries = 5 total attempts, so every backoff slot
actually gets used and nothing is inconsistent between the two numbers.

**Two open questions before I finalize this:**

1. **Dead-letter subject convention** — I'd propose
   `<service>.deadletter.<original-subject>` (e.g.
   `depot.deadletter.stores.product.ProductAdded`), keeping the original subject
   visible for triage. Reasonable, or did you have a different shape in mind?
1. This decision (Nak-vs-NakWithDelay, MaxDeliver/BackOff pairing, the
   dead-letter pattern) has real system-wide consequences for every future
   consumer — worth its own ADL entry (0013) once we close out this Step, same
   as ADL 0012 was written for the `Service` interface. Agreed, or fold it into
   `journey_progress.md` only?

### Step 3 — Scaffold `baskets/go.mod` and `depot/go.mod`,

Good — clean container, clean health check. On to **Step 3**.

Since `stores/go.mod`'s `module` line was already decoupled from its folder name
(per ADL 0014's reasoning), I'll keep that same convention here: each module is
named after the _service_, not the project folder that happens to contain it —
`baskets`/`depot`, not `depot-baskets/baskets`. This keeps things consistent if
either service ever needs to move to its own folder later (unlikely, but the
naming shouldn't depend on that not happening).

One thing that does change from `stores/go.mod`, though: **the relative path in
the `replace` directive.** `stores/` sits directly under the workspace root, one
level up from `foundation/` — hence `../foundation`. `baskets/` and `depot/` sit
_inside_ `depot-baskets/`, two levels down — so the correct relative path is
`../../foundation`. Worth being precise about now, since Step 4's Dockerfiles
for these two services will need to mirror this same depth relationship in their
`COPY`/`WORKDIR` layout for `go mod download` to resolve `foundation` locally
inside the build context — flagging that dependency now so it isn't a surprise
when we get there.

```go
// depot-baskets/baskets/go.mod
```

```go
// depot-baskets/depot/go.mod
```

I've deliberately kept these minimal — just the `foundation` dependency, nothing
else declared yet (no `pgx`, `connectrpc`, `nats.go`, `protobuf`).
`stores/go.mod` accumulated those because real code was already importing them;
here there's no code yet to justify pinning versions for dependencies that
aren't used. Phase 3/4 will pull those in naturally via `go get` as
domain/persistence/API code actually gets written — same "prove it needed before
adding it" instinct as `foundation`'s own "prove it twice before extracting"
discipline.

```go
// go.work
```

Once these three files are in place, `go build ./...` from the workspace root is
the right check — it should succeed trivially (there's no actual Go code in
either new module yet, so there's nothing to compile), but it does confirm the
workspace parses correctly and both new modules resolve `foundation` via the
local `use` mapping without hitting the network for that dependency at all.

Let me know how that goes, and once confirmed we'll move to **Step 4**: empty
`main.go` service skeletons that actually start up, connect to Postgres/NATS,
and shut down cleanly via the `Service` interface — wired into
`docker-compose.yaml`.

That's a known quirk of `go build`'s pattern matching in workspace mode, not a
problem with the `go.work` file itself — the root directory (`.`) isn't itself a
module (no `go.mod` there, by design — it's just the workspace root), and the
`./...` pattern needs to resolve _starting from_ a directory that's either a
module itself or listed in `go.work`'s `use` block. Running the bare pattern
from outside any module trips this specific error.

The fix is just to be explicit about which module trees to build, rather than
relying on `./...` to expand across all of them from the root:

```bash

```

That should succeed — `foundation`, `foundation-smoketest`, and `stores` compile
real code as before, and the two new modules should report success trivially (no
`.go` files yet means "nothing to build" there, not an error, since a `go.mod`
with no source files is a valid empty module).

If you'd rather not type that full list every time as more services get added,
the other common workaround is running `go vet ./...` (or `build`) from _inside_
each module directory individually — but given this project already treats
`Makefile` as the durable, growing interface for exactly this kind of repeated
command (per `system-state.md`'s own description of it), it's probably worth a
`verify`-style target there once we're past groundwork and have real code to
check across every module at once. Not urgent for this Step — just flagging it
as a natural fit for later rather than solving it now.

Let me know how that explicit build goes, and we'll move to Step 4.

### Step 4a — Postgres provisioning + .env for baskets

```bash
#!/bin/bash
```

A straight copy of `03-stores-db.sh`'s shape, just the database name changed —
`eda_dev`/`eda_user` are instance-wide roles (created once in `02-roles.sh`, not
per-database), so no role changes are needed here, only the new database and its
grants.

```env

```

I added `SRV_PORT=8081` explicitly (rather than leaving it at `config.go`'s
`8080` default) since you'll likely want to run `baskets` locally via bare
`go run` (mirroring `stores/run`) _alongside_ `stores` at some point — both
defaulting to `8080` would collide the moment they're both running on your host
at once. `DB_PASSWORD` reuses the same value as `stores/.env`, since `eda_user`
is one shared role across every database on this Postgres instance, not a
per-service credential.

**One real gotcha to flag now, before you run anything** — this is the exact
trap ADL 0006 already documented and named explicitly:
`docker-entrypoint-initdb.d/` scripts (which is where `04-baskets-db.sh` needs
to be mounted) **only run on first-time `initdb` bootstrap**. Your
`eda-postgres` container has already been initialized — `pg-data` is a populated
volume at this point, not a fresh one. That means simply adding this script and
running `docker compose up` (even with the volume mount added in Step 4g) will
**not** cause it to execute — `initdb` already happened, so Postgres skips the
whole directory on every subsequent start, no error, no log line, just silently
never runs.

Per ADL 0006's own documented recovery path, you have two options once we get to
Step 4g and the mount is actually in place:

- **Manually replicate what the script does** — connect as `edasuper` (or via
  `docker compose exec postgres psql -U edasuper -d edadb`) and run the same
  `CREATE DATABASE basketsdb OWNER eda_dev;` + grants block by hand. Fast, keeps
  your existing `storesdb` data intact.
- **Nuke and reinit the volume** (`docker compose down -v`, then `make up`) —
  lets every init script run fresh, including the original three plus these new
  ones. Only worth it if you don't mind losing whatever's currently in
  `storesdb`.

I'd lean toward the manual-replication route given you likely have real test
data in `storesdb` you don't want to lose over provisioning a sibling database —
but that's your call, and either way it's a Step 4g concern, not something to
act on yet since the mount itself isn't wired up.

Confirm this looks right, and I'll move to **4b** —
`baskets/cmd/service/main.go`'s HTTP skeleton and graceful shutdown wiring.
