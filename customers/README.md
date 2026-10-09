# Customers, Phase 1: Domain

Phase 1 builds the `Customer` aggregate with no infrastructure, only pure Go and
unit tests.

- **Step 1:** project scaffold and events (this message)
- **Step 2:** the `Customer` aggregate and its errors
- **Step 3:** unit tests

## Step 1: Scaffold and events

### 1.1 Folders

```bash
mkdir -p customers/internal/{domain,application,events,postgres,connectapi}
mkdir -p customers/{cmd/service,migrations,customerspb}
```

### 1.2 `journey_progress.md`

### 1.3 `go.mod`

```go
// customers/go.mod
```

Dependencies and the `foundation` replace directive come in Phase 2, when
something imports them.

### 1.4 `go.work`

```go
// go.work (add ./customers to the use block)
```

### 1.5 `internal/domain/events.go`

```go
// customers/internal/domain/events.go
```

### What to notice

- **Same shape as `stores`.** The embedded `eventBase` supplies `AggregateID`
  and `OccurredAt`, so each concrete event only implements `EventName`. This is
  the pattern ADL 0002 settled on.
- **Unprefixed event names.** The book uses `customers.CustomerRegistered`. Ours
  follows our own contracts (`StoreCreated`, `BasketStarted`). The service name
  lives in the NATS subject (`customers.customer.CustomerRegistered`), not in
  the event name.
- **Payloads match the book's integration events.** `CustomerRegistered` carries
  name and SMS number, and `CustomerSmsChanged` carries the SMS number.
  `CustomerEnabled` and `CustomerDisabled` carry no data beyond the aggregate
  ID, so `notifications` and `search` can be built later against the same
  shapes.
- **No `CustomerAuthorized` event.** The book emits one, but nothing consumes
  it. Authorization is a read-only check here.

## Check-in

Please create the files and confirm that `go build ./...` (run from
`customers/`) succeeds. If you see an error, send the output.

Two questions for review:

1. Is the event naming convention OK, or would you prefer the book's
   `customers.`-prefixed names?
1. Step 2 involves a few judgment calls I'd like your input on: the blank checks
   will use `strings.TrimSpace` (so `"  "` counts as blank, unlike `stores`), a
   new customer starts enabled, and `ChangeSmsNumber` to the same number is an
   error, like `IncreasePrice` to the same price. Are you comfortable with those
   defaults?

## Step 2: The `Customer` aggregate

### 2.1 `internal/domain/customer.go`

```go
// customers/internal/domain/customer.go
```

### What to notice

- **Same constructor shape as `stores`.** `NewCustomer` returns
  `(*Customer, Event, error)`, and every mutating method returns
  `(Event, error)`. The application layer will capture the event, save the
  aggregate, then publish.
- **Trimmed values are what we store.** The checks use `TrimSpace`, and the
  aggregate keeps the trimmed value. The event then carries exactly what was
  persisted, so `" Alice "` can't be stored with a different value than
  consumers see. This is one small step past the default you approved, so tell
  me if you'd rather keep the raw input.
- **Guards run before mutation.** A rejected `ChangeSmsNumber` leaves the
  customer unchanged and returns a nil event. This is the same discipline as
  `Product.Rebrand`, and Step 3 will test it.
- **`Authorize` is read-only.** It returns only an error, with no event and no
  state change. The same check can be done on a freshly loaded row without
  taking a lock, which fits ADL 0008's pass-through query shape. In Phase 4,
  `ErrCustomerNotAuthorized` will map to a permanent code (`PermissionDenied`),
  the signal Tranche 2 uses to reject an order instead of retrying.
- **Everything else matches the book.** A new customer starts enabled, and
  enable/disable are idempotency-guarded like the store participation toggles.
- **No ID check in `NewCustomer`.** The book rejects a blank ID. Ours follows
  `NewStore`, because per ADL 0009 the handler generates the ID, so a blank one
  can't come from a caller.

## Check-in

