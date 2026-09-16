# EVENT DRIVE ARCHITECTURE STOREFRONT IN GO: DEPOT + BASKETS

## Project Concept

`depot` and `baskets`, each pairing a **projector** (an independent NATS
JetStream consumer maintaining a local read-only cache of `stores`' catalog
data) with an **aggregate owner** — `baskets` owns `Basket`, `depot` owns a lean
`ShoppingList` (`Create → Initiate → Complete`/`Cancel`, Bot/Stop sub-domain
deliberately cut — confirmed unreferenced anywhere else in the book's own
Chapter12 source, including its own public API, before cutting it). First
project to build the _consuming_ side of NATS; Project 1 only ever published.

## Concepts We'll Cover

NATS JetStream pull consumers (`foundation/nats` `Subscriber`), idempotent
cache-projection upserts, two independent durable consumer groups against one
stream, cache-miss fallback strategy, multi-listener graceful shutdown (see ADL
0012), `Basket`/`ShoppingList` as current-state aggregates (ADL 0002) reusing
ADL 0008/0009 unchanged.

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

______________________________________________________________________

## Phase 1 Summary — COMPLETE

**Status:** Single-step phase. Step 1 complete and verified. Phase 1 closed out.

### What Was Built

**Step 1 — `Service` interface, `httpService` adapter, `Run` rewrite.**
`foundation/shutdown/service.go` (the `Service` interface),
`foundation/shutdown/http_service.go` (`httpService`/`NewHTTPService` wrapping
`*http.Server`), and `shutdown.go`'s `Run` generalized from a single
`*http.Server` parameter to `services ...Service`, broadcasting `Shutdown(ctx)`
concurrently against one shared deadline with a per-service `Close()` fallback.
Full reasoning and the alternative considered (the book's `errgroup`/`WaitFunc`
shape) recorded in ADL 0012. `01-stores`' call site updated to the new signature
— one line changed, per ADL 0012's surface-area goal.

### Issues Found & Resolved

Three real bugs surfaced during review, all invisible with only one registered
service (`stores`), all documented in full in ADL 0012's Consequences:

1. Non-deferred `wg.Done()` — `Run` returned before shutdown had actually
   completed.
1. Duplicate `Close()` call on the failure path.
1. The `errs`-branch `return`ing immediately instead of falling through to the
   shared shutdown sequence — would have left sibling services never notified of
   a peer's failure.

A fourth, in the test harness itself: calling `t.Errorf`/`t.Fatalf` from a
background goroutine after the main test goroutine had already exited via an
earlier `t.Fatalf`, causing a nil-pointer panic instead of a clean failure —
fixed by reporting results back over a channel and asserting only from the main
test goroutine.

### Key Concepts Reinforced

**Proving timing, not just final state.**
`TestRun_WaitsForInFlightRequestBeforeReturning` deliberately races a slow
in-flight HTTP request against a triggered shutdown, and was confirmed to fail
(cleanly, on the exact property it exists to check) against a
deliberately-reintroduced non-deferred `wg.Done()`, then confirmed to pass once
restored — same discipline as ADL 0010's
`TestStoreRepository_WithLock_SerializesConcurrentAccess`.

**Concurrent vs. sequential shutdown broadcast.** Shutting services down one
after another inside one shared timeout silently starves later services of
budget — the same class of mistake as ADL 0007's addendum (retry budget vs.
global HTTP timeout). Broadcasting concurrently against one shared deadline is
the only version where the configured timeout means what it says for every
service, not just the first.

**Go 1.22+ per-iteration loop variables.** Confirmed the `svc := svc`
capture-guard pattern common in older Go code (and used throughout the book,
pinned to `go 1.18`) is unnecessary on our current toolchain — each loop
iteration already gets its own variable copy natively.

### State at End of Phase 1

- `foundation/shutdown` now generalizes over any number of `Service`s, proven
  correct via a real concurrency/timing test (both positive and negative), not
  just a manual single-service smoke check.
