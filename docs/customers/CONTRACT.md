<!-- customers/CONTRACT.md -->

# Contract — `customers`

**Status:** Live
**Project folder:** `customers/`
**Last updated:** 2026-10-10

Owns and is the source of truth for customer identity — a customer's name,
SMS number, and whether they are enabled — from registration onward. A
customer is a record, not an account: there are no credentials, sessions, or
login of any kind.

## Published events

All events publish onto the `customers` JetStream stream (subject
`customers.customer.*`), at-least-once delivery. Publish happens after the
aggregate's transaction commits, with inline retry and backoff (five attempts,
~10.5s worst-case budget, same schedule as every other service per ADL 0007),
and attaches a per-call `Nats-Msg-Id` reused across every retry of one logical
publish, relying on the stream's `Duplicate Window` (2 minutes, JetStream's
default, unconfigured) to collapse a late-arriving retry into the original
stored message (ADL 0015). A publish that exhausts all five retries still
leaves the customer saved in Postgres with no corresponding event ever
published — the same known, currently unresolved gap ADL 0007 names for every
other service, demonstrated here against a real outage. The stream has no
dead-letter subject, since `customers` consumes nothing.

Every event's envelope carries `aggregate_id`, `event_name`, `occurred_at`, and
`data`.

### `CustomerRegistered`

- **Subject:** `customers.customer.CustomerRegistered`
- **Published when:** a customer is registered via `RegisterCustomer`
- **Payload:**

```json
{ "name": "string", "sms_number": "string" }
```

- Both values are trimmed of surrounding whitespace before being stored and
  published. A new customer is always enabled; there is no separate "enabled"
  event at registration.

### `CustomerSmsChanged`

- **Subject:** `customers.customer.CustomerSmsChanged`
- **Published when:** `ChangeSmsNumber` succeeds
- **Payload:**

```json
{ "sms_number": "string (the new number, absolute — not a delta)" }
```

### `CustomerEnabled`

- **Subject:** `customers.customer.CustomerEnabled`
- **Published when:** `EnableCustomer` succeeds
- **Payload:** `{}`

### `CustomerDisabled`

- **Subject:** `customers.customer.CustomerDisabled`
- **Published when:** `DisableCustomer` succeeds
- **Payload:** `{}`

### Ordering of events for one customer

State changes are serialized by a row lock, but each event is published after
its transaction commits and outside the lock. Two changes to the same customer
in quick succession can therefore reach the stream in the opposite order from
the one in which they committed. A consumer that applies "latest event wins"
must compare `occurred_at` per `aggregate_id` and discard an older event
arriving after a newer one. The payloads carry the full new value, never a
delta, so this is safe to do.

## Synchronous API surface

ConnectRPC, served at `/customers.v1.CustomersService/<Method>`, supporting
both gRPC and Connect's HTTP+JSON protocol (the latter requires a
`Connect-Protocol-Version: 1` header). All six methods below are live and
verified against real Postgres/NATS, and via a curl script against the
containerized service — see `customers/customerspb/customers.proto` for exact
message shapes. Customer IDs are server-generated (UUID) on registration,
never client-supplied (ADL 0009).

| Method              | Request → Response                         | Notes                                                                                                                      |
| ------------------- | ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `RegisterCustomer`  | `{name, sms_number}` → `{customer}`        | customer id is server-generated; name and number are trimmed and must be non-blank                                         |
| `ChangeSmsNumber`   | `{customer_id, sms_number}` → `{customer}` | errors if blank or identical to the current number                                                                         |
| `EnableCustomer`    | `{customer_id}` → `{customer}`             | errors if already enabled                                                                                                  |
| `DisableCustomer`   | `{customer_id}` → `{customer}`             | errors if already disabled                                                                                                 |
| `AuthorizeCustomer` | `{customer_id}` → `{}`                     | read-only: no write, no lock, no event. Succeeds for an enabled customer; fails with `PermissionDenied` for a disabled one |
| `GetCustomer`       | `{customer_id}` → `{customer}`             | read-only, no write, no event                                                                                              |

### Status codes

| Condition                                          | Connect code         | HTTP |
| -------------------------------------------------- | -------------------- | ---- |
| Unknown customer ID, including a malformed one     | `NotFound`           | 404  |
| Blank name or SMS number; SMS number unchanged     | `InvalidArgument`    | 400  |
| Enable an enabled customer; disable a disabled one | `FailedPrecondition` | 400  |
| `AuthorizeCustomer` on a disabled customer         | `PermissionDenied`   | 403  |
| Anything else (database or messaging failure)      | `Internal`           | 500  |

A caller deciding whether to retry should treat `Unavailable`,
`DeadlineExceeded`, and `Internal` as transient and every other code as
permanent. In particular `PermissionDenied` and `NotFound` will never succeed
on retry.

## Data this service owns

`customers` is the source of truth for customer identity: id, name, SMS number,
and enabled state (the `customers` table). Consumers may cache a read-only
projection of this data but must never write it back.

## Explicit non-goals and known limitations of this contract

- **No authentication, credentials, or sessions.** "Identity" here means a
  record. Login belongs to the storefront project.
- **`customer_id` is not yet threaded through `baskets` or `ordering`.** No
  other service references a customer today, and `ordering` does not yet call
  `AuthorizeCustomer`. This is planned separately (see ADL 0020).
- **SMS numbers are not format-validated.** The only rules are non-blank and
  trimmed. A value like `"abc"` is accepted. The database's blank check
  (`btrim`) strips only ASCII spaces; the domain strips all Unicode whitespace
  and is the real validator.
- **No uniqueness on name or SMS number.** Two customers may share both.
- **A `RegisterCustomer` call that fails with a publish error (`Internal`)
  leaves the customer saved.** A client retrying generates a new ID and creates
  a second customer. The same exposure exists for `stores`.
- **`AuthorizeCustomer` reads without a lock.** It can race with a concurrent
  `DisableCustomer`; the answer reflects the state at the moment of the read.
- **proto3 JSON omits a `false` boolean.** A response for a disabled customer
  has no `enabled` field at all; a client must read a missing `enabled` as
  `false`.
- **Error messages expose the wrapped use-case chain**, as in every other
  service.
- **No customer deletion, no listing, no pagination, no bulk operations, and no
  historical event replay beyond JetStream's own retention window.**
- **Test coverage of the RPC layer is shallow.** The error mapping has unit
  tests and every handler was verified live, end to end, with a curl script
  against the containerized service, but there are no per-handler automated
  tests. Domain, repository, event, and use-case layers have full automated
  coverage, including concurrency, rollback, and outage tests.
- **Read RPCs share the same request timeout ceiling as write RPCs**, the same
  known, unresolved gap ADL 0007's addendum names for every other service.
