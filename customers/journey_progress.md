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

## Phase 1: Domain

### Step 1: Scaffold and events

(in progress)
