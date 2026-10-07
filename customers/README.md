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
