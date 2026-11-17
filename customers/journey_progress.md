# Customers: Journey Progress

## Goal

Build the `customers` service: the system's second root. It applies the
conventions from ADLs 0007-0018 from the first line.

## Decisions carried in

- Customers is built before Payments (ADL 0019, to be written in Phase 6)
- Current-state persistence with row-level locking (ADL 0002, 0010)
- Server-generated IDs (ADL 0009)
- Queries route through the application layer (ADL 0008)
- Idempotent publish via `Nats-Msg-Id` (ADL 0015)
- Independent-context rollback (ADL 0018)
- Unique proto filename: `customers.proto` (ADL 0017)
- Outbox decision deferred to the Payments kickoff
- Contract evolution (price, customer threading) is a separate project after
  this one

## Differences from the book

- `ChangeSmsNumber` fully implemented (the book declares it but never implements
  it)
- `AuthorizeCustomer` is a synchronous RPC with no event
- Updates are locked and state-guarded

## Phases

1. Domain
1. Persistence
1. Events and application layer
1. ConnectRPC API
1. Containerization
1. Close-out

## Phase 1: Domain (complete)

### What was built

- `Customer` aggregate: ID, name, SMS number, enabled (starts enabled)
- Events: `CustomerRegistered`, `CustomerSmsChanged`, `CustomerEnabled`,
  `CustomerDisabled`
- Methods: `NewCustomer`, `ChangeSmsNumber`, `Enable`, `Disable`, `Authorize`
  (read-only, no event)
- `ReconstituteCustomer` for rehydration from the database

### Decisions

- Event names are unprefixed, matching `stores`/`baskets`/`depot`/`ordering`
- Name and SMS number are trimmed, and the trimmed value is what is stored and
  published
- Changing to the same SMS number is an error
- No ID validation in the domain (the handler generates a UUID per ADL 0009)
- `Authorize` emits no event; revisit if a consumer ever needs one

### Verification

- `go test ./customers/internal/domain/...` passes

### Carried into Phase 2

- `domain.CustomerRepository` interface and `ErrCustomerNotFound`
- Provisioning `customersdb`

## Phase 2: Persistence (complete)

### What was built

- `customersdb` provisioning script (`07-customers-db.sh`), applied manually to
  the long-lived volume
- Migration `000001_create_customers_table` with `CHECK (btrim(...) <> '')` on
  name and sms_number
- `domain.CustomerRepository` and `ErrCustomerNotFound`
- `internal/postgres`: `Executor`, context-scoped transaction (ADL 0010),
  `CustomerRepository`
- `WithLock` rolls back on an independent 5s context (ADL 0018) from the first
  commit

### Findings

- `btrim(x)` strips only the ASCII space; Go's `TrimSpace` strips all Unicode
  whitespace. The CHECK constraints are a coarse backstop; the domain is the
  real validator.
- Lock test measures from the moment goroutine A holds the lock, and asserts B
  reads A's committed value
- First test that proves ADL 0018's claim: rollback succeeds after the caller's
  context is canceled

### Verification

- All 8 repository tests pass as `eda_user`
- Breakage 1 (repository bypasses the tx executor): lock test fails, B returned
  in 1.8ms
- Breakage 2 (rollback on caller ctx): only the expired-context test fails

### Carried into Phase 3

- Stream `customers`, subject `customers.customer.*` (no dead-letter subject,
  since Customers consumes nothing)

<!-- customers/journey_progress.md (replace the Phase 3 section) -->

## Phase 3: Events and application layer (complete)

### What was built

- `internal/events`: payloads, envelope, subject mapping, stream `customers`
  (`customers.customer.*`), wire format pinned in tests
- `publishWithRetry` / `publishDomainEvent`: a fifth local copy; promotion into
  `foundation` deferred (it would also drag over `NewEnvelope` and `SubjectFor`)
- Six use cases: RegisterCustomer, ChangeSmsNumber, EnableCustomer,
  DisableCustomer (locked read-modify-write), AuthorizeCustomer, GetCustomer
  (unlocked reads)
- `module.go` without a connect server (added in Phase 4)

### Decisions

- Commands return the saved customer alongside a publish error (ADL 0007
  contract)
