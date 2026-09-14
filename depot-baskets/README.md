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

### Step 3 — Scaffold `baskets/go.mod` and `depot/go.mod`

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

**4b — `baskets/cmd/service/main.go`.**

Before the file itself, one small addition to `foundation/nats` I'd like to fold
in here rather than duplicate: ADL 0013 already named a fixed set of "standard
values" every future `Subscriber` should default to. Right now that's just prose
— nothing enforces it, and writing this `main.go` would mean retyping that exact
`ConsumerConfig` literal by hand, with `depot`'s `main.go` (4e) doing the same
thing again right after. A small helper makes ADL 0013 something the compiler
actually enforces, not just something a future author is trusted to remember
correctly:

```go
// foundation/nats/consumer.go
package foundationnats
func StandardConsumerConfig(durable string) jetstream.ConsumerConfig {}
```

Only `Durable` varies per call site; `FilterSubject` is deliberately left unset
(per your earlier call to consume the whole stream unfiltered). If a future
service genuinely needs different values, it constructs its own
`jetstream.ConsumerConfig` literal directly rather than calling this — the
helper is a convenience for the common case, not a wall preventing deviation.

Now `main.go` itself:

```go
// depot-baskets/baskets/cmd/service/main.go
```

A few things worth explaining before you run this:

**`nc.Close()` is a single `defer`, not called from two branches.** ADL 0012 was
specific that the connection gets closed "once, directly, by `main()` after
`Run()` returns" — `defer` right after a successful `Connect()` guarantees
exactly that regardless of whether `shutdown.Run` returns an error or `nil`,
without needing to duplicate the close call in both an error path and a success
path (which is itself a small bug risk — easy to add a second call site later
and forget the first still exists).

**`storesStreamName = "stores"`** — pulled directly from `CONTRACT.md`'s own
wording ("publish onto the `stores` JetStream stream"). Worth double-checking
against however `01-stores`' (now `stores`') actual `main.go`/`EnsureStream`
call names it, since I'm going off the contract document's prose rather than the
literal string constant in that service's source, which I don't have in front of
me. If the real stream name differs even slightly (casing, etc.),
`CreateOrUpdateConsumer` in `Subscriber.Run()` will fail immediately and loudly
— a good, fail-fast confirmation either way, not a silent mismatch.

**The placeholder handler always returns `nil`** — deliberately. It's proving
the wiring (message arrives, gets acked, `Run`/`Shutdown` behave correctly under
a second real listener), not exercising the backoff/dead-letter path. That path
already has its own ADL and can be tested for real once `depot`/`baskets` have
actual handlers with actual failure modes in Phase 3/4, per the testing note we
already added there.

Try `go build ./depot-baskets/baskets/...` first (should now report real
compiled packages, not the "matched no packages" warning from Step 3), and let
me know how it goes before you attempt to actually run it against the live
containers — running it will also need `NATS_NAME` etc. already correctly set in
`baskets/.env`, which you've already got from Step 4a.

#### EndureStream

**2. A real semantic bug underneath the syntax one: the `Subscriber` is wired to
the wrong stream.**

Look at what `StreamName` actually means in this file — it's defined right above
as `"baskets"`, `EnsureStream`'d as baskets' own _outgoing_ stream (for
`BasketCheckedOut` etc., once Phase 3 needs it), with subjects
`baskets.basket.*`. But the whole point of this `Subscriber` — per
`journey_progress.md`'s own description, "`Store`/`Product` cache projector
consuming `stores`' events" — is to consume the **`stores`** stream, the one
`01-stores` (now `stores`) already created and owns. Passing `StreamName`
(`"baskets"`) into `NewSubscriber` means it would try to attach a consumer to
`baskets`' own empty stream and wait forever for events that will never arrive
there — `stores`' events are on a completely different stream it never touches.