Please create the file and run `go build ./...` from `customers/`. Then we can
move to Step 3, the unit tests: table-style `t.Run` subtests for every
constructor and method, each success path and each guard, checking event type,
payload, and unchanged state on failure.

If you'd like a different error wording or a different set of guards before the
tests lock them in, say so now, since changing them later means touching the
tests too.

## Step 3: Unit tests

### 3.1 `internal/domain/customer_test.go`

```go
// customers/internal/domain/customer_test.go
```

### What to notice

- **Failure paths check state, not just errors.** Each guard test asserts a nil
  event, and where it matters, that the customer is unchanged. The `Authorize`
  test copies the struct (`before := *customer`) and compares it afterward. That
  works because `Customer` has only comparable fields.
- **Ordering of guards is pinned.** The "name reported before sms number" test
  locks in that when both are blank, the name error wins. If we reorder the
  checks later, the test says so.
- **Trimming is tested on both sides of the output.** The aggregate and the
  event payload must carry the trimmed value. This is the property that would
  break silently if someone trimmed only for the check.
- **No ID test.** Nothing validates `id`, by design (ADL 0009), so there's
  nothing to assert.

## Run it

From `customers/`:

```bash
go test ./internal/domain/ -v
go vet ./...
```

All subtests should pass.

## Prove the tests catch bugs

Following the discipline from ADLs 0010 and 0012, don't trust tests you've never
seen fail. Try these two temporary breakages and confirm the named tests fail.
Revert each one afterward:

1. **Mutate before guarding.** In `ChangeSmsNumber`, move
   `c.SmsNumber = smsNumber` above the `if smsNumber == "" { ... }` check. Then
   "blank number fails and does not mutate customer" should fail.
1. **Drop the trim on the stored value.** In `NewCustomer`, delete the
   `name = strings.TrimSpace(name)` line. Then the "trimmed" subtest and the
   whitespace-only name test should fail.

## Check-in

Please run the tests and send me the output, plus the result of the two breakage
experiments if you try them. If everything is green, Phase 1 is complete, and
I'll give you the updated `journey_progress.md` section recording the phase
before we start Phase 2 (persistence).

Two review questions:

1. Do you want anything else covered, such as a test that `NewCustomer` accepts
   an empty ID, to document the ADL 0009 decision?
1. Are you comfortable that `Authorize` carries no event, or should we revisit
   that before it's wired through the application layer in Phase 3?

## Phase 2: Persistence

- **Step 1:** database provisioning, migration, `.env`, Makefile targets (this
  message)
- **Step 2:** repository interface, `postgres` package (`Executor`, tx context,
  repository with `WithLock`)
- **Step 3:** repository integration tests, including a lock-proving concurrency
  test and a rollback test for the ADL 0018 gap

## Step 1: Provisioning and migration

### 1.1 `postgres/init/07-customers-db.sh`

```bash

```

### 1.2 `docker-compose.yaml`

```yaml
# docker-compose.yaml (postgres.volumes: add after the 06 line)
```

### 1.3 Apply it to your existing volume

As in `system-state.md`, init scripts only run on a fresh volume, so apply it
once by hand. Run `make exec/pg`, which connects as `edasuper`, then paste:

```sql

```

The `\c` switch matters. Without it the schema grants would land on `edadb`'s
`public` schema, which is the trap the comment in `06-ordering-db.sh` warns
about.

### 1.4 Migration

```sql
-- customers/migrations/000001_create_customers_table.up.sql
```

```sql
-- customers/migrations/000001_create_customers_table.down.sql
DROP TABLE customers;
```

### 1.5 `.env`

```bash
# customers/.env
```

### 1.6 `Makefile`

```make

```

```make

```

### 1.7 Verify

```bash
make migrate/customers/up
make customersdb/dev
```

```sql
\d customers
\dp customers
```

You should see the six columns and both CHECK constraints. `\dp` should show
`eda_user` with `arwd` (insert, select, update, delete). If you want to see the
constraint working, try this:

```sql

```

It should be rejected.

### What to notice