- Publish happens after commit, outside the row lock
- Use cases stay explicit (no shared helper), matching earlier services
- `AuthorizeCustomer` reads without a lock; it can race with a concurrent
  disable (document in CONTRACT.md)

### Findings

- The retry loop is tested on a virtual clock (`testing/synctest`) with exact
  timing assertions, including the 10.5s worst case from ADL 0007

- Use-case concurrency test (4 simultaneous disables) is deterministic, not
  timing-dependent: without `FOR UPDATE` all 4 succeed

- A hang during development came from a use-case bug in the locked section
  (cause to be confirmed)

- A real 10s outage closes the NATS client permanently (`MaxReconnects=5` x
  `ReconnectWait=2s`); `RetryOnFailedConnect` covers initial connect only. All
  services share this default. Affects ADL 0011's claim about recovery from
  independent restarts.

- The hang during development was `Save(ctx)` instead of `Save(txCtx)` inside
  `WithLock`: the self-deadlock ADL 0018 describes. Running with `-timeout 30s`
  turns a recurrence into a fast failure.

- A real 10s NATS outage permanently closed every service's connection
  (`MaxReconnects=5`). Fixed system-wide by changing the `foundation/config`
  default to -1 (ADL 0011 addendum). A 15s-outage test fails with the old value
  and passes with the new one.

### Verification

- Retry loop: three breakages caught (sleep after final attempt, unbounded
  attempt, changed backoff)
- Use cases: dormant lock (4/4 runs fail), save-without-publish caught
- NATS outage: 1 message for 1 logical publish; fresh-ID-per-attempt gives 3;
  exhausted retries leave a saved customer with no event

### Carried into Phase 4

- `customers.proto`, buf generation, error mapping, server, `cmd/service`

## Phase 4: ConnectRPC API (complete)

### What was built

- `customerspb/customers.proto` (package `customers.v1`, filename unique per ADL
  0017), `buf.yaml`, `buf.gen.yaml`
- `internal/connectapi`: `mapError`, `toProtoCustomer`, `Server`, six handlers
- `module.go` gains `ConnectServer`; `cmd/service` (main, health);
  `make customers/run`
- `customers-http-connect.sh`: 18-step curl verification, including every status
  code

### Decisions

