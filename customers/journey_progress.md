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

- `customerspb/customers.proto` (package `customers.v1`, filename unique per ADL 0017), `buf.yaml`, `buf.gen.yaml`
- `internal/connectapi`: `mapError`, `toProtoCustomer`, `Server`, six handlers
- `module.go` gains `ConnectServer`; `cmd/service` (main, health); `make customers/run`
- `customers-http-connect.sh`: 18-step curl verification, including every status code

### Decisions

- Error codes: not found = `NotFound`; blank name/number and unchanged number = `InvalidArgument` (unchanged number matches `stores`' same-price case); already enabled/disabled = `FailedPrecondition`; disabled customer on authorize = `PermissionDenied`
- Handlers stay thin: no input validation at the RPC layer. The application layer builds and validates aggregates before any repository call, consistent with the other services and ADL 0008. A UUID-shape and length-cap proposal was considered and rejected.
- Responses return the whole `Customer`, not only an ID as in the book

### Findings

- proto3 JSON omits `enabled: false`; clients must read a missing `enabled` as false (CONTRACT.md)
- A malformed ID like `does-not-exist` returns `NotFound`, as in the other services
- Error messages expose the wrapped use-case chain to clients, as elsewhere in the system
- Tranche 2 note for ADL 0020: `ordering` should retry the authorize call only on `Unavailable`, `DeadlineExceeded`, and `Internal`, and treat every other code as permanent. An allowlist of permanent codes would retry requests that can never succeed.

### Verification

- 18-step curl script: all statuses as designed; 4 events published for 4 successful state changes, none for reads, authorizations, or rejected calls