- **The CHECK constraints are a second layer, not a replacement.** The domain
  already trims and rejects blanks. The constraint protects against a future
  code path, or a manual `psql` edit, writing a blank row. `stores` does the
  same with `CHECK (price >= 0)`.
- **`enabled DEFAULT TRUE` mirrors the domain.** `Save` will still write the
  value explicitly, so the default only matters for manual inserts.
- **Migration filenames are zero-padded.** That matches `stores`, `baskets`, and
  `depot`, and it's what `golang-migrate` expects.

## Check-in

Please apply the steps and send me the output of `\d customers` and
`\dp customers`. One question for review: are you OK with the two CHECK
constraints? They go beyond what `stores` does, so it's your call. Dropping them
is a one-line change to the migration.

## Step 2: Repository interface and `postgres` package

### 2.1 `internal/domain/customer_repository.go`

```go
// customers/internal/domain/customer_repository.go
```

### 2.2 `internal/postgres/interface.go`

```go
// customers/internal/postgres/interface.go
```

### 2.3 `internal/postgres/txcontext.go`

```go
// customers/internal/postgres/txcontext.go
```

### 2.4 `internal/postgres/customer_repository.go`

```go
// customers/internal/postgres/customer_repository.go
```

### 2.5 Dependency

From `customers/`:

```bash
go get github.com/jackc/pgx/v5@v5.11.0
go build ./...
```

### What to notice

- **One `find` helper serves both reads.** The only difference is a `FOR UPDATE`
  suffix, the same shape as `baskets` and `depot`. The `FOR UPDATE` form is only
  meaningful inside `WithLock`, because outside a transaction the lock is
  released the instant the statement ends.
- **Every method resolves its executor through `executorFrom`.** This is the
  load-bearing rule from ADL 0010. A method that used `r.db` directly would
  silently run outside the lock.
- **The rollback runs on a fresh `context.Background()` with a 5-second
  budget.** This is the ADL 0018 shape from day one, so `stores`' product
  repository bug can't recur here.
- **`Save` is an upsert, as in `stores`.** A duplicate ID would overwrite
  silently. That's acceptable because IDs are server-generated UUIDs (ADL 0009).
- **No lock in `FindByID`.** `GetCustomer` and `AuthorizeCustomer` are reads and
  will use it (ADL 0008 keeps the use-case layer as the only caller).

## Check-in

Please create the files and confirm `go build ./...` succeeds from `customers/`.
Step 3 adds the integration tests against real Postgres, including a timing test
for the lock and a test that rollback works after the caller's context has
expired, which is the property ADL 0018 admitted it never proved. Those tests
need a connection pool, so I'll have them use `foundation/postgres.NewPool`
through a small test helper.

One decision before then: do you want those tests to run against `customersdb`
directly, like the other services' tests (unique IDs per test, no cleanup), or
would you rather have them clean up their own rows?

## Step 3: Repository integration tests

These run against `customersdb` directly, with a unique ID per test and no
cleanup, like the other services.

### 3.1 `go.mod`

The tests import `foundation`, so the module needs the same `replace` pattern as
`ordering`:

```go
// customers/go.mod
```

Then, from `customers/`:

```bash
go mod tidy
```

If `tidy` complains about anything, send me the output.

### 3.2 `internal/postgres/customer_repository_test.go`

```go
// customers/internal/postgres/customer_repository_test.go
```

### What to notice

- **The lock test measures from the moment A took the lock.** In the `stores`
  version, B started its timer after A had already begun sleeping. B's measured
  wait could therefore come out slightly under 300ms even with a working lock.
  Here A stamps `lockedAt` right after acquiring the lock. B cannot return
  before A commits, and A commits after sleeping `holdDuration`, so
  `returnedAt - lockedAt >= holdDuration` must hold whenever the lock works.
- **B also asserts what it read.** Blocking alone doesn't prove correctness. B
  must read A's committed value, which shows that B actually waited for the
  commit instead of reading stale data.
- **`signal` is idempotent and deferred in A.** If A fails before taking the
  lock, B is released instead of hanging until the test timeout.
