# System State

The single source of truth for "what's actually running right now." Updated at
the start and end of every project. See `docs/roadmap.md` for the working
agreement around this.

Events published and consumed, and API surfaces, are documented in each
service's `CONTRACT.md`, not here.

Last updated: 2026-10-12 (restructured; `customers` complete: live and
containerized as of 2026-10-10)

## Services

| Service                         | Status      | Project Dir              | Contract                            | Last Updated |
| ------------------------------- | ----------- | ------------------------ | ----------------------------------- | ------------ |
| `foundation` (shared package)   | Live        | `foundation/`            | n/a                                 | 2026-08-30   |
| `stores`                        | Live        | `stores/`                | `stores/CONTRACT.md`                | 2026-08-30   |
| `depot`                         | Live        | `depot-baskets/depot/`   | `depot-baskets/depot/CONTRACT.md`   | 2026-09-16   |
| `baskets`                       | Live        | `depot-baskets/baskets/` | `depot-baskets/baskets/CONTRACT.md` | 2026-09-12   |
| `ordering`                      | Live        | `ordering/`              | `ordering/CONTRACT.md`              | 2026-09-30   |
| `customers`                     | Live        | `customers/`             | `customers/CONTRACT.md`             | 2026-10-10   |
| `payments`                      | Not started | `payments/`              | TBD                                 | n/a          |
| `storefront` (Templ + Datastar) | Not started | `storefront/`            | n/a (frontend)                      | n/a          |
| `search`                        | Not started | `search/`                | TBD (read-only)                     | n/a          |
| `notifications`                 | Not started | `notifications/`         | TBD                                 | n/a          |

## Infrastructure

### docker-compose.yaml

- Postgres 17 and NATS 2.12 (JetStream), plus five app services: `stores`
  (host port 8080), `baskets` (8081), `depot` (8082), `ordering` (8083) and
  `customers` (8084). Every host port is bound to `127.0.0.1` only.
- All services sit on the `eda-net` bridge network with a pinned subnet
  (`172.18.0.0/16`), which the Postgres HBA rules reference (ADL 0006).
- `customers` is a root service and depends only on `postgres` and `nats`.
  `ordering` depends on `postgres`, `nats`, `stores`, `baskets` and `depot`.
- `ordering` and `customers` build with `golang:1.26`, required by `go.work`'s
  own version; earlier services were recorded as `golang:1.25`.
- Verified live: all five app services together via `make build`, `down`,
  `up`, plus a 15s NATS outage with all five staying healthy.

### NATS (JetStream)

- Streams use explicit `FileStorage`; durability across container restart is
  confirmed.
- No Docker healthcheck: `nats:2.12` ships no shell or utilities (ADL 0011).
  Startup safety comes from `RetryOnFailedConnect(true)` in `foundation/nats`.
- Client reconnect is unlimited by default (`NATS_MAX_RECONNECTS` = -1 in
  `foundation/config`; ADL 0011 addendum). Previously an outage over ~10s
  permanently closed every service's connection while the process kept running.
- Every publisher attaches a per-call `Nats-Msg-Id`, reused across all retries
  of one logical publish (ADL 0015), relying on each stream's 2-minute
  Duplicate Window.
- Every service's dead-letter subject uses the `>` wildcard, not `*`.

### Postgres

- `edadb` is the administration database. The container was created with it
  in place of the default `postgres` database, and the superuser `edasuper`
  connects to it to create the per-service databases. No service uses it.
- One database per service: `storesdb`, `basketsdb`, `depotdb`, `orderingdb`,
  `customersdb`.
- Hardened baseline per ADL 0006.
- A real `pg_isready` healthcheck gates `depends_on` for every dependent
  service.
- `docker-entrypoint-initdb.d/` scripts (`03` to `07`) only run on a fresh
  volume. On the long-lived `pg-data` volume, each new service's database,
  role and privilege setup was applied manually once.

### Makefile

<!-- docs/system-state.md: Makefile section, replace the development targets bullet -->

- Development targets: `<service>/run` (a bare `go run` for fast local
  iteration, not containerized), `<service>db/dev`, `nats/baskets` and
  `format`.
- Development targets: `<service>/run` (a bare `go run` for fast local
  iteration, not containerized), `<service>db/dev`, `foundation/smoke`,
  `nats/baskets` and `format`.
- Per-service `logs/<service>` and `kill/<service>` exist for every app
  service. Note: `kill/<service>` sends SIGTERM through Docker, which Docker
  treats as a manual stop, so `restart: unless-stopped` does not apply: the
  container stays down, including after a daemon restart, until started
  manually. Crash recovery (the process dying without Docker's involvement)
  has not been tested.

## How to update this file

At the **start** of a project: confirm every service listed as a live
dependency in `docs/roadmap.md`'s project table is marked "Live" here before
beginning. If it isn't, stop and reconcile before writing any code.

At the **end** of a project: flip the new service(s) to "Live," update the
"Last Updated" cell for every row that changed, and set the "Last updated"
line at the top.

Infrastructure notes are short bullets under their component. Append a bullet;
never grow a table cell.
