# 19. Build `customers` before `payments`

Date: 2026-10-10

## Status

Accepted

## Context

ADL 0001 placed `payments` at position 4 and `customers` at position 5, moving
`customers` ahead of the storefront only. Before starting `payments`, the
system was compared against the source book's `Chapter12/`. In the book,
`payments` depends on customer identity throughout: `AuthorizePayment` takes a
`customer_id`; the create-order saga's first step authorizes the customer; an
`Order` carries a `CustomerID` and `PaymentID`; `OrderReadied` carries
`customer_id`, `payment_id`, and `total`. `customers` is a zero-dependency
root, so building it earlier violates nothing in ADL 0001's dependency graph.

## Decision

Build `customers` before `payments`. A `contract-evolution` project (ADL 0020)
follows `customers` and precedes `payments`.

## Alternatives considered

**`payments` first, with a placeholder customer ID.** Rejected: it breaks the
no-stubbing commitment and builds exactly the fake identity scheme that ADL 0001
moved `customers` earlier to avoid.

## Consequences

- ADL 0001's position numbers no longer match the real build order. It is not
  rewritten (ADL 0014's precedent for historical records); `docs/roadmap.md`
  carries the amendment.
- `customers` was built with no consumer: nothing calls it until ADL 0020's
  second tranche. Its contract was designed against the book's consumers
  (`notifications`, `search`) and against `ordering`'s planned synchronous call.