- **The expired-context test is new.** ADL 0018 admitted that this property was
  never proven. The callback takes the row lock, cancels the caller's context,
  and returns an error. If rollback used the caller's context, `Rollback` would
  fail on the dead context. `WithLock` would then return the rollback error, and
  `errors.Is(err, errBoom)` would be false. The second half then confirms the
  row lock is free, using a fresh 2-second context. The context cancellation is
  deliberately inside the callback so it happens after the lock is held.

### 3.3 Run it

From `customers/`:

```bash
go test ./internal/postgres/ -v
```

### 3.4 Prove the tests catch bugs

Try these temporary breakages, one at a time, and revert each:

1. **Dormant lock.** In `find`, change `exec := executorFrom(ctx, r.db)` to
   `exec := r.db`. Then `SerializesConcurrentAccess` should fail. B no longer
   blocks, and it reads the old value.
1. **Rollback on the caller's context.** In `WithLock`, change
   `tx.Rollback(rbCtx)` to `tx.Rollback(txCtx)`. Then
   `RollsBackAfterCallerContextExpires` should fail with a "rolling back
   transaction" error. `RollsBackOnCallbackError` should still pass, because its
   context is alive. That asymmetry shows the new test checks something the old
   one can't.

## Check-in

Please send the passing output. If you run the breakage experiments, send those
results too, and I'd particularly like to see experiment 2, since this is the
first test that proves ADL 0018's claim instead of inferring it.

One question for review: the `.env` here uses `eda_user` (the application role),
so these tests exercise the same privileges the service will have. Is that what
you want, or should tests use `eda_dev` like the migrations? I'd keep `eda_user`
because a missing grant would surface here instead of in the container later.

## Step 2: `publishDomainEvent` and `publishWithRetry`

### 2.1 `internal/application/publish_retry.go`

```go
// customers/internal/application/publish_retry.go
```

### 2.2 `internal/application/publish_event.go`

```go
// customers/internal/application/publish_event.go
```

### 2.3 Dependencies

From `customers/`:

```bash

```

These are the versions `ordering` already uses, so the workspace stays
consistent.

### What to notice

- **One `msgID` per logical publish, generated before the loop.** The same value
  goes to every attempt. This is the ADL 0015 rule. A fresh ID per attempt would
  defeat JetStream's duplicate detection, and the failure would be invisible
  unless a test inspects the stream.
- **Every publish attempt is bounded.** `publishAttemptTimeout` (1.5s) comes
  from the forced-outage test in ADL 0007, and the five attempts with
  200/400/800/1600ms delays give the ~10.5s worst case. These are the same
  constants as the other four services.
- **The failure contract is the same.** When retries run out, the caller gets an
  error and the aggregate stays saved. Use cases will return the saved aggregate
  alongside the error so a caller can tell "nothing happened" from "saved but
  not published".
- **Small cleanups from the earlier copies.** I renamed the `attemp` typo to
  `attempt`, and `attempCtx` to `attemptCtx`. Behavior is identical.
- **No unit test for the retry loop yet.** `Publisher` is a concrete type
  wrapping the `jetstream.JetStream` interface, so a fake is possible but
  involves a large interface. The real proof is Step 5, which kills and restarts
  NATS and checks the stream's message count.

## Check-in

Please create the files, confirm `go build ./...` succeeds, and send me the Step
1 test output as well.

One question for review: do you want me to write a fake-JetStream unit test for
the retry loop (exact attempt counts, backoff timing, context cancellation
mid-wait), or rely on Step 5's real-outage test alone? The fake would cover
cases that are awkward to force against real NATS, like cancellation during a
backoff wait. It costs roughly 40 lines of test scaffolding. It would also make
a good base if we promote `publishWithRetry` into `foundation` later.

## The fake JetStream retry test

Two things make this test cheap and exact:

- **The fake embeds the `jetstream.JetStream` interface** and overrides only
  `Publish`. This is the same trick as `fakeMsg` in `ordering`'s tests. Any
  other method call would panic on the nil embedded interface, which tells us
  the code touched something we didn't expect.
