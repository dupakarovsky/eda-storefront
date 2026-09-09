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

## Phase 2 Summary — COMPLETE

**Status:** All steps (1 through 4g) complete and verified against live
containers. Phase 2 closed out.

### What Was Built

**Step 1 — `foundation/nats.Subscriber`.** A second concrete
`shutdown.Service` implementation (`foundation/nats/subscribe.go`),
alongside the existing `Publisher`. Wraps a JetStream pull consumer
(`Consumer.Consume`), blocking `Run()` on an internal `done` channel to
satisfy the `Service` contract, with `Shutdown` calling `Drain()` and
genuinely waiting on `ConsumeContext.Closed()` (bounded by the caller's
`ctx`) rather than returning immediately, and `Close` calling `Stop()` as
the hard-abandon fallback. A `sync.Mutex` guards the `ConsumeContext`
field itself, written in `Run`'s goroutine and read from separate
`Shutdown`/`Close` goroutines spawned by `shutdown.Run`'s broadcast.

**Step 2 — Redelivery and dead-letter policy (ADL 0013).**
`onMessage` computes an explicit `NakWithDelay` from the message's own
`NumDelivered` and the consumer's configured `BackOff` schedule, rather
than a bare `Nak()` — necessary because `Nak()` bypasses `AckWait`/
`BackOff` entirely (confirmed against the `nats.go/jetstream` docs, not
assumed). On `MaxDeliver` exhaustion, an injected `DeadLetterFunc` is
invoked and `msg.Term()` is called explicitly, rather than trusting
`MaxDeliver`'s own bookkeeping alone. `foundation/nats.PublishDeadLetter`
republishes the raw message onto `<service>.deadletter.<original-subject>`
via the existing `Publisher`. A `msg.Metadata()` failure is handled by
deliberately leaving the message unacked, falling back to JetStream's own
AckWait-timeout redelivery path (which does honor `BackOff` natively).
`foundation/nats.StandardConsumerConfig(durable)` encodes the agreed
values (`AckWait: 5s`, `MaxDeliver: 5`, four-step `BackOff` schedule) as a
single reusable helper, so ADL 0013 is enforced by the compiler for every
future consumer, not just documented as a convention to remember.

**Step 3 — `baskets`/`depot` module scaffolding.** Two new, separate Go
modules (`depot-baskets/baskets/go.mod`, `depot-baskets/depot/go.mod`),
each `replace`-ing `foundation` via `../../foundation` (one directory
level deeper than `stores`', since both live nested inside the
`depot-baskets/` project folder). Both wired into the root `go.work`.

**Step 4 — Full service skeletons, wired into `docker-compose.yaml`.**
Both `baskets` and `depot` now have a `Module` type (Postgres pool, NATS
connection/JetStream context, `Publisher`, `Subscriber`) constructed via
`New(ctx, cfg, logger)`, a `cmd/service/main.go` following `stores`' exact
shape (router/middleware/HTTP server/`shutdown.Run` with **two** services
— HTTP and the NATS `Subscriber`), and a `/health` handler checking both
Postgres and NATS liveness. Each ensures its own outgoing stream
(`baskets`/`depot` respectively) and attaches a durable consumer
(`baskets-stores-projector` / `depot-stores-projector`) to `stores`' own
stream — referenced by name only, never ensured, since neither service
owns it. Both handlers are currently no-op placeholders (log-only, always
return `nil`) — no domain logic yet. `basketsdb`/`depotdb` provisioned via
new init scripts (`postgres/init/04-baskets-db.sh`,
`05-depot-db.sh`), each service containerized via its own two-stage
Dockerfile, and both added as full service blocks in
`docker-compose.yaml` (ports `8081`/`8082`, `depends_on: stores:
condition: service_started` for correct startup ordering).

**Mid-phase — folder rename (ADL 0014).** `01-stores` → `stores`,
`02-depot-baskets` → `depot-baskets`, dropping numeric build-order
prefixes from every project folder now that they're redundant with
`docs/roadmap.md`'s own project table. Verified the Go module path itself
never depended on the folder name, so this was a pure path-reference
fix across `go.work`, `docker-compose.yaml`, the `Makefile`,
`CONTRACT.md`, and `roadmap.md`'s own diagram — no `.go` import
statements needed changing.

### Issues Found & Resolved

1. **`msg.Nak()` silently bypasses `AckWait`/`BackOff`** — the single
   most consequential finding of this phase. Caught during design, before
   any code shipped with it, by checking the actual `nats.go/jetstream`
   API rather than assuming `Nak()` behaved like a timeout-based
   redelivery. Full reasoning in ADL 0013.
2. **`Subscriber` wired to the wrong stream — twice, independently, in
   both `baskets`' and `depot`'s first drafts.** Both initially passed
   their own outgoing stream name (`baskets`/`depot`) into `NewSubscriber`
   instead of `stores` — the consumer would have attached to an empty
   stream and silently received nothing, no error, no crash. Caught by
   code review before either was ever run, in both cases.
