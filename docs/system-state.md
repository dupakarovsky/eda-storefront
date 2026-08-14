# System State

The single source of truth for "what's actually running right now." Updated
at the start and end of every project — see `docs/roadmap.md` for the
working agreement around this.

Last updated: 2026-08-14 (`foundation` project complete)

| Service                         | Status      | Project Dir                 | Publishes                                                                              | Consumes                                                                                   | Last Updated |
| ------------------------------- | ----------- | --------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------ |
| `foundation` (shared package)   | Live        | `foundation/`               | —                                                                                      | —                                                                                          | 2026-08-14   |
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

| Component            | Status | Notes                                                                                                                                                                                                                                                                                          |
| -------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docker-compose.yml` | Live   | Postgres 17 + NATS 2.12 (JetStream), each with a named volume (`pg-data`, `nats-data`), on a pinned-subnet `eda-net` bridge network, ports bound to `127.0.0.1` only. Grows by one service block per project from here.                                                                        |
| NATS (JetStream)     | Live   | `EnsureStream` uses explicit `FileStorage`; durability across container restart confirmed (`nats_seq` continuity + matching `/jsz` stream state before/after a `down`/`up` cycle, including from a fresh volume).                                                                              |
| Postgres             | Live   | Hardened baseline per ADL 0006 — custom `pg_hba.rules` (`peer` for local socket scoped to `edasuper`, `scram-sha-256` for TCP, explicit reject-all fallback), confirmed on both an existing and a freshly-initialized volume. One database (`edadb`) so far; one per service as each is added. |
| `Makefile`           | Live   | `up`, `down` (deliberately non-destructive of volumes), `logs`, `ps`, `smoke`. Grows by one set of targets per project.                                                                                                                                                                        |

## How to update this file

At the **start** of a project: confirm every service listed as a live
dependency in `docs/roadmap.md`'s project table is actually marked "Live"
here before beginning. If it isn't, stop and reconcile before writing any
code.

At the **end** of a project: flip the new service(s) from "Not started" (or
"In progress") to "Live," fill in real published/consumed event names (the
TBD placeholders above are guesses from the source book, not commitments),
and update "Last Updated" for every row that changed.