- **`testing/synctest` gives the test a fake clock.** Inside a bubble,
  `time.After`, `context.WithTimeout`, and `time.Now` run on virtual time. When
  every goroutine is blocked, the clock jumps forward instantly. We can assert
  the full 10.5s worst case to the millisecond and the test still finishes in
  milliseconds. It's stable in Go 1.25+, so your 1.26 toolchain has it, and it
  needs no new dependency.

### `internal/application/publish_retry_test.go`

```go
// customers/internal/application/publish_retry_test.go
```

### Run it

From `customers/`:

```bash
go test ./internal/application/ -run PublishWithRetry -v
```

It should finish in milliseconds even though the scenarios represent up to 10.5
seconds of virtual time.

### What to notice

- **The timing assertions are exact equalities.** Under the virtual clock, real
  work takes zero time, so `elapsed != 3*time.Second` is a deterministic check.
  On a real clock this would need tolerances and would be flaky.
- **The 10.5s number from ADL 0007 is now an assertion.** Five attempts at 1.5s
  plus the 200 + 400 + 800 + 1600ms delays gives 10.5s. A change to any constant
  that breaks the documented budget fails the test, instead of silently
  invalidating ADL 0007 and the `SRV_REQUEST_TIMEOUT` defaults built on it.
- **The two cancellation tests catch different mistakes.** One cancels during
  the backoff wait, and the other cancels while an attempt is in flight. A retry
  loop could handle one correctly and not the other.
- **The "last error" test uses five distinct errors.** With a single repeated
  error, a bug returning the first error would pass undetected.

### Prove the tests catch bugs

Try each one separately and revert it afterward:

1. **Sleep after the final attempt.** Delete the
   `if attempt == publishMaxAttempts { break }` block. The exhaustion test
   should report 6.2s instead of 3s.
