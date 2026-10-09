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