These two streams need to stay clearly distinct in this file — one you _ensure_
(yours), one you only _reference by name_ (someone else's, already ensured by
`stores`' own `main`/`module` at its own startup):

```go
// depot-baskets/baskets/module.go

```

```go
// depot-baskets/baskets/module.go

```

Worth being deliberate here since it's easy to get backwards: `baskets`
**ensures** `baskets` (it owns that stream — nobody else will create it), and
**only references** `stores` by name (it doesn't own that stream, isn't
responsible for its existence, and would get a clear, loud
`CreateOrUpdateConsumer` error at startup if `stores` hasn't ensured it yet — a
legitimate, if slightly fragile, implicit ordering dependency worth remembering
once `docker-compose.yaml`'s `depends_on` is wired in Step 4g).

#### Events Replay on first Durable Consumer

That flood of replayed events is actually the correct and expected behavior, not
a surprise bug — and it's genuinely useful to have witnessed it directly rather
than just read about it in the JetStream docs.

**Why it happened:** `baskets-stores-projector` is a _brand-new_ durable
consumer name — this is the very first moment it's ever existed. JetStream
consumers default to `DeliverAllPolicy` when no `DeliverPolicy` is explicitly
set (which ours doesn't), meaning a fresh durable consumer starts by replaying
**every retained message currently on the stream**, from the beginning, before
it ever catches up to "live." Since `stores`' stream has been accumulating
events since Phase 3/4 of that project's own testing, `baskets` — the moment it
first attached — received the entire history in one burst. That's JetStream's
durability guarantee working exactly as advertised, and it's a direct, concrete
demonstration of something ADL 0003 named as a reason to pick JetStream over
core NATS in the first place: durable, replayable streams. You just replayed one
for real.

This also matters for Phase 3: it confirms the "idempotent cache upsert"
requirement `journey_progress.md` already flagged isn't a theoretical nicety —
it's load-bearing from the very first startup. Any real handler will need to
treat a `StoreCreated` for an already-known store ID as a normal, expected case,
not an edge case, since a consumer restart or a redeployment could replay
history again under certain conditions (e.g. if the durable consumer itself were
ever deleted and recreated).

One consequence worth knowing now, before it confuses you later: **this replay
won't happen again** on subsequent runs of `baskets`. JetStream persisted
`baskets-stores-projector`'s delivery cursor server-side the moment those
messages were acked — next time you start `baskets`, it'll resume from where it
left off (live events only), not replay history again. If you ever want to force
a full replay again for testing, you'd need to delete that specific consumer
server-side (`nats consumer rm stores baskets-stores-projector`, or via the
JetStream API) — not something to do casually once real cache data depends on
it, but worth knowing it's possible.

Content-type fix — good call removing it rather than fixing the JSON body to
match; a bare `"ok"` text response for a health check doesn't need to masquerade
as JSON.

Two things left to actually close out 4c:

1. **The live single-event test** — the second curl command, so we see one new
   `StoreCreated` arrive on its own, distinct from the historical flood,
   confirming ongoing (not just historical) delivery works:

```bash
curl -X POST localhost:8080/stores.v1.StoresService/CreateStore \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"name": "Test Store", "location": "Test Location"}'
```

2. **The shutdown test** — `Ctrl+C` the running `baskets` process and paste the
   tail of the log output. I want to specifically see whether `Drain()`
   completing (via `cc.Closed()`) actually gates `Run()`'s return, or whether it
   returns instantly — that's the real, first-ever proof of ADL 0012's
   two-listener concurrent-shutdown-broadcast design against a genuine second
   `Service`, not the synthetic test harness from Phase 1.

#### Shutdown Sequence Race

Both results are exactly what should happen — and the second one (the shutdown
ordering) is genuinely interesting, worth a careful read rather than a quick
"looks fine."

**The live event** — clean, single `StoreCreated` arriving well after the
historical replay, confirming ongoing delivery works independently from the
backlog-catch-up we saw earlier. That's 4c's core proof point, done.

**The shutdown log** — read the ordering carefully, because there's a subtlety
here that looks like it could be a bug but isn't:

```
1. WARN  shutdown signal received
2. INFO  service goroutine has exited  service=BASKETS-HTTP
3. INFO  shutdown complete
4. INFO  service goroutine has exited  service=BASKETS-STORES-PROJECTOR
```

Notice: **"shutdown complete" (line 3) prints _before_ the subscriber's own
"goroutine has exited" (line 4).** At first glance that looks backwards — how
can shutdown be "complete" while a service's `Run()` goroutine hasn't logged its
own exit yet? It's not a bug, but it does expose something worth understanding
precisely, because it's a real property of this design, not an illusion.

Trace the actual causality: when `cc.Closed()` fires inside
`Subscriber.Shutdown(ctx)`, it calls `markDone()` (closing `s.done`) and then
returns `nil`. That single channel-close event has **two independent downstream
effects, on two different goroutines, with no ordering guaranteed between
them**:

- The goroutine running `Shutdown(ctx)` itself (spawned by `shutdown.Run`'s
  broadcast loop) proceeds past the `return nil`, its `defer wg.Done()` fires,
  and once _both_ services' `Shutdown` calls have done the same, `wg.Wait()`
  unblocks and `"shutdown complete"` gets logged — this is what we saw as line
  3\.
- Completely separately, the _original_ goroutine sitting on `<-s.done` inside
  `Run()` wakes up because that same channel just closed, returns `nil`, and
  only _then_ does its enclosing goroutine log `"service goroutine has exited"`
  — line 4.

Both are legitimate, correct consequences of the exact same event (`s.done`
closing) — but they're two different goroutines racing to log, and Go's
scheduler gives no guarantee about which one gets there first. `BASKETS-HTTP`
happened to log its exit _before_ "shutdown complete" purely because
`http.Server.Shutdown()` on an idle server returns almost instantly; the
subscriber's version of that same log line just happened to lose the race this
particular run. Run it again and you might well see the opposite ordering, or
all four lines in yet another sequence — it's non-deterministic, and that's
fine, because nothing downstream depends on this specific ordering. By the time
`"shutdown complete"` is logged, both `Shutdown()` calls have genuinely, fully
returned — which is the only guarantee `shutdown.Run`'s contract actually makes.
The stray `Run()`-exit log line arriving a beat later is just a cosmetic side
effect of logging from a goroutine that has nothing left to do at that point but
say so.

Worth being certain there's no real risk hiding here: `main()`'s
`defer nc.Close()` only executes once `shutdown.Run` itself returns to `main`,
which is _after_ `wg.Wait()` unblocks — so no chance of `nc.Close()` racing
against a subscriber goroutine that's still meaningfully touching the NATS
connection. By the time `cc.Closed()` fires, draining is already fully finished;
everything after that in `Run()` is just an inert channel receive and a log
line, with no further NATS interaction to race against.

This is worth a line in `journey_progress.md`'s eventual Phase 2 write-up — same
spirit as Phase 1's "concurrent vs. sequential shutdown broadcast" lesson:
**"shutdown complete" logging only proves every `Shutdown()` call returned, not
that every `Run()` goroutine has finished logging its own exit — those are two
independent happens-before chains sharing one root cause, not one ordered
sequence.** Good one to have actually witnessed rather than just accepted from
the ADL's prose.

**4c is done** — HTTP, Postgres, NATS, historical replay, live delivery, and
graceful shutdown all confirmed against a real running process. That's
`depot`/`baskets` no longer relying on Phase 1's synthetic test alone; the real
two-listener contract is now proven end-to-end.

Ready for **4d–4f** (the same sequence for `depot`) — given how closely
`baskets` and `depot` mirror each other structurally, would you rather I draft
`depot`'s `module.go`/`main.go`/`http-health.go` directly (adapted for `depot`'s
own naming — `depot-stores-projector`, `depotdb`, port `8082`, etc.), or would
you prefer to write the first pass yourself this time and have me review it the
way we did with `baskets`?

### Step 4g - Dockerfiles

Good — same shutdown ordering pattern as `baskets` (HTTP goroutine exit, then
"shutdown complete," then subscriber goroutine exit, all racing the same way for
the same reason). Consistent, expected, no new bugs there.

Here's **4g**.

**Dockerfiles** — same two-stage shape as `stores`', with paths adjusted for the
nested `depot-baskets/` location. Remember the go.mod `replace` directive is
`../../foundation` (two levels up), so the container's copy structure needs to
preserve that same relative depth:

```dockerfile

```

Both `EXPOSE 8080` — correct and consistent with `stores`' now-fixed value,
since this is the container-_internal_ port; distinguishing
`baskets`/`depot`/`stores` on the host side is `docker-compose.yaml`'s job via
the `ports:` mapping, not the Dockerfile's.

**One thing worth catching before it becomes a confusing runtime surprise:**
`baskets/.env` and `depot/.env` explicitly set `SRV_PORT=8081`/`8082` —
correctly, for _local_ `make baskets/run`/`make depot/run`, where all three
services share one host network namespace and would collide on `8080` otherwise.
But inside a container, each service is alone in its own network namespace —
there's no collision to avoid, and the Dockerfile's `EXPOSE 8080` plus the
compose `ports:` mapping already assumes the process listens on `8080`
internally. Left unoverridden, `env_file` would carry `SRV_PORT=8081` straight
into the container, the process would bind `:8081`, and the compose `ports:`
mapping (`8081:8080`) would point at a port nothing's listening on. So both
service blocks need an explicit `SRV_PORT: '8080'` override in `environment:`,
the same way `stores` already overrides
`DB_HOST`/`DB_PORT`/`NATS_URL`/`LOG_FILE_PATH` — for exactly the same reason:
values in `.env` are tuned for bare-metal local iteration, and the container
needs its own, separately-correct set.

**`docker-compose.yaml` additions:**

```yaml
# docker-compose.yaml — postgres service, volumes: list
```

```yaml
# docker-compose.yaml — new service blocks, alongside stores
```

`depends_on: stores: condition: service_started` gives correct start _ordering_
(compose won't even attempt to start `baskets`/`depot` until `stores`' container
process has launched) — it doesn't guarantee `stores` has finished
`EnsureStream`-ing by that point, which is the race you've already confirmed is
acceptable for now. Worth a one-line note in `journey_progress.md` alongside
that decision so it isn't accidentally rediscovered as a "new" concern in Phase
5's integration pass.

Before running `make build`/`make up` against all five containers together: same
volume caveat as before applies to the two new init script mounts — they're
correctly wired for any _future_ full reinit, but since `pg-data` is already
initialized, they won't execute against your current running instance (your
manually-provisioned `basketsdb`/`depotdb` grants already stand in for that).
Nothing to do about that now, just don't be surprised if you don't see them fire
in the postgres container's logs.

Try `make build` then `make up` then `make ps`, and confirm all five containers
(`postgres`, `nats`, `stores`, `baskets`, `depot`) report healthy/running, plus
a `/health` curl against `8081` and `8082` — that's the real proof this step is
done, and closes out Phase 2 entirely.

### **Phase 3 — `baskets`**

1. `Basket`/`Item` domain aggregate — _in progress, this Step_
1. Domain tests (`basket_test.go`) — mirroring `store_test.go`'s coverage:
   constructor validation, each guard/error path, event emission per transition
1. `events` package — envelope/payload/subject/stream for `Basket`'s five
   published events, following `stores/internal/events`' exact shape
1. `catalog` package — plain `Store`/`Product` read-model structs + repository
   interface (per our earlier decision: separate from `domain`, no invariants,
   writer is the projector, reader is `application`)
1. `postgres` package — `BasketRepository`
   (`WithLock`/`FindByIDForUpdate`/`Save`/`FindByID`, same pattern as
   `stores_repository.go`) and `CatalogRepository` (upsert-oriented, simpler —
   no `WithLock` needed since projections aren't user-driven concurrent writes)
1. `application` layer — `StartBasket`, `AddItem`, `RemoveItem`, `Checkout`,
   `CancelBasket`, `GetBasket` use cases, plus the real catalog-projection
   handler replacing `bsk_module.go`'s current `placeholderHandler`
1. `connectapi` — proto definition, RPC server wiring, error mapping (404/400,
   per `stores`' convention)
1. Integration & close-out — live verification against real
   `stores`/Postgres/NATS, `CONTRACT.md`, `system-state.md` update

This is `baskets` only — `depot`'s `ShoppingList` is still Phase 4, unchanged
from `journey_progress.md`'s original outline.

Does this breakdown look right, or is there a Step you'd split further, merge,
or reorder before I fix the collision above and we close out Step 1?

### STep 1

Here's `basket_test.go`, following `store_test.go`'s structure — one `Test*`
function per aggregate method, `t.Run` subtests for each success/error path, and
one shared constructor helper:

```go
// baskets/internal/domain/basket_test.go
```

Coverage mirrors `store_test.go`'s density: every guard branch in `basket.go`
has a corresponding test (blank ID isn't tested since `NewBasket` doesn't
validate it — same as `Store`, which only validates `name`/`location`, not `id`,
since ID validity is the caller's/`connectapi`'s concern per ADL 0009).

One thing to flag before you run it: `RemoveItem`'s quantity-decrement test and
the "checked out" test both call `Checkout()` internally to reach that state,
which means those specific subtests are silently depending on `Checkout` already
working correctly — if `Checkout` itself had a bug, it could mask or distort a
`RemoveItem` test's real failure. `store_test.go` has this same characteristic
(its participation tests depend on `EnableParticipation` to reach the "already
enabled" state), so it's consistent with the existing convention, not a new risk
— just worth knowing about if a `RemoveItem` failure ever looks confusing at
first glance.

Drop this alongside `basket.go`/`events.go`/`basket_repository.go` and run
`go test ./internal/domain/...` from `depot-baskets/baskets`. Let me know what
comes back.

### Step 2

Here's the `events` package for `baskets`, following the four-file split
(`envelope.go`, `payload.go`, `subject.go`, `stream.go`):

```go

```

```go

```

```go

```

Two things worth flagging rather than silently carrying forward.

First, `stores/internal/events/envelope.go`'s `NewEnvelope` has a real bug you
may want to backport a fix for: `data, err := json.Marshal(payload)` is followed
by `if err != nil { }` — an empty block that swallows the error and falls
through to construct the `Envelope` anyway, with `Data` left as whatever
partial/zero value `json.Marshal` returned. I did _not_ replicate that here —
`baskets`' version above returns a wrapped error immediately, matching the
`Marshal` method right below it in the same file, which does handle its own
error correctly. Every domain event type here marshals cleanly today (no
channels, functions, or cyclic references), so this was never going to fire in
practice — but it's a real latent bug in `stores`, not a style choice, and worth
a one-line fix there independent of this project if you want `stores`'
`CONTRACT.md`-facing behavior to actually match what the code guarantees.

Second, every aggregate-lifecycle event (`BasketStarted`, `BasketCheckedOut`,
`BasketCanceled`) gets an empty `struct{}` payload, same as
`StoreParticipationEnabled` did _not_ do in `stores` — `stores` always included
at least the resulting state (`Participating: true`) even when arguably
redundant with the event name itself. I left these three empty because there's
no equivalent "resulting state" to echo — `Status` isn't part of these events'
identity, `AggregateID`/`EventName` already say everything a consumer needs (a
basket started, or reached checkout, or was canceled). Worth a gut-check: does
`ordering` (Phase 3, next project) need anything else out of `BasketCheckedOut`
beyond "this basket ID checked out"? If it needs the full item list at time of
checkout, this payload is too thin and needs revisiting before Phase 4/5
close-out — not urgent now, but worth naming before we're several Steps past it.

Drop these four files into `depot-baskets/baskets/internal/events/`, run
`go build ./...` from `depot-baskets/baskets` to confirm it compiles against the
domain package. Let me know how it goes.

### Step 3

Here's the `catalog` package — plain structs, no constructors with invariants,
no events, matching the "read-model only" decision from before:

```go
// baskets/internal/catalog/store.go
```

```go
// baskets/internal/catalog/product.go
```

```go
// baskets/internal/catalog/repository.go
```

A few choices worth explaining rather than leaving implicit.

Types are named `Store`/`Product` (not `CatalogStore`/`CatalogProduct`) — since
the package itself is called `catalog`, callers reference them as
`catalog.Store`/`catalog.Product`, and prefixing the type name too would be Go's
classic stutter anti-pattern (`catalog.CatalogStore`). This is different from
how `domain.Store` reads in `stores`, but that's just because `stores`' own
domain package is generically named `domain`, not `store` — the naming
convention is actually consistent once you account for the package name doing
the qualifying work either way.

`Repository` has no `WithLock`/`FindByIDForUpdate`, unlike `BasketRepository` —
this was already anticipated in the Step 5 line of our Phase plan (upserts are
idempotent by construction, and the projector is the only writer, so there's no
concurrent-write race to guard against the way `EnableStoreParticipation`'s
read-modify-write on `Store` needs to).

`FindProductByID` is the only read method, and there's deliberately no
`FindStoreByID` yet — since your validation answer was "reject if missing from
cache" only (no participation check), nothing in this Phase actually needs to
_read_ a cached `Store` back out. `UpsertStore` still exists on the write side,
because the projector needs somewhere to put
`StoreCreated`/`StoreRebranded`/`StoreParticipationEnabled`/`StoreParticipationDisabled`
events regardless of whether anything reads them yet — leaving that data on the
floor would just mean redoing this Step later once `FindStoreByID` does become
necessary (e.g. if participation-checking gets added in Phase 4/5). This is the
same "build the write side now, defer the read method until something needs it"
discipline ADL 0008 used for `stores`' own query layer.

One thing I did **not** write: a `catalog_test.go`. Every `domain` file so far
had a matching test because it had actual behavior to verify — constructors that
validate, methods that enforce invariants and emit events.
`catalog.Store`/`catalog.Product` are plain data with no methods at all; there's
nothing here a unit test could meaningfully exercise beyond "does this struct
have the fields I just declared it to have." The real verification for this
package happens later, once the Postgres implementation and the projector
handler exist — that's where an upsert either round-trips correctly or it
doesn't.

Does that match what you had in mind, or is there a read method / field you'd
want added now rather than deferred? Once confirmed, drop these three files into
`depot-baskets/baskets/internal/catalog/` and run `go build ./...` to confirm it
compiles standalone (it has no dependency on `domain` at all, so this should be
a trivial build).

### Application Layer

Ported the retry/publish helpers, with import paths swapped from
`stores/internal/*` to `baskets/internal/*` — otherwise unchanged, since the
retry-window/backoff/timeout logic ADL 0007 established doesn't need to differ
per service:

```go
// baskets/internal/application/publish_retry.go
```

```go
// baskets/internal/application/publish_event.go
```

Now the six use cases:

```go
// baskets/internal/application/start_basket.go
```

```go
// baskets/internal/application/add_item.go
```

```go
// baskets/internal/application/remove_item.go
```

```go
// baskets/internal/application/checkout.go
```

```go
// baskets/internal/application/cancel_basket.go
```

```go
// baskets/internal/application/get_basket.go
```

That's five command use cases plus one query use case (`GetBasket`, thin per ADL
0008 — no repo write, no publish, exactly the shape `GetStore`/`ListProducts`
established). `AddItem` is the one genuinely new shape: it's the only use case
in either service so far that takes a dependency on _two_ repositories, and it's
the concrete result of the Option 2 decision from last turn — the catalog
`FindProductByID` call runs inside `txCtx`, the same transaction
`FindByIDForUpdate`/`Save` use, all three resolving to the identical `pgx.Tx`
via `executorFrom`.

Before I write the catalog-projection handler (replacing `bsk_module.go`'s
placeholder), there's a real design question I haven't raised yet: that handler
decodes _`stores`' own_ published events, not `baskets`' own. Per ADL 0004,
`baskets` never imports `stores/internal/events` or `stores/internal/domain` —
the only thing that travels forward is `stores/CONTRACT.md`. So the
envelope-decoding and payload-shape code the projector needs has to be written
fresh, locally in `baskets`, based on what `CONTRACT.md` documents `stores`
publishes — not shared code, even though it'll look almost identical to
`stores`' own `events` package by construction.

#### CatalogRepository fix

Good — the fix is genuinely narrow. Here's the migration, the interface change,
the repository update, and the projector.

```sql
-- baskets/migrations/000003_add_catalog_products_seq_guard.up.sql
ALTER TABLE catalog_products
    ADD COLUMN last_applied_seq BIGINT NOT NULL DEFAULT 0;
```

```sql
-- baskets/migrations/000003_add_catalog_products_seq_guard.down.sql
ALTER TABLE catalog_products
    DROP COLUMN last_applied_seq;
```

`last_applied_seq` only needs to exist on `catalog_products` — every
`catalog_stores` write is already an absolute `UPDATE`/upsert, so only
`AdjustProductPrice` needed the guard. Defaulting to `0` means the first-ever
price adjustment for a product (any real NATS stream sequence number starts at
`1` and only increases) always applies, with no separate initialization needed
on `CreateProduct`.

```go
// baskets/internal/catalog/repository.go — AdjustProductPrice signature updated
```

```go
// baskets/internal/postgres/catalog_repository.go — AdjustProductPrice replaced
```

`RowsAffected() == 0` is genuinely ambiguous between two different causes —
either this exact sequence number was already applied (the redelivery case we're
guarding against), or the product doesn't exist at all (an out-of-order
`ProductPriceIncreased` arriving before `ProductAdded` has landed, which
shouldn't happen given single-subject ordering, but the `WHERE` clause can't
distinguish the two). `ErrPriceAdjustmentSkipped` deliberately doesn't try to
tell them apart — the projector handler below treats it as a benign no-op either
way, logged but not retried, since retrying a stale redelivery would just hit
the same guard again.

```go
// baskets/internal/catalog/project.go
```

A few things worth being explicit about.

The event name strings (`"StoreCreated"`, `"ProductPriceIncreased"`, etc.) are
**literal string constants written directly in this switch**, not references to
`domain.StoreCreatedEvent`-style constants — because `baskets` has no such
constants to reference; those live in `stores/internal/domain`, which ADL 0004
says never crosses the boundary. This is a real, if narrow, drift risk: if
`stores` ever renames one of these event strings, nothing in `baskets` would
fail to compile — it would just silently stop matching, fall into `default`, log
a warning, and skip the projection. `CONTRACT.md` is the only thing keeping
these two services' understanding of the string in sync, which is exactly the
discipline ADL 0004's Consequences section already named ("`CONTRACT.md` needs
to be kept honest and current... or downstream projects will be built against a
stale contract").

`msg.Metadata()` is only called in the price-delta branch, not unconditionally
at the top — every other case is naturally idempotent and has no use for the
sequence number, so there's no reason to pay for the call (or handle its error)
on every message.

The `default` case logs and returns `nil` (ack, not retry) rather than treating
an unrecognized event name as a handler failure — an unknown event isn't a
transient problem `NakWithDelay`'s backoff would ever resolve; retrying it five
times and dead-lettering it would just be noise for something that's actually a
contract-drift bug, not a processing failure.

Last piece for this Step: wiring `Project(catalogRepo, logger)` into
`bsk_module.go`, replacing `placeholderHandler`, and constructing all six use
cases + both repositories there. Want that now, or do you want to review this
batch (repository, migration, projector) first?

### Wiring to Module

Here's the fully wired `bsk_module.go`:

```go
// baskets/internal/module.go (formerly bsk_module.go)
```

A few things changed or removed, worth confirming rather than assuming you'd
want silently.

`placeholderHandler` is gone entirely — it's replaced by
`catalog.Project(catalogRepo, logger)`, so there's no more log-only no-op; every
`stores` event landing on this consumer now actually mutates
`catalog_stores`/`catalog_products`.

I resolved the double-import oddity from the original file
(`foundation/postgres` was imported twice, once as `foundationpostgres`, once
bare as `postgres`) down to a single `foundationpostgres` alias, since the local
`postgres` name is now needed for _this_ project's own `internal/postgres`
package (`BasketRepository`/`CatalogRepository`) — the original double-import
would now collide with that. This wasn't a bug exactly (Go permits importing one
path under two names in the same file), but it's no longer just redundant, it's
actually unusable now that `baskets/internal/postgres` needs the plain
`postgres` name for itself.

Both repositories are constructed by passing `pool` (a `*pgxpool.Pool`) directly
wherever an `Executor` is expected — this works because `pgxpool.Pool` already
implements every method `postgres.Executor` requires
(`Exec`/`QueryRow`/`Query`/`Begin`), the same implicit-satisfaction pattern
`stores` relies on.

`connectapi.Server` and its wiring are still commented out, matching where
`stores` left it in earlier phases and where `bsk_cmd_srv_main.go` currently has
its RPC routes commented too — that's Step 7, not this one.

Run `go build ./...` and `go vet ./...` from `depot-baskets/baskets` — this is
the first build that has to successfully wire `domain`, `catalog`, `events`,
`postgres`, and `application` together into one working `Module`, so it's a
meaningful integration check even before any real container is involved. Let me
know what comes back.

### NATS retry on fail policy

Confirmed, and more precisely than I expected: `Messages: 4`,
`First Sequence: 1`, `Last Sequence: 4` — no unaccounted gaps, no extra control
messages. Every one of the 4 slots is a real, stored `BasketStarted` publish.
Test 1 accounts for exactly one (`seq=1`). Test 2's single `Execute` call —
which should have produced exactly one message — accounts for the other three
(`seq=2,3,4`). The duplicate-publish theory is confirmed directly, not just
inferred from timestamps.

And there's something even better in that same output:
**`Duplicate Window: 2m0s`**. JetStream streams ship with a built-in
idempotent-publish mechanism specifically for this exact ambiguity — a publisher
attaches a `Nats-Msg-Id` header to a message, and if the _same_ message ID
arrives again within the configured window (2 minutes, here — the default),
JetStream recognizes it as a duplicate and returns the _original_ message's ack
instead of storing a second copy. This stream already has that window
configured. Our code just never uses it — `Publisher.Publish` calls
`p.js.Publish(ctx, subject, payload)` with no message ID at all, so every retry
attempt looks like a brand-new, distinct message to JetStream, deduplication
window or not.

This is the correct fix, not a workaround — forcibly disabling `nats.go`'s
reconnect buffer would stop _this specific_ symptom, but it wouldn't solve the
underlying ambiguity (a publish can still land server-side while the client, for
any number of reasons, never sees the ack). Message-ID-based idempotency solves
the actual problem: retries of the _same logical event_ converge to one stored
message, regardless of why the client couldn't confirm the earlier attempt.

Three things need to change, and the most important one is **not
`baskets`-local**:

```go
// foundation/nats/publish.go — Publish signature gains a msgID parameter
func (p *Publisher) Publish(
    ctx context.Context,
    subject string,
    payload []byte,
    msgID string) (
*jetstream.PubAck, error) {

    ack, err := p.js.Publish(ctx, subject, payload, jetstream.WithMsgID(msgID))
    if err != nil {
        return nil, fmt.Errorf("publishing to %q: %w", subject, err)
    }

    return ack, nil
}
```

```go
// baskets/internal/application/publish_retry.go
// — msgID threaded through, same on every attempt
func publishWithRetry(
    ctx context.Context,
    publisher *foundationnats.Publisher,
    logger *slog.Logger,
    subject string,
    payload []byte,
    msgID string)
(*jetstream.PubAck, error) {
    ...
    ack, err := publisher.Publish(attempCtx, subject, payload, msgID)
    ...
}
```

```go
// baskets/internal/application/publish_event.go
// — one msgID generated per logical call, reused across all retries
import (
    ...
    "github.com/google/uuid"
)

func publishDomainEvent(
    ctx context.Context,
    publisher *foundationnats.Publisher,
    logger *slog.Logger,
    event domain.Event) error {
    ...
    msgID := uuid.NewString()

    ack, err := publishWithRetry(ctx, publisher, logger, subject, payload, msgID)
    ...
}
```

The critical detail: `msgID` is generated **once**, before entering
`publishWithRetry`'s loop, and the exact same value is passed on attempt 1, 2,
and 3. That's what makes JetStream see three arrivals of the _same_ ID rather
than three different ones — a fresh UUID per attempt would defeat the whole
mechanism.

Three consequences worth flagging, not just the code:

`foundation/nats.Publish`'s signature change is a breaking change to shared
code. `stores`' own `publishWithRetry`/`publishDomainEvent` calls
`Publish(ctx, subject, payload)` with the old three-argument shape — this won't
compile against `stores` until its local copy is updated the same way. Given
`stores` has the exact same client library, the exact same missing-msg-ID gap,
and (per your earlier NATS-restart test) the exact same retry-during-outage code
path, it almost certainly has this identical duplicate-publish bug, just never
actually verified via sequence-number inspection the way we just did here.

ADL 0009 stated flatly that `uuid.NewString()` in the `connectapi` handler is
"the only place in the codebase that currently needs the
`github.com/google/uuid` dependency; the `application` and `domain` layers have
no reason to import it." That's no longer true — `application` now needs it too,
for message IDs, which is a different purpose than aggregate-ID generation but
the same dependency. Worth a note in that ADL's Consequences (or a new one), not
silently invalidating what it said.

This is a system-wide correctness fix discovered mid-`baskets`, affecting shared
`foundation` code and (almost certainly) `stores` too — squarely the kind of
decision `roadmap.md`'s own working agreement says gets an ADL entry, not just a
fix buried in a progress log.

### Step 7 -- ConnectAPI Layer

Before writing `baskets`' own version, there's a real, silent bug in
`stores/internal/connectapi/errors.go` worth fixing rather than carrying
forward: those four `errors.Is(...)` lines inside the `case` block —

```go

```

— are standalone expression statements, not part of the `case`'s condition list
above them. Go happily compiles this (calling a function and discarding its
return value is legal), but it means these four lines do _nothing at all_.
`ErrProductNameIsBlank`, `ErrProductPriceIsNegative`, and `ErrNotAPriceDecrease`
(checked twice, redundantly, itself a sign this was a copy-paste-in-progress
edit) never actually match this `case` — they fall straight through to `default`
and get mapped to `CodeInternal` (500) instead of the intended
`CodeInvalidArgument` (400). A client sending a blank product name or a negative
price currently gets told "internal server error" instead of "bad request,"
which is exactly backwards from what `CONTRACT.md`'s 400/404 convention
promises. The fix is just moving those four onto the `case` line itself,
comma-separated like the others:

```go
// stores/internal/connectapi/errors.go — corrected case
```

Now the proto file for `baskets`, following `CONTRACT.md`'s own request/response
shape convention (thin, one field per concern, server-generated IDs never
accepted from the client per ADL 0009):

```proto

```

`StartBasketRequest` is empty — no fields at all — since the basket ID is
server-generated (ADL 0009) and nothing else about starting a basket needs input
from the caller. That's a genuinely different shape from every one of `stores`'
`Create*` requests, which all take at least a `name`/`location`; `Basket` has no
such initial data to supply.

You'll need to run this through your `buf generate` pipeline (per ADL 0003) to
produce `basketspb`/`basketspbconnect` before any of the Go code below compiles.

```go
// baskets/internal/connectapi/server.go
```

```go
// baskets/internal/connectapi/proto.go
```

```go
// baskets/internal/connectapi/errors.go
```

```go
// baskets/internal/connectapi/basket.go
```

And the wiring — `module.go` gains the `Server`, `main.go` uncomments the route:

```go
// baskets/internal/module.go — Server field added, constructed in New
type Module struct {
    ...
    ConnectServer *connectapi.Server
}
```

```go
// baskets/internal/module.go — inside New, after the use cases are built
```

```go
// baskets/cmd/service/main.go — RPC route uncommented
path, connectHandler := basketspbconnect.NewBasketsServiceHandler(mod.ConnectServer)
router.Handle(path, connectHandler.ServeHTTP)
```

`quantity` in the proto is `int32`, requiring the explicit
`int32(item.Quantity)` cast in `toProtoBasket` — `domain.Item.Quantity` is a
plain Go `int`, which `stores` never had to convert since none of its domain
fields needed a numeric proto type at all (`Store`/`Product` are all strings,
bools, and `float64`, which map directly). This is a small, easy-to-miss thing
worth having flagged rather than let silently compile-error on you.

Run `buf generate`, then `go build ./...` and `go vet ./...` from
`depot-baskets/baskets` once the generated `basketspb`/`basketspbconnect`
packages exist. Let me know what comes back — and whether you want the `stores`
errors.go fix applied there now too, or left as noted for you to patch directly.

#### Testing RPC endpionts

Nice — that closes Step 7 clean, no errors on generation or build. That's every
layer of `baskets` now built: `domain` → `events` → `catalog` → `postgres` →
`application` → `connectapi`, plus the ADL 0015 fix backported to both services.

Before Step 8, there's a scoping question worth settling.
`journey_progress.md`'s own Phase 5 (the _overall_ project close-out, covering
both `baskets` and `depot`) already promises "end-to-end event flow checked
(create a store/product → confirm both caches pick it up independently)" — that
specific check needs `depot` to exist too, which it doesn't yet (Phase 4 hasn't
started). So I don't think we should try to do _that_ full check right now. What
we can meaningfully verify today is `baskets` alone, live, talking to a real
`stores` that's already running — proving the one cache, not both.

