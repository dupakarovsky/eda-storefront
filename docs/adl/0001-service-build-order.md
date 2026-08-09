# 1. Service build order and dependency sequencing

Date: 2026-08-09

## Status

Accepted

## Context

The source system (_Event-Driven Architecture in Golang_) has eight
services with a real dependency graph, traced directly from its event
handlers rather than assumed:

- `stores` and `customers` are roots — they publish events but consume
  nothing from other services.
- `depot` and `baskets` both consume `stores`' events (`StoreCreated`,
  `ProductAdded`, etc.) to maintain a local product/store cache.
- `ordering` consumes `baskets` (`BasketCheckedOut`) _and_ `depot`
  (`ShoppingListCompleted`) — the one service with two upstream
  dependencies.
- `payments` consumes `ordering` (`OrderReadied`, `OrderCanceled`).
- `notifications` and `search` both consume `ordering` and `customers`
  (`search` also consumes `stores`) — terminal, read-side services.

Separately, we've committed to building this incrementally as a sequence of
projects, at most two services per project, and — critically — **without
stubbing any service dependency**. Every project that depends on another
service must be built and tested against a genuinely running instance of it,
not a mock or fake standing in for it.

We also want a "polished" web storefront (Templ + Datastar), which needs
real product data, a real cart, a real checkout flow, and real customer
identity to be a genuine artifact rather than a demo built on placeholder
data.

## Decision

Build in this order:

1. `stores` (root)
2. `depot` + `baskets` (both depend only on `stores`, and not on each other
   — pairable within the two-service cap)
3. `ordering` (depends on both `baskets` and `depot`; must be solo despite
   the cap, since it has two live upstream dependencies to stand up)
4. `payments` (depends on `ordering`)
5. `customers` (root — but deliberately built _here_, not earlier)
6. Storefront (Templ + Datastar)
7. `search`
8. `notifications`

The one deliberate deviation from the book's own dependency graph is
**moving `customers` to position 5, ahead of `search`/`notifications`**,
even though nothing in the backend dependency graph strictly needs it until
`search`/`notifications` (position 7–8). The reason: the storefront (position 6) needs real customer identity — cart ownership, checkout, order history —
to be genuine rather than built on a placeholder session/fake-ID scheme that
would later need to be ripped out once `customers` existed for real. Since
`customers` is a zero-dependency root, building it early costs nothing and
avoids that rework.

## Alternatives considered

**Story-first ordering (`ordering` + `depot` as Project 1, stubbing `stores`
and `baskets`).** Rejected specifically because of the no-stubbing
commitment — this would have required faking two upstream event sources as
temporary scaffolding, which is exactly the pattern we've decided against
for this multi-project system, even though it's a pattern used successfully
within individual projects before (e.g. `court-reservation`'s `stubStore`).

**Leaving `customers` in its book-native position (built alongside or after
`search`/`notifications`).** Rejected because it would force the storefront
project to either wait until position 7–8 to be built at all, or be built
earlier against placeholder identity and reworked later. Moving one root
service earlier, with zero dependency cost, was judged strictly better than
either option.

**Building `search` and `notifications` together as one project**, as
originally discussed when they were still both terminal, low-priority
capstone work. Superseded once `notifications` was upgraded from optional
capstone to "build it for real, with an actual email/SMS integration" —
that's genuine standalone scope (a `Notifier` port, a dev adapter, a real
provider adapter, real delivery verification) comparable to `depot`+`baskets`
in weight, not a light pairing with `search`.

## Consequences

- Every project from Project 2 onward requires standing up an increasing
  number of real live services (plus NATS and Postgres) just to develop
  against — by Project 3 (`ordering`), that's four services running
  simultaneously. `docker-compose.yml` needs to be treated as a first-class,
  continuously-extended deliverable from Project 1 onward, not assembled
  after the fact.
- The storefront (Project 6) is the first point in the roadmap where every
  service it needs (`stores`, `baskets`, `ordering`, `payments`,
  `customers`) is genuinely live — it is the first real end-to-end
  milestone of the whole system, not an earlier one.
- `search` and `notifications` remain the two most deferrable pieces of the
  original book's scope — the system is fully usable (browse, cart, order,
  pay, have an account) without either of them.