3. **Two Postgres init scripts (`04-baskets-db.sh`, then independently
   `05-depot-db.sh`) referenced the wrong `--dbname`** in their grants
   block (`basketsdb`'s script briefly targeted `edadb`'s own session
   context by omission; `depot`'s script literally said `--dbname
basketsdb`, copy-pasted from its sibling). Both live databases were
   provisioned correctly by hand regardless, by explicitly `\c`-ing to the
   right database first — but both scripts-on-disk needed the same fix
   for any future full volume reinit to provision correctly.
4. **`baskets`'/`depot`'s `main.go` didn't `os.Exit` on module-init
   failure** — logged the error, then fell through to `defer mod.Close()`
   on a `nil` `*Module`, which would nil-pointer panic instead of exiting
   cleanly. Traced back to the exact same gap already present, unnoticed,
   in `stores`' own `main.go` since Phase 4 of that project — fixed in
   all three.
5. Two small but real copy-paste artifacts in `baskets`' health handler:
   a stale `Content-Type: application/json` header on a plain-text body,
   and a NATS-down branch that logged `"postgres unreachable"` instead of
   `"nats unreachable"`.
6. `foundation/config.go`'s `LoggerConfig.FilePath` and `NATSConfig.Name`
   still carried stale, `stores`-specific defaults (`"stores.log"`,
   `"foundation-smoketest"`) that would have silently propagated into
   every future service. Fixed to an empty default (console-only logging)
   and no default (forcing each service's own `.env` to state a real
   connection name) respectively, before either new service's `.env` was
   even written.

### Key Concepts Reinforced

**A shared event's replay is a real, load-bearing guarantee, not a
theoretical one.** Both `baskets` and `depot`, on their very first
startup, independently replayed the entire historical event backlog
`stores`' stream had accumulated — a direct, concrete demonstration of
why ADL 0003 picked JetStream over core NATS. This also makes the
"idempotent cache upsert" requirement `journey_progress.md`'s own Concepts
section already named a proven, load-bearing necessity for Phase 3/4's
real handlers, not a nicety.

**Two independent durable consumers against one stream genuinely don't
interact.** `baskets-stores-projector` and `depot-stores-projector`
each maintain their own separate delivery cursor on `stores`' stream —
confirmed by both independently replaying the full history on first
attach, neither affected by the other's ack state.

**"Shutdown complete" and "service goroutine has exited" are two
independent happens-before chains sharing one root cause, not one
ordered sequence.** Observed directly, twice (once for `baskets`, once
for `depot`): the log line order between a service's own `Run()`-exit
message and `shutdown.Run`'s "shutdown complete" message is
non-deterministic — both are legitimate consequences of the same
`s.done` channel closing, raced across two different goroutines with no
ordering guarantee between them. Nothing downstream depends on this
ordering; `shutdown.Run`'s only real guarantee is that every `Shutdown()`
call has returned by the time "shutdown complete" logs, which held in
both observed runs.

**An unused Go constant doesn't fail the build — it fails silently.**
Both the `baskets` and `depot` wrong-stream bugs (`Issues Found #2`) would
have compiled and run without any visible error; only human review of the
actual argument passed to `NewSubscriber`, not the compiler, caught them.
Worth carrying forward as a standing reminder that "it builds" and "it's
correct" are different bars for this specific class of mistake.

### State at End of Phase 2

- `foundation/nats` now exposes `Subscriber`, `DeadLetterFunc`/
  `PublishDeadLetter`, and `StandardConsumerConfig` — a complete,
  documented (ADL 0013), compiler-enforced consuming counterpart to the
  existing `Publisher`.
- `baskets` and `depot` are both genuinely live, containerized services
  (`eda-baskets`, `eda-depot`), each with a working `/health` endpoint,
  each independently consuming `stores`' event stream via its own durable
  consumer — but **skeleton only**: no `Basket`/`ShoppingList` domain, no
  ConnectRPC surface, no real cache-projection logic. The placeholder
  handler logs and acks; it does not yet write anything to
  `basketsdb`/`depotdb`.
- `basketsdb` and `depotdb` exist, are correctly granted to `eda_user`
  (verified via `\dn+ public` after correcting the manual-provisioning
  database-context mistake), and have no tables yet.
- `docker-compose.yaml` runs all five containers (`postgres`, `nats`,
  `stores`, `baskets`, `depot`) together, cleanly, verified via `make
up`/`make ps`/`/health` against all three HTTP services.
- **Known, accepted gap carried forward:** `depends_on: stores:
condition: service_started` guarantees container start order only, not
  that `stores` has finished `EnsureStream`-ing its stream before
  `baskets`/`depot` attempt to attach a consumer to it. Accepted as a
  reasonable race for now — a container restart would recover — but
  worth remembering if it's ever observed to actually bite.
- **Known, deliberately deferred:** real concurrency/failure-injection
  testing of the redelivery and dead-letter path (forced handler
  failures, forced metadata-read failures, forced dead-letter-publish
  failures) — proving ADL 0013's design under load, the same way ADL 0010
  and ADL 0012 were each proven with a dedicated timing test. Deferred
  specifically until Phase 3/4 gives the `Subscriber` a real handler with
  a real failure mode to test against, rather than testing the placeholder
  in isolation.

### Carried Into Phase 3

- Both projector handlers are still pure placeholders — Phase 3's first
  real task for `baskets` is replacing its placeholder with genuine
  `Store`/`Product` cache-projection logic (idempotent upserts, per the
  concept reinforced above), before any `Basket`/`Item` domain work
  begins.
- `docs/system-state.md` needs updating to reflect `baskets`/`depot` as
  live-but-skeleton — HTTP/Postgres/NATS plumbing only, explicitly not
  yet publishing or consuming any real domain event, so Phase 3 doesn't
  start from an inflated sense of what already exists.
- The `stores`-liveness race in `depends_on` (noted above) and the
  deferred redelivery/dead-letter testing are both standing items to
  revisit, not forgotten gaps — same treatment as ADL 0007's outbox
  deferral and ADL 0012's known multi-failure absorption limitation.
