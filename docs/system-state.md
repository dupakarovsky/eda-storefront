# System State

The single source of truth for "what's actually running right now." Updated
at the start and end of every project — see `docs/roadmap.md` for the
working agreement around this.

Last updated: 2026-08-09 (initial creation, nothing built yet)

| Service                         | Status      | Project Dir                 | Publishes                                                                              | Consumes                                                                                   | Last Updated |
| ------------------------------- | ----------- | --------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------ |
| `foundation` (shared package)   | Not started | `foundation/`               | —                                                                                      | —                                                                                          | —            |
| `stores`                        | Not started | `01-stores/`                | `StoreCreated`, `StoreRebranded`, `ProductAdded`, `ProductRebranded`, `ProductRemoved` | —                                                                                          | —            |
| `depot`                         | Not started | `02-depot-baskets/depot/`   | `ShoppingListCompleted` (+ others TBD)                                                 | `stores` events                                                                            | —            |
| `baskets`                       | Not started | `02-depot-baskets/baskets/` | `BasketCheckedOut` (+ others TBD)                                                      | `stores` events                                                                            | —            |
| `ordering`                      | Not started | `03-ordering/`              | `OrderCreated`, `OrderReadied`, `OrderCanceled` (+ others TBD)                         | `baskets`, `depot` events                                                                  | —            |
| `payments`                      | Not started | `04-payments/`              | TBD                                                                                    | `ordering` events                                                                          | —            |
| `customers`                     | Not started | `05-customers/`             | `CustomerRegistered`, `CustomerSmsChanged`                                             | —                                                                                          | —            |
| `storefront` (Templ + Datastar) | Not started | `06-storefront/`            | — (frontend)                                                                           | `stores`, `baskets`, `ordering`, `payments`, `customers` (via API + NATS for live updates) | —            |
| `search`                        | Not started | `07-search/`                | — (read-only)                                                                          | `stores`, `ordering`, `customers` events                                                   | —            |
| `notifications`                 | Not started | `08-notifications/`         | —                                                                                      | `customers`, `ordering` events                                                             | —            |

## Infrastructure

| Component            | Status      | Notes                                                                     |
| -------------------- | ----------- | ------------------------------------------------------------------------- |
| `docker-compose.yml` | Not started | Created in Project 1; grows by one service block per project thereafter   |
| NATS (JetStream)     | Not started | Stood up in Project 1                                                     |
| Postgres             | Not started | One database per service, stood up incrementally as each service needs it |
| `Makefile`           | Not started | Created in Project 1; grows by one set of targets per project thereafter  |

## How to update this file

At the **start** of a project: confirm every service listed as a live
dependency in `docs/roadmap.md`'s project table is actually marked "Live"
here before beginning. If it isn't, stop and reconcile before writing any
code.

At the **end** of a project: flip the new service(s) from "Not started" (or
"In progress") to "Live," fill in real published/consumed event names (the
TBD placeholders above are guesses from the source book, not commitments),
and update "Last Updated" for every row that changed.