- `01-stores` runs against the new `Run` signature, manually verified: clean
  startup, clean SIGTERM shutdown sequence, correct log ordering.
- **Known limitation, deliberately deferred** (ADL 0012): concurrent `Run()`
  failures beyond the first are silently absorbed, not logged. Worth closing
  once `depot`/`baskets` both being live makes this concrete rather than
  hypothetical — same treatment as ADL 0007's outbox deferral.

### Carried Into Phase 2

- `Service` is the contract the NATS `Subscriber` must be built against from the
  start — `Shutdown` → `ConsumeContext.Drain()`, `Close` → `Stop()`.
- The shared `*nats.Conn` is explicitly **not** a `Service` — closed once,
  directly, by `main()` after `Run()` returns, to avoid a `Subscriber` yanking
  the connection out from under a `Publisher` mid-retry.

## Phase 2 Summary — COMPLETE

**Status:** All steps (1 through 4g) complete and verified against live
containers. Phase 2 closed out.

### What Was Built

**Step 1 — `foundation/nats.Subscriber`.** A second concrete `shutdown.Service`
implementation (`foundation/nats/subscribe.go`), alongside the existing
`Publisher`. Wraps a JetStream pull consumer (`Consumer.Consume`), blocking
`Run()` on an internal `done` channel to satisfy the `Service` contract, with
`Shutdown` calling `Drain()` and genuinely waiting on `ConsumeContext.Closed()`
(bounded by the caller's `ctx`) rather than returning immediately, and `Close`
calling `Stop()` as the hard-abandon fallback. A `sync.Mutex` guards the
`ConsumeContext` field itself, written in `Run`'s goroutine and read from
separate `Shutdown`/`Close` goroutines spawned by `shutdown.Run`'s broadcast.

**Step 2 — Redelivery and dead-letter policy (ADL 0013).** `onMessage` computes
an explicit `NakWithDelay` from the message's own `NumDelivered` and the
consumer's configured `BackOff` schedule, rather than a bare `Nak()` — necessary
because `Nak()` bypasses `AckWait`/ `BackOff` entirely (confirmed against the
`nats.go/jetstream` docs, not assumed). On `MaxDeliver` exhaustion, an injected
`DeadLetterFunc` is invoked and `msg.Term()` is called explicitly, rather than
trusting `MaxDeliver`'s own bookkeeping alone.
`foundation/nats.PublishDeadLetter` republishes the raw message onto
`<service>.deadletter.<original-subject>` via the existing `Publisher`. A
`msg.Metadata()` failure is handled by deliberately leaving the message unacked,
falling back to JetStream's own AckWait-timeout redelivery path (which does
honor `BackOff` natively). `foundation/nats.StandardConsumerConfig(durable)`
encodes the agreed values (`AckWait: 5s`, `MaxDeliver: 5`, four-step `BackOff`
schedule) as a single reusable helper, so ADL 0013 is enforced by the compiler
for every future consumer, not just documented as a convention to remember.

**Step 3 — `baskets`/`depot` module scaffolding.** Two new, separate Go modules
(`depot-baskets/baskets/go.mod`, `depot-baskets/depot/go.mod`), each
`replace`-ing `foundation` via `../../foundation` (one directory level deeper
than `stores`', since both live nested inside the `depot-baskets/` project
folder). Both wired into the root `go.work`.

