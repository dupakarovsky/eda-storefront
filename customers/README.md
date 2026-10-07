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