- Error codes: not found = `NotFound`; blank name/number and unchanged number =
  `InvalidArgument` (unchanged number matches `stores`' same-price case);
  already enabled/disabled = `FailedPrecondition`; disabled customer on
  authorize = `PermissionDenied`
- Handlers stay thin: no input validation at the RPC layer. The application
  layer builds and validates aggregates before any repository call, consistent
  with the other services and ADL 0008. A UUID-shape and length-cap proposal was
  considered and rejected.
- Responses return the whole `Customer`, not only an ID as in the book

### Findings

- proto3 JSON omits `enabled: false`; clients must read a missing `enabled` as
  false (CONTRACT.md)
- A malformed ID like `does-not-exist` returns `NotFound`, as in the other
  services
- Error messages expose the wrapped use-case chain to clients, as elsewhere in
  the system
- Tranche 2 note for ADL 0020: `ordering` should retry the authorize call only
  on `Unavailable`, `DeadlineExceeded`, and `Internal`, and treat every other
  code as permanent. An allowlist of permanent codes would retry requests that
  can never succeed.

### Verification

- 18-step curl script: all statuses as designed; 4 events published for 4
  successful state changes, none for reads, authorizations, or rejected calls

## Phase 5: Containerization (complete)

### What was built

- `customers/Dockerfile` (`golang:1.26` builder, copies only `foundation` and
  `customers`), compose service on host port 8084, `logs/customers` and
  `kill/customers` targets
- No `depends_on` on other services: Customers is a root (ADL 0001)

### Findings

- The container reads the same `customersdb` as the local run; `DB_HOST` and
  `NATS_URL` are overridden by the compose environment
- Graceful shutdown on SIGTERM logs the full sequence (signal, goroutine exit,
  shutdown complete) and closes NATS and Postgres after `Run` returns, as ADL
  0012 requires
- With no `NATS_MAX_RECONNECTS` override anywhere, the `foundation` default
  alone carried all five services through a 15s NATS outage

### Verification

- 18-step curl script against the container: same statuses and events as the
  local run
- `make build` → `make down` → `make up`: five services healthy, data intact
- 15s NATS outage: five healthy; a new customer registered; a new store reached
  both catalog caches

## Phase 6: Close-out (complete)

### What was built

- `CONTRACT.md`, including the status-code table and retry guidance for callers
- `docs/system-state.md` and `docs/roadmap.md` updated; ADL 0019 and ADL 0020 written; ADL 0011 and ADL 0018 addenda

## Project summary

Customers is live: a root service with six RPCs, four events, and no
consumers. Verification: 8 domain test groups, 8 repository tests (as
`eda_user`, with lock and expired-context rollback tests), 6 fake-JetStream
retry tests on a virtual clock, 14 use-case tests against real Postgres and
NATS, 2 real-outage tests, 1 reconnect test, an error-mapping unit test, an
18-step curl script against the container, and a full `down` → `up` cycle.

## Findings that apply system-wide

- A NATS outage over ~10s permanently closed every service's connection (fixed by `NATS_MAX_RECONNECTS` default -1; ADL 0011 addendum)
- `docker compose kill` does not trigger `restart: unless-stopped`: the container stays down. Observed, not investigated. Whether a real crash restarts it was not tested.
- Use-case `Save(ctx)` inside `WithLock` self-deadlocks (ADL 0018 addendum)
- `btrim` strips only ASCII space; the domain's `TrimSpace` strips all Unicode whitespace
- proto3 JSON omits a `false` boolean

## Handoff: next session is `contract-evolution` (ADL 0020)

Create `contract-evolution/journey_progress.md` (folder only, no `go.mod`).

**First, before any code:**

1. Apply the ADL 0011/0018 addenda and the roadmap/system-state edits above.
2. Run `ordering/ordering-http-connect-2.sh` against the live stack: the one regression not yet seen after the NATS default change.
3. Settle the Tranche 2 open questions in ADL 0020.

**Proposed phases:**

1. Tranche 1a: pre-check query; `stores` two-decimal rule; absolute `price` on the two price events; projection in `baskets` and `depot` switched to absolute with `last_applied_seq`, tolerating events without `price`; `000004` migrations; delta-reorder test
2. Tranche 1b: `Checkout` resolves price (replace `FindStoreIDsByProductIDs`); `BasketCheckedOut.price`; `ordering` `Item.Price`, `order_items.price`, `GetTotal()`, `OrderCreated`/`OrderReadied` enrichment; four contracts
3. Tranche 2a: `baskets` `customer_id` (domain, migration `000005`, events, RPC, every test and script)
4. Tranche 2b: `ordering` `CustomerID`, `customersclient`, authorization with the retry classification, `rejected` status and `OrderRejected`, Dockerfile/`go.mod`/compose/config wiring
5. Close-out: contracts, `system-state.md`, ADL status

**Housekeeping to fold in when touching those files:**

- `baskets/checkout.go`: `fmt.Errorf("getting store ids")` drops the cause (no `%w`); `FindStoreIDsByProductIDs` lacks `defer rows.Close()`
- `foundation/nats/connect.go`: the comment saying `MaxReconnects` 0 means "no reconnects" is wrong (the library default applies); `foundation/config/config.go`: remove the commented-out old line and the "Defult" typo
- `.gitignore`: `customers/bin/`, `*.log`

**Still open, outside this project:** the outbox decision and the money-type decision, both before `payments`; promoting `publishWithRetry`/`publishDomainEvent` into `foundation` (the retry loop now has tests the other four copies lack); backporting the expired-context rollback test; the `restart` behavior above.

**Attach to the next session (ADL 0004 amended: this project modifies four live services, so their source is a real dependency, not reference):** `docs/` in full; `foundation/` in full; full source of `stores`, `depot-baskets`, `ordering`; `customers/CONTRACT.md`, `customers/customerspb/` (generated code, since `ordering` will import it), and `customers/customers-http-connect.sh`; `docker-compose.yaml`; `Makefile`; `postgres/init/`.