**Step 4 — Full service skeletons, wired into `docker-compose.yaml`.** Both
`baskets` and `depot` now have a `Module` type (Postgres pool, NATS
connection/JetStream context, `Publisher`, `Subscriber`) constructed via
`New(ctx, cfg, logger)`, a `cmd/service/main.go` following `stores`' exact shape
(router/middleware/HTTP server/`shutdown.Run` with **two** services — HTTP and
the NATS `Subscriber`), and a `/health` handler checking both Postgres and NATS
liveness. Each ensures its own outgoing stream (`baskets`/`depot` respectively)
and attaches a durable consumer (`baskets-stores-projector` /
`depot-stores-projector`) to `stores`' own stream — referenced by name only,
never ensured, since neither service owns it. Both handlers are currently no-op
placeholders (log-only, always return `nil`) — no domain logic yet.
`basketsdb`/`depotdb` provisioned via new init scripts
(`postgres/init/04-baskets-db.sh`, `05-depot-db.sh`), each service containerized
via its own two-stage Dockerfile, and both added as full service blocks in
`docker-compose.yaml` (ports `8081`/`8082`,
`depends_on: stores: condition: service_started` for correct startup ordering).

**Mid-phase — folder rename (ADL 0014).** `01-stores` → `stores`,
`02-depot-baskets` → `depot-baskets`, dropping numeric build-order prefixes from
every project folder now that they're redundant with `docs/roadmap.md`'s own
project table. Verified the Go module path itself never depended on the folder
name, so this was a pure path-reference fix across `go.work`,
`docker-compose.yaml`, the `Makefile`, `CONTRACT.md`, and `roadmap.md`'s own
diagram — no `.go` import statements needed changing.

### Issues Found & Resolved

1. **`msg.Nak()` silently bypasses `AckWait`/`BackOff`** — the single most
   consequential finding of this phase. Caught during design, before any code
   shipped with it, by checking the actual `nats.go/jetstream` API rather than
   assuming `Nak()` behaved like a timeout-based redelivery. Full reasoning in
   ADL 0013.
1. **`Subscriber` wired to the wrong stream — twice, independently, in both
   `baskets`' and `depot`'s first drafts.** Both initially passed their own
   outgoing stream name (`baskets`/`depot`) into `NewSubscriber` instead of
   `stores` — the consumer would have attached to an empty stream and silently
   received nothing, no error, no crash. Caught by code review before either was
   ever run, in both cases.
1. **Two Postgres init scripts (`04-baskets-db.sh`, then independently
   `05-depot-db.sh`) referenced the wrong `--dbname`** in their grants block
   (`basketsdb`'s script briefly targeted `edadb`'s own session context by
   omission; `depot`'s script literally said `--dbname basketsdb`, copy-pasted
   from its sibling). Both live databases were provisioned correctly by hand
   regardless, by explicitly `\c`-ing to the right database first — but both
   scripts-on-disk needed the same fix for any future full volume reinit to
   provision correctly.
1. **`baskets`'/`depot`'s `main.go` didn't `os.Exit` on module-init failure** —
   logged the error, then fell through to `defer mod.Close()` on a `nil`
   `*Module`, which would nil-pointer panic instead of exiting cleanly. Traced
   back to the exact same gap already present, unnoticed, in `stores`' own
   `main.go` since Phase 4 of that project — fixed in all three.
1. Two small but real copy-paste artifacts in `baskets`' health handler: a stale
   `Content-Type: application/json` header on a plain-text body, and a NATS-down
   branch that logged `"postgres unreachable"` instead of `"nats unreachable"`.
