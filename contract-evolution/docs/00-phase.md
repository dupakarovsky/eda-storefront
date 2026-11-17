## Phase 0, Step 1: restructure `system-state.md`

The wide-table problem comes from the Publishes/Consumes columns and the
paragraph-sized infrastructure cells. This version keeps one narrow services
table, and each `CONTRACT.md` becomes the single source of truth for events and
APIs. It also merges in your 2026-10-10 changes. Replace the whole file.

```markdown
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

- Hardened baseline per ADL 0006.
- One database per service: `edadb` (foundation smoke-testing), `storesdb`,
  `basketsdb`, `depotdb`, `orderingdb`, `customersdb`.
- A real `pg_isready` healthcheck gates `depends_on` for every dependent
  service.
- `docker-entrypoint-initdb.d/` scripts (`03` to `07`) only run on a fresh
  volume. On the long-lived `pg-data` volume, each new service's database,
  role and privilege setup was applied manually once.

### Makefile

- Deployment targets: `up`, `down`, `build`, `logs`, `ps`, `exec/pg`, and
  `migrate/<service>/up|down` for stores, baskets, depot, ordering and
  customers.
- Development targets: `<service>/run` (a bare `go run` for fast local
  iteration, not containerized), `<service>db/dev`, `foundation/smoke`,
  `nats/baskets` and `format`.
- Per-service `logs/<service>` and `kill/<service>` exist for every app
  service. Note: `docker compose kill` (so `kill/<service>`) does not trigger
  `restart: unless-stopped`; the container stays down until started manually.

## How to update this file

At the **start** of a project: confirm every service listed as a live
dependency in `docs/roadmap.md`'s project table is marked "Live" here before
beginning. If it isn't, stop and reconcile before writing any code.

At the **end** of a project: flip the new service(s) to "Live," update the
"Last Updated" cell for every row that changed, and set the "Last updated"
line at the top.

Infrastructure notes are short bullets under their component. Append a bullet;
never grow a table cell.
```

**What's happening here.** The main design choice is that this file no longer
duplicates anything a `CONTRACT.md` already owns. The old Publishes and Consumes
columns were the reason the table couldn't be edited by hand, and they were also
a second place for event names to drift out of date. ADL 0004 already says
`CONTRACT.md` must stay honest and current, so pointing at the contracts removes
a duplication that could only go stale. The `customers` row used to claim only
two events, `CustomerRegistered` and `CustomerSmsChanged`, while the contract
lists four. That is exactly the kind of drift this removes.

The infrastructure notes moved from table cells to plain bullets per component,
so future updates (like the one for `customers`) are an appended line instead of
an edit inside a long cell. The `customers` additions from your list are folded
into the compose, NATS, Postgres and Makefile sections. I merged them rather
than appending, because the old text and the new facts needed to agree. For
example, the old NATS note didn't mention unlimited reconnect at all.

Two things to double-check against your real files, since I derived them from
the earlier text and the compose file. I only know the `golang:1.25` for
`stores`, `baskets` and `depot` from the old note, so verify their Dockerfiles.
I also assumed each service's `CONTRACT.md` sits at its project folder root, as
the `Project folder:` header in each contract implies.

**What to do.** Replace `docs/system-state.md` with this content and open it in
a Markdown preview. If you use `mdformat --wrap=80`, run it on that file as
well. There's nothing to compile, so the check is visual. Confirm that the table
renders, and that nothing from the old file that you still care about has been
lost. Paste back anything you think I dropped.

Any questions about the restructure, or anything you'd rather keep in the table?
If it looks right to you, we'll move to Step 2, the `roadmap.md` amendment for
the real build order (ADL 0019).

---

That output explains one more thing. `eda-postgres` shows "Created 2 minutes
ago", while everything else is 31+ minutes old. When you ran
`docker compose up -d customers`, Compose saw that the `07-customers-db.sh`
mount path in `docker-compose.yaml` had changed (your typo fix) and recreated
the Postgres container. This is exactly the behavior ADL 0006 describes, where
mount changes need container recreation, not restart. Your data is safe because
it lives in the `pg-data` volume, and the init scripts don't re-run against a
populated volume, so the corrected mount only matters if you ever rebuild from
scratch.

It also means that four services you didn't touch just survived their database
disappearing and coming back. That's worth confirming before we trust the stack.

## Check the stack is healthy

```bash
for p in 8080 8081 8082 8083 8084; do printf "%s " $p; curl -s -o /dev/null -w "%{http_code}\n" localhost:$p/health; done
```

You should see `200` for all five ports (stores, baskets, depot, ordering,
customers). If any returns `500`, paste it with
`docker compose logs --since 10m <service>`.

## About the restart behavior

Your result matches Docker's documented behavior: a restart policy only applies
when a container exits on its own. If you stop or kill a container through
Docker, it counts as manually stopped, so `unless-stopped` ignores it until you
start it again. That includes after a daemon restart, which is why `customers`
stayed down when the machine came back. `make kill/<service>` sends SIGTERM
through Docker, so it can never test crash recovery. A real crash (the process
dying without Docker's involvement) takes a different path, and we haven't
tested it. I'll note that for later and not chase it now.

Replace the last bullet of the Makefile section in `docs/system-state.md` with
this more accurate wording:

```markdown
<!-- docs/system-state.md: Makefile section, replace the last bullet -->

- Per-service `logs/<service>` and `kill/<service>` exist for every app
  service. Note: `kill/<service>` sends SIGTERM through Docker, which Docker
  treats as a manual stop, so `restart: unless-stopped` does not apply: the
  container stays down, including after a daemon restart, until started
  manually. Crash recovery (the process dying without Docker's involvement)
  has not been tested.
```

## Phase 0, Step 4: regression run

This is the regression we haven't seen since the NATS reconnect default changed
(ADL 0011 addendum). It exercises the whole chain: `stores` → `baskets` and
`depot` projections → `baskets` checkout → `ordering` consumes and orchestrates
`depot` → completion → `readied`, plus the cancel path. With Postgres having
just been recreated, it's also a good test of the connection pools.

Run it from the repo root and keep a copy of the output:

```bash
bash ordering/ordering-http-connect-2.sh 2>&1 | tee /tmp/ordering-regression.txt
```

It needs `jq` and takes about 15 seconds because of the `sleep` calls. This is
what a good run looks like:

- The `GetOrder` for order #1, before completing, shows status `created` with a
  non-empty `shoppingListId`. If that field is empty, `ordering` never consumed
  `BasketCheckedOut` or couldn't reach `depot`.
- After `CompleteShoppingList`, `GetOrder` shows `readied`.
- `CancelOrder` on the readied order prints `HTTP 400`.
- Order #2 shows `created`, then `CancelOrder` returns `canceled`, and `depot`'s
  `GetShoppingList` for its list shows `canceled`.
- `CancelOrder` on the already-canceled order prints `HTTP 400`.
- Both unknown-ID calls print `HTTP 404`.

After it finishes, also run this to catch silent failures the script wouldn't
show, such as dead-lettered messages:

```bash
docker compose logs --since 5m ordering depot baskets | grep -i -E "error|dead-lett|exhausted"
```

Ideally that prints nothing. Paste the script output and the grep result, and
tell me if anything differs from the list above. If it's all clean, Phase 0 is
done and I'll write the Phase 0 summary for `journey_progress.md` before we
start Phase 1.

---