Concretely, with all containers up (`make up`, confirming
`stores`/`baskets`/`postgres`/`nats` are healthy via `make ps`), here's a real
end-to-end smoke test that exercises the entire chain we've built this whole
Phase — real `stores` event, real projector consumption, real catalog
validation, real basket mutation:

```bash
curl -s -X POST http://127.0.0.1:8080/stores.v1.StoresService/CreateStore \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"name": "Smoke Test Grocer", "location": "1 Smoke Ave"}'
```

That returns a `store_id` — grab it, then add a product to it:

```bash
curl -s -X POST http://127.0.0.1:8080/stores.v1.StoresService/AddProduct \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"store_id": "<STORE_ID_FROM_ABOVE>", "name": "Sourdough Loaf", "description": "Fresh baked", "sku": "SKU-SMOKE-1", "price": 5.00}'
```

That returns a `product_id`. Give the projector a second or two to actually
consume `ProductAdded` off the `stores` stream (it should be near-instant, but
it's a real async hop, not a synchronous call), then confirm the catalog cache
actually picked it up — either by querying `basketsdb` directly:

```bash
make basketsdb/dev
```

```sql
SELECT * FROM catalog_products WHERE sku = 'SKU-SMOKE-1';
```

— or by skipping straight to the real test, which is whether `baskets` itself
now accepts that product:

```bash
curl -s -X POST http://127.0.0.1:8081/baskets.v1.BasketsService/StartBasket \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{}'
```

```bash
curl -s -X POST http://127.0.0.1:8081/baskets.v1.BasketsService/AddItem \
  -H "Content-Type: application/json" \
  -H "Connect-Protocol-Version: 1" \
  -d '{"basket_id": "<BASKET_ID_FROM_ABOVE>", "product_id": "<PRODUCT_ID_FROM_ABOVE>"}'
```

If that `AddItem` call succeeds, it's a genuinely meaningful result — it means a
product that only ever existed because `stores` published an event, consumed
entirely asynchronously by `baskets`' own NATS consumer, projected into
`baskets`' own Postgres, was then validated by `AddItem`'s catalog check _inside
a live transaction_ and accepted. That's the whole Phase 3 thesis proven
end-to-end, not just each layer's unit tests passing in isolation.

One negative case worth checking too, since it's the one we can't get from any
earlier test: try `AddItem` with a `product_id` that was never created in
`stores` at all (a made-up UUID) — it should come back `CodeNotFound` per
`mapError`, not `CodeInternal`, confirming the ConnectRPC error-mapping layer is
wired correctly end-to-end, not just correct in the Go source.

Want to run through this now and report back what you get, or is there a
different close-out shape you'd rather do first?

#### Script Testing with http-connect.sh

```bash

```

A few things worth explaining about the shape of this script, since it's not
just a mechanical port of the `stores` version.

**The `sleep 2` after the two `AddProduct` calls is the one genuinely new
element** — nothing in the `stores` script needed it, because every one of its
calls is synchronous end-to-end (the RPC handler itself waits on the use case,
which waits on Postgres/NATS directly). Here, the path from "product exists in
`stores`" to "product exists in `baskets`' catalog cache" goes through an
entirely asynchronous hop — `stores` publishes, `baskets`' `Subscriber` consumes
it off the stream on its own schedule — so there's a real, if normally
sub-second, race between this script moving on to `AddItem` and the projector
actually finishing its upsert. Two seconds is a generous margin for a local
Docker setup; if `AddItem` intermittently fails with `NotFound` on the very
first product but not the second, that's the race showing up, not a real bug —
worth distinguishing from an actual regression if it happens.

**The two-product, increment-then-decrement sequence isn't just padding** — it's
deliberately exercising `AddItem`'s quantity-merge behavior (adding `PRODUCT_ID`
twice should read back as `quantity: 2`, not two separate line entries) and
`RemoveItem`'s decrement-not-delete path (removing it once should drop it to
`quantity: 1`, still present, not gone) against real Postgres rows — both were
verified in the application-layer test suite already, but only there; this is
the first time either goes through the full HTTP → ConnectRPC → `application` →
Postgres chain.