1. `foundation/config.go`'s `LoggerConfig.FilePath` and `NATSConfig.Name` still
   carried stale, `stores`-specific defaults (`"stores.log"`,
   `"foundation-smoketest"`) that would have silently propagated into every
   future service. Fixed to an empty default (console-only logging) and no
   default (forcing each service's own `.env` to state a real connection name)
   respectively, before either new service's `.env` was even written.

### Key Concepts Reinforced

**A shared event's replay is a real, load-bearing guarantee, not a theoretical
one.** Both `baskets` and `depot`, on their very first startup, independently
replayed the entire historical event backlog `stores`' stream had accumulated —
a direct, concrete demonstration of why ADL 0003 picked JetStream over core
NATS. This also makes the "idempotent cache upsert" requirement
`journey_progress.md`'s own Concepts section already named a proven,
load-bearing necessity for Phase 3/4's real handlers, not a nicety.

**Two independent durable consumers against one stream genuinely don't
interact.** `baskets-stores-projector` and `depot-stores-projector` each
maintain their own separate delivery cursor on `stores`' stream — confirmed by
both independently replaying the full history on first attach, neither affected
by the other's ack state.

**"Shutdown complete" and "service goroutine has exited" are two independent
happens-before chains sharing one root cause, not one ordered sequence.**
Observed directly, twice (once for `baskets`, once for `depot`): the log line
order between a service's own `Run()`-exit message and `shutdown.Run`'s
"shutdown complete" message is non-deterministic — both are legitimate
consequences of the same `s.done` channel closing, raced across two different
goroutines with no ordering guarantee between them. Nothing downstream depends
on this ordering; `shutdown.Run`'s only real guarantee is that every
`Shutdown()` call has returned by the time "shutdown complete" logs, which held
in both observed runs.

**An unused Go constant doesn't fail the build — it fails silently.** Both the
`baskets` and `depot` wrong-stream bugs (`Issues Found #2`) would have compiled
and run without any visible error; only human review of the actual argument
passed to `NewSubscriber`, not the compiler, caught them. Worth carrying forward
as a standing reminder that "it builds" and "it's correct" are different bars
for this specific class of mistake.

### State at End of Phase 2

- `foundation/nats` now exposes `Subscriber`, `DeadLetterFunc`/
  `PublishDeadLetter`, and `StandardConsumerConfig` — a complete, documented
  (ADL 0013), compiler-enforced consuming counterpart to the existing
  `Publisher`.
- `baskets` and `depot` are both genuinely live, containerized services
  (`eda-baskets`, `eda-depot`), each with a working `/health` endpoint, each
  independently consuming `stores`' event stream via its own durable consumer —
  but **skeleton only**: no `Basket`/`ShoppingList` domain, no ConnectRPC
  surface, no real cache-projection logic. The placeholder handler logs and
  acks; it does not yet write anything to `basketsdb`/`depotdb`.
- `basketsdb` and `depotdb` exist, are correctly granted to `eda_user` (verified
  via `\dn+ public` after correcting the manual-provisioning database-context
  mistake), and have no tables yet.
- `docker-compose.yaml` runs all five containers (`postgres`, `nats`, `stores`,
  `baskets`, `depot`) together, cleanly, verified via
  `make up`/`make ps`/`/health` against all three HTTP services.
- **Known, accepted gap carried forward:**
  `depends_on: stores: condition: service_started` guarantees container start
  order only, not that `stores` has finished `EnsureStream`-ing its stream
  before `baskets`/`depot` attempt to attach a consumer to it. Accepted as a
  reasonable race for now — a container restart would recover — but worth
  remembering if it's ever observed to actually bite.
- **Known, deliberately deferred:** real concurrency/failure-injection testing
  of the redelivery and dead-letter path (forced handler failures, forced
  metadata-read failures, forced dead-letter-publish failures) — proving ADL
  0013's design under load, the same way ADL 0010 and ADL 0012 were each proven
  with a dedicated timing test. Deferred specifically until Phase 3/4 gives the
  `Subscriber` a real handler with a real failure mode to test against, rather
  than testing the placeholder in isolation.

### Carried Into Phase 3

- Both projector handlers are still pure placeholders — Phase 3's first real
  task for `baskets` is replacing its placeholder with genuine `Store`/`Product`
  cache-projection logic (idempotent upserts, per the concept reinforced above),
  before any `Basket`/`Item` domain work begins.
- `docs/system-state.md` needs updating to reflect `baskets`/`depot` as
  live-but-skeleton — HTTP/Postgres/NATS plumbing only, explicitly not yet
  publishing or consuming any real domain event, so Phase 3 doesn't start from
  an inflated sense of what already exists.
- The `stores`-liveness race in `depends_on` (noted above) and the deferred
  redelivery/dead-letter testing are both standing items to revisit, not
  forgotten gaps — same treatment as ADL 0007's outbox deferral and ADL 0012's
  known multi-failure absorption limitation.

## Phase 3 Summary — COMPLETE

**Status:** All 8 Steps complete and verified against live containers, including
two Steps' worth of real correctness fixes discovered mid-Phase, not just
planned work. Phase 3 closed out.

### What Was Built

**Step 1 — `Basket`/`Item` domain aggregate.** `NewBasket`,
`AddItem`/`RemoveItem` (quantity-merge on repeat add, decrement-then-remove on
repeat removal, `ProductID`-keyed with no separate `Item.ID`),
`Checkout`/`Cancel` (asymmetric error reporting on `Cancel` — distinguishing
already-checked-out from already-canceled — versus `AddItem`/`RemoveItem`'s
single collapsed `ErrBasketNotOpen`), an explicit `BasketStatus` lifecycle
guard. `BasketRepository` interface following `StoreRepository`'s shape
(`Save`/`FindByID`/`FindByIDForUpdate`/`WithLock`).

**Step 2 — Domain test suite.** Full coverage of every guard/error path and
every event emission, mirroring `store_test.go`'s density.

**Step 3 — `events` package.** Envelope/payload/subject/stream, structurally
identical to `stores/internal/events`, four files. `ItemAdded`'s payload carries
the running `Quantity` total (a deliberate decision, tested explicitly via a
repeat-`AddItem` round-trip case `stores` never needed).

**Step 4 — `catalog` package.** Plain `Store`/`Product` read-model structs,
deliberately separate from `domain` (no invariants, no events) — the projector
is the writer, `application` is the reader. Repository interface went through
one real redesign mid-Step: an initial upsert-whole-aggregate shape was replaced
with one narrow method per consumed event (`RenameStore`,
`SetStoreParticipation`, `AdjustProductPrice`, etc.), since most of `stores`'
events carry only a changed field, not the aggregate's full state, and the
original shape would have required a read-before-write this package was
deliberately built to avoid.

**Step 5 — `postgres` package.** `BasketRepository` (delete-then-reinsert
`basket_items` sync on every `Save`, chosen over an initially-implemented
diff-based sync for simplicity, given realistic basket sizes),
`CatalogRepository` (narrow per-event methods matching Step 4's interface). Two
migrations (`baskets`/`basket_items`; `catalog_stores`/`catalog_products`) plus
a third, added mid-Step, adding `catalog_products.last_applied_seq` — see Issues
Found below.

**Step 6 — `application` layer.** `StartBasket`, `AddItem` (the first use case
in either service depending on two repositories in one transaction —
`BasketRepository` and `catalog.Repository`, both resolving to the same `pgx.Tx`
via `executorFrom`), `RemoveItem`, `Checkout`, `CancelBasket`, `GetBasket`
(thin, per ADL 0008). The catalog-projection handler (`catalog.Project`)
replacing the Phase 2 placeholder — decodes `stores`' wire-format events via
locally-defined envelope/payload types (per ADL 0004, no import of `stores`' own
packages), dispatching to `catalog.Repository`'s narrow methods.

**Step 7 — `connectapi`.** `basketspb/api.proto` (six RPCs; `StartBasket` takes
no input fields at all, unlike every one of `stores`' `Create*` requests),
`Server`/`errors.go`/`proto.go`/`basket.go`, wired into `module.go` and
`main.go`.

**Step 8 — Integration verification.** Full `internal/application` suite run
file-by-file against real Postgres/NATS (all six use cases, including the
retry-under-NATS-outage test). A live, scripted, end-to-end curl verification
against real `stores` + `baskets` + Postgres + NATS together — quantity
increment/decrement, checkout, four distinct negative cases, and a direct
`catalog_products` query confirming `AdjustProductPrice`'s delta math and
sequence guard.

### Issues Found & Resolved

1. **`BasketStatus` constant names collided with event type names**
   (`BasketCanceled` as both a status constant and an event struct) in the same
   package — caught at first compile attempt, fixed by renaming every status
   constant with a `Status` suffix (`BasketOpenStatus`, etc.).
1. **`catalog.Repository`'s original upsert-whole-aggregate design was
   unworkable** for any event carrying only a partial field change (name change,
   participation toggle, price delta) — would have required a read-before-write
   this package was explicitly built to avoid. Caught before any Postgres code
   was written, redesigned to one narrow method per event shape.
1. **`AdjustProductPrice`'s delta-based update was not safe under message
   redelivery** — applying `price = price + delta` twice for the same logical
   event (a real path: handler succeeds, crash/ack-failure before `msg.Ack()`,
   JetStream redelivers) would silently double-apply the adjustment, unlike
   every other catalog write (absolute upserts, naturally idempotent). Fixed
   with a `last_applied_seq` guard column and a conditional
   `UPDATE ... WHERE last_applied_seq < $seq`, using the raw JetStream stream
   sequence number as the dedup key. Caught by reasoning through the redelivery
   path before writing the naive version into production code, not discovered by
   a failing test.
1. **A genuine cross-service publish-duplication bug (ADL 0015)** — the first
   real-infrastructure NATS-outage retry test
   (`TestStartBasket_Execute_SucceedsAfterNATSRestart`) passed on its own terms,
   but inspecting the actual stream afterward (`nats stream info baskets`)
   showed 4 stored messages where 2 were expected — `nats.go`'s client-side
   reconnect buffer had silently queued and later flushed "failed" retry
   attempts as genuine, separately-acked, duplicate messages. Fixed via
   `Nats-Msg-Id`-based idempotent publish (JetStream's own built-in
   `Duplicate Window` mechanism, present by default but never previously used by
   either service), verified by rerunning the same test and confirming the
   stream now advances by exactly 1 sequence number per logical call instead of
   3\. Backported to `stores` same-day, since it shares the identical
   vulnerability via the same `foundation/nats.Publisher.Publish` call.
   `PublishDeadLetter` (also in `foundation/nats`) was independently found to
   share the same unguarded-`Publish` call and patched at the same time.
1. **`AddItem` returned the wrong error class for a blank `product_id`** —
   `404 NotFound` instead of `400 InvalidArgument` — because the catalog
   existence check (chosen to run first, inside the same transaction, per the
   Option 2 design decided earlier in the Phase) ran before the domain's own
   blank-ID guard ever got a chance to fire. A blank string is a well-formed (if
   useless) SQL parameter, so it resolved to `catalog.ErrProductNotFound` rather
   than `domain.ErrProductIDIsBlank`. Caught by the live end-to-end verification
   script, not by any earlier unit or integration test — none of which exercised
   a blank `productID` through the full `application`-layer transaction. Fixed
   with an early, fail-fast blank check ahead of `WithLock` entirely.
1. **A dead, silently-no-op `case` block in
   `stores/internal/connectapi/errors.go`** — four `errors.Is(...)` calls
   written as standalone statements rather than case conditions, meaning
   `ErrProductNameIsBlank`, `ErrProductPriceIsNegative`, and
   `ErrNotAPriceDecrease` (checked twice, redundantly) never actually matched
   their intended `case`, silently falling through to `CodeInternal` (500)
   instead of `CodeInvalidArgument` (400). Found and fixed while writing
   `baskets`' own `errors.go` for Step 7, by comparing the two files' shapes
   directly — a pre-existing `stores` bug, unrelated to any `baskets`-specific
   work, that had gone unnoticed since whenever it was originally written.

### Key Concepts Reinforced

**Verifying actual infrastructure state, not just a use case's return value.**
The Phase's single most consequential finding (ADL 0015) was only caught because
a passing test's _result_ wasn't treated as the end of the question —
`nats stream info` was run afterward specifically to check whether the
underlying stream matched what the test's success implied. Same discipline as
ADL 0010's and ADL 0012's dedicated timing tests, now extended to publish-side
durability.

**Validation order determines which error a caller actually sees.** Issue #5 is
the general lesson: when multiple checks could reject the same input, whichever
one runs _first_ wins, regardless of which one is semantically more correct for
that specific failure. `AddItem` is the first use case in either service where a
cross-repository check runs ahead of the domain aggregate's own guard, and it's
the first place this ordering actually had an observable, wrong-facing
consequence.

**JetStream's native idempotent-publish mechanism (`Nats-Msg-Id` +
`Duplicate Window`) is the correct fix for publish-outcome ambiguity** — better
than disabling client-side reconnect buffering, which would only have addressed
the one narrow mechanism this investigation happened to surface, not the general
"did my write land or not" problem inherent to any request/response protocol
under network instability.

**A design that looks structurally sound on paper can still fail once the event
shapes it needs to consume are examined concretely.** `catalog.Repository`'s
redesign (Issue #2) happened _before_ any bug was written, purely from walking
through what each of `stores`' actual event payloads carries — a reminder that
"does this interface's shape match the data it needs to handle" is worth
checking against real payloads, not just against the abstract idea of "a
repository for this read model."

### State at End of Phase 3

- `baskets` is fully live: `domain`, `events`, `catalog`, `postgres`,
  `application`, `connectapi` all built, unit-tested (`domain`/`events`),
  integration-tested against real Postgres/NATS (`application`, full
  six-use-case suite plus the NATS-outage retry test), and verified end-to-end
  via live HTTP/ConnectRPC calls against a running `stores`.
- The catalog projector is real, not a placeholder — consuming `stores`' full
  event set, applying changes via narrow per-event methods, with a
  sequence-guarded idempotent price adjustment.
- ADL 0015 (idempotent publish via message ID) is adopted system-wide, applied
  to both `stores` and `baskets`.
- **Known, deliberately deferred gaps, carried forward rather than forgotten:**
  - The `last_applied_seq` guard has only been proven to compute the _correct_
    result under normal operation (verified via the live script's price-math
    check) — not proven to actually block a genuine redelivery-after-crash
    duplicate, which would require a dedicated failure-injection test (killing
    the projector mid-handler, after Postgres commits but before `msg.Ack()`),
    the same "prove the failure mode, then prove the fix" discipline ADL
    0010/0012 each used.
  - The live end-to-end script's `sleep 2` between publishing a `stores` event
    and expecting `baskets`' catalog to reflect it is a timing assumption, not a
    guarantee — worked reliably in this environment, but isn't a deterministic
    wait.
  - `BasketCheckedOut`/`BasketStarted`/`BasketCanceled` carry empty payloads —
    whether `ordering` (Phase 3 in the roadmap's numbering, but a different,
    later project than this one) needs more than a bare basket ID from
    `BasketCheckedOut` specifically is still unknown and worth revisiting once
    that project's actual needs are concrete.
  - The catalog projector's event-name matching (`"StoreCreated"`, etc., as
    local string literals) has no compiler-enforced link to `stores`' own event
    constants — a silent rename on `stores`' side would degrade silently (logged
    warning, skipped projection) rather than fail loudly.

### Carried Into Phase 4 (`depot`)

- The same architecture — narrow per-event catalog-style repository methods (not
  upsert-whole-aggregate), sequence-guarded idempotent handling for any
  delta-based projection, `Nats-Msg-Id`-based idempotent publish from the very
  first line of `depot`'s own `publishDomainEvent` rather than needing the ADL
  0015 discovery repeated — should be `depot`'s starting point, not something to
  re-derive.
- The validation-ordering lesson (Issue #5): any `depot` use case combining a
  domain-level input guard with a cross-repository lookup should run the cheap,
  local guard first.
- Standing items, not forgotten: the `BasketCheckedOut` payload-thinness
  question, and the still-owed redelivery-failure-injection test for the
  sequence-guard pattern — worth addressing once `depot`'s own equivalent (if it
  needs one) makes the second item concrete across two services instead of
  hypothetical for one.