1. **Unbounded attempts.** Change the publish call to
   `publisher.Publish(ctx, ...)`, dropping the per-attempt timeout. This is the
   failure ADL 0007 found in a real outage, where one attempt blocked for ~4.7s.
   The bounded-attempt test should fail with a deadlock panic ("all goroutines
   in bubble are blocked") instead of hanging, because synctest detects it.
1. **Flat backoff.** Change the delay to a constant `publishBaseDelay`. The gap
   assertions should fail in both the retry and exhaustion tests.

### What this doesn't prove

- **The message ID is only partly checked.** `jetstream.PublishOpt` is an opaque
  function type, so the fake can't read the ID back. All the test can assert is
  that exactly one option arrives on every call. That the same ID is used across
  attempts is guaranteed by structure, because `publishDomainEvent` generates it
  once before the loop. Whether JetStream actually collapses duplicates is a
  property of the server, so only Step 5 can show it (a forced outage, then a
  stream message-count check).
- **Real reconnect-buffer behavior** is likewise Step 5 territory.

## Check-in

Please send:

1. the output of the retry tests
1. confirmation that `go build ./...` works for Step 2
1. the breakage results, if you try them, especially experiment 2

After that comes Step 3, the six use cases (`RegisterCustomer`,
`ChangeSmsNumber`, `EnableCustomer`, `DisableCustomer`, `AuthorizeCustomer`,
`GetCustomer`).

All four runs behaved as designed. Please confirm the file is back to its
original state (baseline green).

## Reading the results

| Run                       | Result                                                            | What it proves                                                                               |
| ------------------------- | ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Baseline                  | 6/6 pass in 3ms                                                   | 10.5s of virtual time runs in milliseconds                                                   |
| Sleep after final attempt | Exhaustion test: 6.2s vs 3s. Bounded-attempt test: 13.7s vs 10.5s | The extra sleep is the last backoff, 3.2s. Both tests caught it, and I'd only predicted one. |
| Unbounded attempts        | Deadlock panic                                                    | This is the failure ADL 0007 found in a real outage.                                         |
| Changed backoff           | Gaps `[300ms 600ms]`, 4.5s and 12s totals                         | The schedule values are pinned.                                                              |

- **The panic trace shows ADL 0007's failure mode.** In the stack,
  `fakeJetStream.Publish` is parked on `chan receive (nil chan)`. Without the
  per-attempt timeout, `context.Background().Done()` is a nil channel, so the
  fake blocks forever, just as the real client blocked in the ambiguous
  reconnect state. `synctest` found that no goroutine could ever wake and
  panicked instead of hanging. Because a panic aborts the whole test binary, the
  two cancellation tests after it never ran in that experiment. That is normal.
- **The last run wasn't quite the flat-backoff experiment I proposed.** The gaps
  `300, 600, 1200, 2400` are still doubling. That's what a base delay of 300ms
  would produce, not a constant delay. It is still a useful result, because it
  shows the tests pin the exact schedule. It doesn't show they catch the shape
  (exponential vs flat). If you want that one too, make the delay a constant
  `publishBaseDelay`. You should see gaps of `[200ms 200ms]` in the retry test,
  `[200ms 200ms 200ms 200ms]` in the exhaustion test, and 8.3s in the bounded
  test (7.5s of attempts plus 0.8s of waits). It's a one-minute check, so it's
  your call.

The retry loop is verified except for the two things only a real server can
show: message-ID deduplication and reconnect-buffer behavior. Both are Step 5.

## Step 3: The six use cases

Following the existing pattern, each use case is a small struct with an
`Execute` method. Commands that change state use `WithLock`, and the two reads
don't.

### 3.1 `internal/application/register_customer.go`

```go

```

### 3.2 `internal/application/change_sms_number.go`

```go
// customers/internal/application/change_sms_number.go
```

### 3.3 `internal/application/enable_customer.go`

```go
// customers/internal/application/enable_customer.go
```

### 3.4 `internal/application/disable_customer.go`

```go
// customers/internal/application/disable_customer.go
```

### 3.5 `internal/application/authorize_customer.go`

```go
// customers/internal/application/authorize_customer.go
```

### 3.6 `internal/application/get_customer.go`

```go
// customers/internal/application/get_customer.go
```

### What to notice

- **State-changing commands return the customer even when the publish fails.**
  This follows ADL 0007's contract. A caller seeing `(customer, err)` knows the
  row was saved but the event wasn't delivered. A caller seeing `(nil, err)`
  knows nothing happened.
- **`WithLock` wraps only the read-modify-write.** The publish happens after
  commit, outside the transaction, so a slow NATS can't hold a row lock. That is
  also why the retry budget doesn't interact with the lock.
- **`AuthorizeCustomer` reads without a lock.** It doesn't mutate anything, so a
  lock would only serialize authorization checks behind in-flight updates. The
  check can race with a concurrent `DisableCustomer`. That's inherent to any
  authorization decision made on a snapshot, and acceptable here because
  authorization is advisory and the order flow is eventually consistent. I'd
  document it in `CONTRACT.md`. It takes no publisher, which shows in the
  constructor.
- **Wrapped errors keep their identity.** Every `%w` chain still satisfies
  `errors.Is` against `ErrCustomerNotFound`, `ErrCustomerNotAuthorized`, and the
  domain sentinels. Phase 4's `mapError` relies on this.
- **There is no `NotFound` special case.** `RebrandStore` in `stores` has a
  switch that wraps `ErrStoreNotFound` and logs unexpected errors differently,
  but the logging adds nothing the wrapped error doesn't already carry.

## Verify

From `customers/`:

```bash
go build ./...
go vet ./...
```

Behavior tests come in Step 4, because they need the module wiring (a real pool,
publisher, and stream) to run against Postgres and NATS.

## Check-in

Please send the build and vet results. One design question: the three
state-changing use cases `ChangeSmsNumber`, `EnableCustomer`, and
`DisableCustomer` are nearly identical, and a small helper could remove roughly
60 lines. I lean toward keeping them explicit, because every service so far
(`stores` has five of these) does it that way and each use case can diverge on
its own. A shared helper would couple three things that have no reason to change
together. Do you agree, or do you want to extract the helper?