**Four negative cases, not one**, each targeting a different layer of
`mapError`: `CancelBasket` on an already-`checked_out` basket (a domain guard
rejecting a state transition), `AddItem` against a real UUID `stores` never
created (the catalog-miss path, `CodeNotFound`), a blank `productId` (domain
validation, `CodeInvalidArgument`), and `Checkout` on a basket with zero items
(a different `CodeInvalidArgument` path than the blank-productId one, confirming
both guards map correctly, not just one of them by coincidence).

One thing this script _can't_ verify, worth naming rather than letting it look
covered: `AdjustProductPrice`'s sequence-guard fix (ADL 0015's price-delta
idempotency) never gets exercised here, since neither `IncreaseProductPrice` nor
`DecreaseProductPrice` gets called against the store/product this script
creates. If you want that path checked too, it'd need `stores`' own price
endpoints called against `PRODUCT_ID` before the `sleep`, then a direct
`catalog_products` query (`make basketsdb/dev`) to confirm the adjusted price
landed exactly once — happy to extend the script that way if you want it covered
now rather than left implicit.

### Test conclusion

Everything landed correctly this run — including the fix. `price: 6.5` matches
the exact `5.00 → +2.00 → -0.50 → 6.50` math, `last_applied_seq: 120` confirms
the guard column is populated with a real stream sequence rather than left at
its default, and the blank-`productId` case now correctly returns
`400 invalid_argument: "the product id cannot be blank"` instead of last time's
incorrect `404`. All ten scenarios — quantity increment, decrement-not-delete,
checkout, all four negative cases, and the price-delta cache — now match
expectations exactly. That closes out the verification we set out to do for this
piece of Step 8. Two things are worth carrying forward rather than treating as
fully closed, both of which we already named honestly along the way rather than
discovering now: the `AdjustProductPrice` sequence guard has been proven to
compute the _correct_ result under normal operation, but not proven to actually
block a genuine redelivery-after-crash duplicate (that needs a dedicated
failure-injection test, not curl calls against a healthy system); and the async
projector race (`sleep 2`) worked fine here but is still a timing assumption,
not a guarantee — a slower CI environment or a heavily loaded broker could in
principle need longer. At this point `baskets` has: full
domain/event/catalog/postgres/application coverage via unit and integration
tests, a live end-to-end RPC verification against real `stores`/Postgres/NATS,
and two real correctness bugs found and fixed along the way (the blank-productId
ordering issue, and ADL 0015's cross-service publish-duplication fix). That's a
solid stopping point for Step 8's scope on `baskets` specifically — the
remaining Step 8 items from the Phase plan (`CONTRACT.md` for `baskets`,
`system-state.md` update, `journey_progress.md` close-out with the deferred
reminders we've been collecting) are documentation, not more code or testing.
Want me to draft those three now — `baskets/CONTRACT.md`, the `system-state.md`
diff, and the full `journey_progress.md` Phase 3 close-out (including the
`BasketCheckedOut` payload-thinness reminder for Phase 4/5, and the
newly-deferred redelivery-failure-injection test) — to actually close out this
Phase, or is there something else you want to check first?

## Phase 4 Plan -- Depot

Once that's settled, here's the Step breakdown I'd propose for Phase 4,
mirroring Phase 3's shape exactly:

- Step 1 — `ShoppingList` domain aggregate (lifecycle guards, event emission)
- Step 2 — Domain test suite
- Step 3 — `events` package (envelope/payload/subject/stream)
- Step 4 — `catalog` package (Store/Product read-model + narrow per-event
  `Repository`, reusing `baskets`' redesigned shape from the start this time)
- Step 5 — `postgres` package (repositories + migrations)
- Step 6 — `application` layer (use cases, catalog projector wired in)
- Step 7 — `connectapi` (proto + server)
- Step 8 — Integration verification (real Postgres/NATS tests + live curl
  script)

Good — that locks in the design. Let me recap the finalized shape so we're both
building against the same thing before any code gets written.

**`ShoppingList` aggregate — final shape for Step 1:**

```
ShoppingList
  ID     string
  Status ShoppingListStatus
  Stops  map[string]*Stop   // keyed by storeID

Stop
  StoreName     string
  StoreLocation string
  Items         []Item      // {ProductID, Quantity}, same shape as Basket.Item
```

**Lifecycle**, all flat states, no bot-related ones:

- `NewShoppingList(id)` → `pending`, emits `ShoppingListCreated`. Takes no
  `orderID` — `ordering` doesn't exist yet (ADL 0004), and we're treating that
  gap as a deliberate, named deferral rather than a placeholder field. I'll add
  a line to this effect in the Phase 4 summary so it's a documented standing
  item, the same way ADL 0007's outbox gap and ADL 0012's multi-failure
  absorption are tracked — something Project 3 (`ordering`) must revisit, not
  silently discover.
- `AddItem(store *Store, product *Product, quantity int)` — only legal while
  `pending`. Finds or creates the `Stop` for `store.ID`, then merges into that
  `Stop`'s `Items` (same increment-on-repeat-add semantics as `Basket.AddItem`).
  Requires the `application` layer to resolve both `store` and `product` from
  `depot`'s own catalog cache before calling in — two lookups instead of
  `baskets`' one, a direct consequence of `Stops` needing store data the
  aggregate itself never fetches.
- `RemoveItem(storeID, productID string)` — takes both IDs, per your call, to
  avoid ambiguity if two stores coincidentally carry a product with the same ID.
  Finds the `Stop` by `storeID`, decrements/removes the item within it, and
  deletes the `Stop` entirely if that was its last item — so `Stops` never
  carries dead, empty entries forward.
- `Initiate()` — `pending` → `active`, emits `ShoppingListInitiated`. No
  `Assign()`/`AssignedBotID` at all; there's no bot to assign to, so `active`
  just means "picking has started," full stop.
- `Complete()` — `active` → `completed`, emits `ShoppingListCompleted`,
  rejecting an empty `Stops` map with `ErrShoppingListIsEmpty` (mirrors
  `Basket.Checkout`'s `ErrBasketIsEmpty`), per your confirmation. This is the
  one event `ordering` will eventually consume — and per your own reasoning, a
  completed list is presumably a precondition for creating the order, not
  something ordering waits on the other way around. Worth double-checking
  against the book's actual sequencing once `ordering` is real, but it's a
  reasonable working assumption for now.
- `Cancel()` — legal from `pending` or `active`, emits `ShoppingListCanceled`.
  I'd keep `Basket.Cancel`'s pattern of two distinct terminal-state errors
  (`ErrShoppingListAlreadyCompleted`, `ErrShoppingListAlreadyCanceled`) rather
  than one collapsed error, for consistency.

That's a complete, self-consistent Step 1 with no dangling references to
services that don't exist yet. If this matches what you're picturing, I'll write
`shopping_list.go`, `shopping_list_status.go`, and `events.go` (mirroring
`baskets_basket.go`, and folding status + events into files matching your naming
convention) as the next message, then we do the check-in per the skill before
moving to Step 2 (the domain test suite).

Ready for me to write Step 1?

### Step 1 Shopping List Domain

Let's start with the domain layer — four files, mirroring `baskets`' Step 1
split (aggregate, status, events, repository interface).

```go
// depot/internal/domain/shopping_list_status.go
```

```go
// depot/internal/domain/events.go
```

```go
// depot/internal/domain/shopping_list.go
```

```go
// depot/internal/domain/shopping_list_repository.go
```

**What's happening here, and two deliberate divergences from the book worth
flagging:**

The overall shape mirrors `Basket` closely — status guards on every mutating
method, an `Event` returned alongside the error so the caller (the future
`application` layer) can publish it, a `Reconstitute` constructor for the
postgres layer to rehydrate from. `Stops` is the one real structural difference:
a `map[string]*Stop` keyed by store ID, each holding its own denormalized
`StoreName`/`StoreLocation`/`Items`, so a shopping list genuinely reads as
"here's what to gather from each store," not just a flat cart.

First divergence: `AddItem` here takes no explicit `quantity` parameter, unlike
the book's `AddItem(store, product, quantity int)`. I chose to match
`baskets.AddItem`'s convention instead — one unit added per call, incremented on
repeat — for consistency across the two services' APIs, since that's the pattern
the storefront (Project 6) will eventually call against for carts and,
presumably, order fulfillment alike. If you'd rather keep the book's
explicit-quantity signature (useful if `ordering` later wants to hand `depot` a
whole basket's worth of quantities in fewer calls), that's a one-method change
now, before anything else depends on the shape — worth deciding here rather than
after Step 6 wires it in.

Second: I added `ItemAdded`/`ItemRemoved` events, which the book's version
doesn't emit at all — `AddItem` in the book never calls anything event-related.
I added them purely for symmetry with `baskets`' event set, on the theory that a
consistent "every mutation gets an event" convention across services is worth
more than exactly matching the book here, since nothing in the roadmap says a
future consumer needs them — same "TBD" status `system-state.md` already carries
for `depot`'s full event list. If you'd rather keep `depot` lean and only emit
the four lifecycle events the book actually defines, that's easy to trim now.

Also note `AddItem`'s signature takes `storeName`/`storeLocation` as plain
strings rather than a `*Store` struct — this is intentional, matching the
"domain doesn't import `catalog`" separation ADL 0004/Phase 3 already
established for `baskets`. The `application` layer (Step 6) will be the one
resolving a `catalog.Store`/`catalog.Product` and unpacking the fields it needs
before calling in here.

Copy these four files into `depot/internal/domain/`, then run `go build ./...`
from the `depot` module — there's no test suite yet (that's Step 2), so this is
just confirming it compiles cleanly. Let me know what you get, and any thoughts
on the two divergences above before we move to Step 2.

#### Shopping List Initiated to Completed

Good catch — that's a real gap I left implicit. `Initiate()` isn't triggered
automatically by anything inside the domain; it has to be called explicitly, the
same way `Basket.Checkout()` is explicitly called via a `Checkout` RPC rather
than firing itself once some condition is met.

The way I've designed it, `Initiate()` is the `ShoppingList`'s equivalent of
"picking has begun" — someone (a warehouse worker, in the book's framing) has
started actually walking the stores and gathering the items on the list. In our
system, since we have no `Bot` domain to auto-trigger this, it has to be a
distinct application-layer use case with its own ConnectRPC method — call it
`InitiateShoppingList`, taking just a `shoppingListID`, exactly like `Checkout`
in `baskets` takes just a `basketID`. Whoever's driving fulfillment (today: your
curl script; eventually, presumably some depot-side operator UI, though that's
out of scope for this system per the roadmap) would call it once they're ready
to lock the list and start gathering.

The practical effect of that transition, worth being explicit about:
`AddItem`/`RemoveItem` both guard on `Status == ShoppingListPendingStatus`, so
once `Initiate()` fires, the list's `Stops` are frozen — no more additions or
removals are possible from that point forward. That's a deliberate mirror of
`Basket.Checkout()` sealing the basket's contents, just one state earlier in
`depot`'s case (`pending → active` here, versus `open → checked_out` there being
the final seal). So the full call sequence for a `ShoppingList` in this project
would be: `CreateShoppingList` → some number of `AddItem`/`RemoveItem` calls
while `pending` → `InitiateShoppingList` (freezes contents, moves to `active`) →
`CompleteShoppingList` (moves to `completed`, the event `ordering` will
eventually consume) — or `CancelShoppingList` at any point before completion.

Does that match what you had in mind, or were you picturing `Initiate()` firing
automatically off some other signal (e.g. the moment the first item is added,
rather than a standalone call)? If it's the latter, we'd fold `Initiate`'s
effect into `AddItem` itself and drop it as a separate public method — but I'd
lean against that, since it would mean a `ShoppingList` with even one item can
never be edited again, which seems too eager compared to `Basket`'s much
longer-lived "open" window.

### Stop 2 - Testing ShoppingList

Domain tests, one file, covering every guard and every event emission for the
five behaviors:

```go
// depot/internal/domain/shopping_list_test.go
```

**What's here, and a couple of choices worth flagging:**

Every guard on every method has its own dedicated failure test (`AddItem`'s
three guards, `RemoveItem`'s two, `Initiate`'s one, `Complete`'s two, `Cancel`'s
two), plus a success case per method that also asserts the emitted event's
concrete type and field values — the same density `journey_progress.md`
describes for `store_test.go`/`baskets_basket_test.go`.
`TestShoppingList_AddItem_BlockedAfterInitiate` is the one test with no `Basket`
equivalent worth calling out: it specifically proves the freeze-on-`Initiate`
behavior we discussed a couple of messages back, since that's a real behavioral
claim, not just an implementation detail — worth pinning down with a test rather
than trusting the guard reads correctly by eye.

`TestShoppingList_RemoveItem_SameProductDifferentStores` is the other one worth
flagging — it directly exercises the reasoning behind requiring `storeID` in
`RemoveItem`'s signature (your call from a few messages ago), proving that
removing a product from one store's stop genuinely leaves an identically-IDed
product in a different store's stop untouched.

One thing I did _not_ test: `NewShoppingList`'s domain-level `id` is never
validated for blankness (same as `NewBasket`'s `id` in `baskets` — that
constructor trusts its caller). That's consistent with ADL 0009: ID generation
and validation are the `connectapi` layer's concern via `uuid.NewString()`, not
the domain's.

Copy this into `depot/internal/domain/shopping_list_test.go` and run
`go test ./internal/domain/...` from the `depot` module. Every test should pass.
Let me know what you get.

### Step 3 -- Events package

```go
// depot/internal/events/stream.go
```

```go
// depot/internal/events/subject.go
```

```go
// depot/internal/events/payload.go
```

```go
// depot/internal/events/envelope.go
```

**What's here — this one's a near-direct port, so only two things are worth
calling out rather than a full walkthrough.**

First, the subject/stream naming: `depot`'s stream subjects are
`depot.shopping_list.*`, matching `baskets.basket.*`'s `service.aggregate.*`
convention exactly, and `aggregateFor` maps all six event types to the single
string `"shopping_list"` — same flat single-aggregate-per-service shape
`baskets` has with `"basket"`. Worth noting this immediately obsoletes the
`depotStreamName`/`depotStreamSubjects` constants currently sitting inline in
`depot_module.go` from Phase 2 (the `-- TEMPORARY --` block) — we'll swap
`Module.New` over to importing this `events` package instead of its own local
constants when we get to Step 6, so don't touch `depot_module.go` yet, just
flagging it's coming.

Second, `ItemAdded`'s payload carries `StoreID` alongside `ProductID`/`Quantity`
— unlike `baskets`' `ItemAddedPayload`, which only has `ProductID`/`Quantity`.
That's a direct, necessary consequence of `Stops` being store-keyed: a
downstream consumer of this event (should one ever exist) can't make sense of
"add this product" without knowing which store's stop it belongs to, the same
way `RemoveItem`'s domain signature needed `storeID` to disambiguate.

Copy these four files into `depot/internal/events/`, then run `go build ./...`
from the `depot` module to confirm it compiles. Let me know what you get, and
any questions before Step 4 — the `catalog` package.
