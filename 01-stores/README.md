# EVENT DRIVE ARCHITECTURE STOREFRONT IN GO: STORES

## Phase Plan

**Phase 1 — Domain Model** Build `Store` and `Product` as current-state
aggregates: fields, constructors, behavior methods (`Rebrand`,
`EnableParticipation`/ `DisableParticipation`, `IncreasePrice`/`DecreasePrice`,
`Remove`, etc.), validation, and the event payload types they raise. No
persistence, no API yet — testable in isolation with a throwaway script.

**Phase 2 — Postgres Persistence** Schema + migration for `stores` and
`products` tables, repository interfaces, and Postgres adapters with
`Reconstitute` rehydration and current-state upserts. This is where row-lock vs
optimistic-version concurrency gets decided per aggregate.

**Phase 3 — Event Publishing** Wire aggregate actions to real NATS JetStream
publishes using `foundation/nats`, including defining the `stores` stream and
subject scheme. By the end, a throwaway script can create a store and we can
watch the event land on NATS for real.

**Phase 4 — ConnectRPC API** Define `storespb/api.proto`, run `buf` codegen,
implement the Connect handlers, and mount them on foundation's `Router`. Covers
both commands (create/rebrand/add product/etc.) and queries (get store, get
catalog, etc.).

**Phase 5 — Wiring, Compose, and Verification** `module.go` composition root,
`main.go`, extending `docker-compose.yml`/ `Makefile` with the `stores` service
and its own Postgres database, then real `curl`/integration verification against
live containers.

**Phase 6 — Contract and Close** Finalize `CONTRACT.md` with real event/API
shapes (superseding the `system-state.md` placeholders), update
`system-state.md`, and write the closing Phase summary.

## **Phase 1 — Domain Model**

### Step 1: Store and Product

**Setup:** create `01-stores/` at the workspace root, and inside it run
`go mod init eda-storefront/stores` (adjust the module path to match whatever
prefix `foundation`'s `go.mod` uses, so imports stay consistent). Then add it to
the root `go.work`:

```
// go.work
go 1.25.0

use (
    ./foundation
    ./foundation-smoketest/
    ./01-stores
)
```

Now the aggregate. Two files: `internal/domain/events.go` for the shared event
wrapper type, and `internal/domain/store.go` for `Store` itself.

```go
// internal/domain/events.go
```

```go
// internal/domain/store.go
```

What's happening here: `DomainEvent` is a deliberately minimal replacement for
the book's `ddd.Event`/`registry.Registerable` machinery — ADL 0002 already
ruled out porting `internal/es`, and since we're not event-sourcing, we don't
need an event type that knows how to be replayed, only one that knows its name
and payload well enough to hand to a publisher later (Phase 3).
`Name`/`Payload any` is enough for that.

Notice `NewStore` takes `id` as a parameter rather than generating it
internally. That's on purpose — ID generation (UUID or otherwise) is an
application-layer decision, not a domain one, so the constructor stays pure and
trivially testable: same inputs, same outputs, no hidden side effect.
`Participating` is left at its zero value (`false`) on creation, matching the
book — a new store doesn't participate until someone explicitly enables it,
which we'll build in the next step.

One more thing worth flagging: the book wraps errors with
`github.com/stackus/errors` and an `ErrBadRequest` sentinel, presumably so the
gRPC layer can map domain errors to status codes generically. We're using plain
`errors.New` for now — simpler, and we don't yet know how ConnectRPC's
error-code mapping wants to work (Phase 4). If we need richer error
classification once we get there, we can revisit this without it being a big
change.

Go ahead and create those two files, then run `go build ./...` from inside
`01-stores/` to confirm it compiles cleanly — there's no `main` yet, so this is
just a compile check. Let me know how that goes, and if you have questions about
the `DomainEvent` design before we move to Step 2 (`Rebrand`,
`EnableParticipation`, `DisableParticipation`).

### Step 2: StoreParticipation Enabled, Disable and Rebrand Events

Agreed — that's consistent with the `StoreCreated`/`StoreRebranded` shape, and
avoids the mixed-visibility problem entirely by trading one type with a variable
name for two types with fixed names. Here's the full `events.go` addition and
the corrected `store.go` methods:

```go
// internal/domain/events.go
```

`Rebrand`'s blank-name check is back in, using the same `ErrStoreNameIsBlank`
sentinel `NewStore` already uses — one error, reused wherever "a store needs a
non-blank name" is being enforced, rather than a second near-identical sentinel.

Build it and let me know how it goes. Once that's clean, we'll move to `Product`
— same pattern (constructor + behaviors + events), but with a couple of new
things: it references a `StoreID` it doesn't own, and price changes
(`IncreasePrice`/`DecreasePrice`) need to decide what happens on invalid input
(negative resulting price) that `Store`'s methods so far haven't had to
consider.

### Step 3: Product Aggragate

`Product` follows the same shape as `Store`, with a bit more happening in the
price methods. Here's the full aggregate — constructor, `Rebrand`,
`IncreasePrice`/`DecreasePrice`, and `Remove` — plus its events.

```go
// internal/domain/events.go — additions
```

Three deviations from the book worth flagging explicitly:

1. **`Rebrand` validates blank name**, same reasoning as `Store.Rebrand` — the
   book didn't check it there either, and we already decided that gap was worth
   closing.
1. **`IncreasePrice`/`DecreasePrice` reject negative prices**, which the book's
   version doesn't do (only `InitProduct` checks `price < 0`). Since these take
   an absolute target price, not a delta, nothing stops a negative one from
   slipping through post-creation without this.
1. **Price is stored and set directly** (`p.Price = price`) rather than via
   delta arithmetic. The book computes `Delta` and later reapplies it with
   `p.Price = p.Price + payload.Delta` inside `ApplyEvent` — a pattern that
   exists because their `ApplyEvent` is the _only_ path that mutates state
   (event-sourced rehydration). We don't have that separate path — the method
   itself is the one and only place `Price` changes — so recomputing the same
   value via addition would just be redundant arithmetic for the same result.
   The event's `Delta` field is kept as-is, since a consumer (`depot`'s cache,
   say) may still want "price went up by $2" rather than just the new absolute
   price.

`Remove()` always succeeds — no invariant currently blocks removing a product —
so it returns `(Event, error)` only for signature consistency with the other
methods, not because an error is actually reachable. If a real precondition
shows up later (e.g. "can't remove a product with pending orders"), the
signature's already there to support it.

Build this and let me know how it goes — then we'll write `product_test.go` the
same way we did for `Store`.

#### Test Files

`product_test.go`, following the same shape as `store_test.go` — a shared
constructor helper, then one `t.Run` subtest per behavior and its error paths,
including the equal-price case for both `IncreasePrice` and `DecreasePrice`.

```go
// internal/domain/product_test.go
```

One thing to note on the negative-price checks for
`IncreasePrice`/`DecreasePrice`: since both already reject any target price on
the wrong side of the current price via `<=`/`>=`, and `NewProduct` already
guarantees `Price >= 0` at creation, a negative target price for `IncreasePrice`
would already be caught by `price <= p.Price` in virtually every real case (you
can't increase into negative territory from a non-negative starting price) — the
explicit `price < 0` check is mostly a defense-in-depth/clarity thing rather
than something reachable through normal use. Worth keeping for a clear error
message either way, but wanted to flag that the two checks overlap in coverage
rather than being fully independent guards.

Run `go test ./01-stores/internal/domain/... -v` and let me know how it goes.

## **Phase 2 — Postgres Persistence**

Here's how I'd like to break Phase 2 into Steps, following the placeholder
pattern for this domain (schema + seed → interface → real adapter → wiring):

**Step 1 - Create new database roles**

**Step 2 — Migrations: schema for `stores` and `products`** Write the
`golang-migrate` up/down SQL files defining both tables. This also means adding
a `stores` service database to `docker-compose.yaml` and a `migrate` target to
the `Makefile`, since nothing runs against Postgres yet without them.

**Step 3 — Repository port interfaces** Define `StoreRepository` and
`ProductRepository` as interfaces in `internal/domain/`, with `Save` and a
`FindByID`-style read that returns a rehydrated aggregate. This is the seam
Phase 4's application layer will depend on, decoupled from Postgres specifics.

**Step 4 — Postgres adapter: `Store`** Implement the `Store` repository against
real Postgres — `Reconstitute` rehydration and a current-state upsert. This is
also where we decide row-lock vs optimistic-version-column concurrency for
`Store` specifically, based on its actual write pattern (infrequent
rebrand/toggle, low contention).

**Step 5 — Postgres adapter: `Product`** Same shape for `Product`, but its write
pattern is different (price changes could plausibly race), so we'll look at that
concurrency decision separately rather than copying Step 3's choice by default.

**Step 6 — Wiring and verification** A throwaway script that creates a store,
adds a product, saves both, reloads them via `Reconstitute`, and confirms the
round-trip against a real running container — closing out Phase 2 in a genuinely
tested state.

Does this breakdown look right, or would you like to split/merge any of these
before we start on Step 1?

### Step 1: Create Roles

Good — since the pg_hba rules already grant `scram-sha-256` to `all` users over
TCP (not just `edasuper`), we don't need to touch `pg_hba.rules` at all. The
only role-specific line is the `peer` one, and it stays scoped to `edasuper`
since our new roles will connect over TCP like any application does. And since
we're fine wiping the current volume, I'll fold role creation straight into
`docker-entrypoint-initdb.d` alongside `init.sh`, rather than writing a separate
manual-application path.

Three small changes for Step 1:

```env
# .postgres.env
POSTGRES_USER=edasuper
POSTGRES_PASSWORD=supersecure
POSTGRES_DB=edadb
POSTGRES_HOST_AUTH_METHOD=scram-sha-256
EDA_DEV_PASSWORD=devsecure
EDA_USER_PASSWORD=usersecure
```

```sh
# ./postgres/init/02-roles.sh
#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE ROLE eda_dev LOGIN PASSWORD '$EDA_DEV_PASSWORD';
    CREATE ROLE eda_user LOGIN PASSWORD '$EDA_USER_PASSWORD';

    GRANT CONNECT ON DATABASE $POSTGRES_DB TO eda_dev, eda_user;

    GRANT USAGE, CREATE ON SCHEMA public TO eda_dev;
    GRANT USAGE ON SCHEMA public TO eda_user;

    ALTER DEFAULT PRIVILEGES FOR ROLE eda_dev IN SCHEMA public
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO eda_user;

    ALTER DEFAULT PRIVILEGES FOR ROLE eda_dev IN SCHEMA public
        GRANT USAGE, SELECT ON SEQUENCES TO eda_user;
EOSQL
```

```yaml
# ./docker-compose.yaml — postgres.volumes
volumes:
    - pg-data:/var/lib/postgresql/data
    - ./postgres/init/init.sh:/docker-entrypoint-initdb.d/init.sh
    - ./postgres/init/02-roles.sh:/docker-entrypoint-initdb.d/02-roles.sh
    - ./postgres/config:/etc/postgresql/conf.d
```

What's happening here: `docker-entrypoint-initdb.d` runs every script it finds
in filename order, and by the time it gets to ours, the official image has
already run `initdb` and created `edasuper` as superuser plus the `edadb`
database from `.postgres.env` — so `psql --username "$POSTGRES_USER"` in our
script is connecting as `edasuper` and has full authority to create the two new
roles. `eda_dev` gets `CREATE` on the `public` schema — enough to run migrations
— but nothing beyond that; it's not superuser and can't touch roles or other
databases. `eda_user` gets no DDL rights at all, only what the two
`ALTER DEFAULT PRIVILEGES` statements grant it. That default-privileges trick is
the important part: it's not a one-time grant on today's tables, it's a standing
rule that says "any table `eda_dev` creates from now on, automatically grant
`eda_user` DML on it" — so Phase 2's migrations, and every migration in every
later project, don't each need their own manual `GRANT` afterward.

Since the compose file references a new bind-mounted path, this needs a full
recreation, not a restart — and since you confirmed the current volume only
holds smoke-test data, the cleanest path is to wipe it rather than trying to
replay `init.sh`'s idempotency dance on a live volume. Run:

```
docker compose down -v
docker compose up -d
```

Then verify the roles landed by connecting as each one over TCP (this goes
through the `scram-sha-256`/`127.0.0.1` line in `pg_hba.rules`, not the `peer`
line, so no `docker compose exec` OS-user complication here):

```
psql -h 127.0.0.1 -p 5433 -U eda_dev -d edadb -c "\du"
psql -h 127.0.0.1 -p 5433 -U eda_user -d edadb -c "\du"
```

Both should connect (password prompt — `devsecure` / `usersecure`) and print the
role list including `eda_dev` and `eda_user` alongside `edasuper`. Let me know
what you get.

#### Update the .env file

Right — but not to `eda_dev` for everything. The `.env` file backs
`config.Load()`, which every actual _service_ (the `stores` service in Phase 4,
`foundation-smoketest` today) uses to open its runtime connection pool. Runtime
traffic should go through `eda_user` — it's application DML only, no DDL —
matching the least-privilege point of doing this in the first place. `eda_dev`
is reserved for the migration step specifically, which we'll run separately (via
the `Makefile`'s upcoming `migrate` target, likely its own connection string)
rather than through this same `.env`/`config.Load()` path — mixing schema-owner
credentials into the app's runtime config would quietly undo the privilege split
we just built.

```env
# .env
DB_HOST=127.0.0.1
DB_PORT=5433
DB_USER=eda_user
DB_PASSWORD=usersecure
DB_DATABASE=edadb
DB_SSL_MODE=disable
NATS_URL=nats://127.0.0.1:4223
```

After saving that, re-run the smoke test (`make smoke`) to confirm the pool
still connects — now as `eda_user` instead of `edasuper`. It should succeed
identically, since `NewPool` only pings the database and `eda_user` has
`CONNECT` on `edadb`; nothing in the smoke test yet issues DDL or writes that
would expose the privilege difference.

Once that's confirmed, we're done with Step 1 — roles provisioned,
`pg_hba.rules` verified as already permissive enough, and the app's runtime
config now pointed at the least-privileged role. Ready to move to Step 2
(migrations for `stores`/`products`, where `eda_dev`'s credentials will actually
get used for the first time), or any questions on the role split first?

### **Step 2 — Migrations: schema for `stores` and `products`**

Before writing the schema itself, there's a decision `system-state.md` already
flagged: _"One database (`edadb`) so far; one per service as each is added."_
Right now everything — including `eda_dev`/`eda_user` — is scoped to the single
`edadb` database created by `POSTGRES_DB`. `stores` needs its own database, not
a schema inside `edadb`, to keep services from ever being able to reach into
each other's tables even at the Postgres level.

One wrinkle: `ALTER DEFAULT PRIVILEGES` is scoped per-database, not global — the
grants we set up in `02-roles.sh` only apply within `edadb`. A new database
needs its own `CREATE DATABASE` plus its own copy of those same grants run
_inside_ it.

```sh
# ./postgres/init/03-stores-db.sh
```

`CREATE DATABASE ... OWNER eda_dev` means `eda_dev` — not `edasuper` — owns
`storesdb` outright, which matters later since ownership is what lets `eda_dev`
run migrations without needing extra grants. The second block has to run as a
separate `psql` invocation targeting `storesdb` directly, because you can't `\c`
databases mid-script the way `GRANT`/`ALTER DEFAULT PRIVILEGES` need scoping
here — each database's default-privilege rule is independent.

This means adding a fourth mount line for `03-stores-db.sh` in
`docker-compose.yaml`, alongside `01-init.sh`/`02-roles.sh`, and another
`docker compose down -v && docker compose up -d` to pick it up on a fresh
volume. Does `storesdb` work as the naming convention for you (matching later
`depotdb`/`basketsdb`/etc. per service), or would you rather name these
differently before I lock it into the migration connection strings next?

#### Migration files

Locking those in: `id`/`store_id` as `TEXT PRIMARY KEY`/`TEXT` (indexed
automatically as the PK), `price` as `NUMERIC(10,2)` with a `CHECK` guarding the
domain's own non-negative invariant at the DB layer too, audit columns on both
tables, no `version` column yet.

Here are the migration files, following `golang-migrate`'s paired up/down
convention and the `01-stores/migrations/` path from the folder structure we
agreed on:

```sql
-- 01-stores/migrations/0001_create_stores_and_products.up.sql
```

```sql
-- 01-stores/migrations/0001_create_stores_and_products.down.sql
```

Assuming it's (or will be) ignored, I'd add the migrate-specific values to it
rather than inventing a second env file — keeps all database credentials in one
gitignored place instead of splitting them across two:

```env
# .postgres.env
POSTGRES_USER=edasuper
POSTGRES_PASSWORD=supersecure
POSTGRES_DB=edadb
POSTGRES_HOST_AUTH_METHOD=scram-sha-256
EDA_DEV_PASSWORD=devsecure
EDA_USER_PASSWORD=usersecure
MIGRATE_DB_HOST=127.0.0.1
MIGRATE_DB_PORT=5433
MIGRATE_DB_NAME=storesdb
```

```makefile
# Makefile
include .postgres.env
export

.PHONY: migrate-up migrate-down

migrate-up:
	migrate -path 01-stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/$(MIGRATE_DB_NAME)?sslmode=disable" \
		up

migrate-down:
	migrate -path 01-stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/$(MIGRATE_DB_NAME)?sslmode=disable" \
		down 1
```

`include .postgres.env` pulls those `KEY=value` lines in as Make variables, and
`export` passes them through to the shell each recipe line runs in — so
`$(EDA_DEV_PASSWORD)` in the target body resolves at runtime from the untracked
file, and the only thing that ends up in git is the variable name, never the
value. Only `.postgres.env` needs to be gitignored for this to hold; the
`Makefile` itself stays fully trackable with zero secrets in it.

Can you confirm `.postgres.env`'s current gitignore status, and if it needs
adding, go ahead and do that plus `git rm --cached .postgres.env` if it was ever
actually committed? Once that's settled, run `make migrate-up` and let me know
what you get.

### Step 3 — Repository port interfaces\*\*

Two files, following the same one-name-per-concern convention as
`internal/domain/events.go`. First, the interfaces themselves:

```go
// 01-stores/internal/domain/store_repository.go
```

```go
// 01-stores/internal/domain/product_repository.go
```

Second, `Reconstitute` constructors alongside the existing ones in `store.go`
and `product.go`:

```go
// 01-stores/internal/domain/store.go — add below NewStore
```

```go
// 01-stores/internal/domain/product.go — add below NewProduct
```

A few choices worth flagging. `Save` is one method for both insert and update —
since ADL 0002 committed to current-state rows, not event sourcing, there's no
meaningful "create vs. update" distinction at the repository boundary; the
Postgres adapter will implement it as a genuine
`INSERT ... ON CONFLICT DO UPDATE` upsert in Step 4, but the _interface_ doesn't
need to know that. `FindByID` returns `(*Store, error)` rather than
`(*Store, bool)`, with a dedicated `ErrStoreNotFound`/`ErrProductNotFound`
sentinel — matching Go's idiomatic error-based "not found" signaling rather than
a boolean flag, and giving the Postgres adapter (Step 4) something concrete to
map `pgx.ErrNoRows` onto. And `Reconstitute` exists as a separate constructor
from `NewStore`/`NewProduct` specifically because it skips validation and
doesn't raise a domain event — it's rebuilding an aggregate that's already
known-valid from a database row, not creating a new one, so re-running
`NewStore`'s blank-name check or emitting a spurious `StoreCreated` event would
be wrong on both counts.

These are pure interfaces and constructors with no external dependencies, so
there's nothing to run yet — just `go build ./...` from `01-stores/` to confirm
it compiles cleanly against the existing domain types. Any questions on the
`Save`-as-upsert or the `Reconstitute` design before we move to Step 4, the
actual Postgres adapter for `Store`?

That's exactly the right question to press on, and answering it makes the shape
of the interface fall out naturally rather than being a stylistic pick.

In the book's event-sourced model, `stores`/`products` in Postgres _isn't_ a
queryable table of current state at all — it's an append-only event stream
(`internal/es`, per ADL 0002's research). You genuinely cannot run
`SELECT * FROM products WHERE store_id = ?` against that; there's no row to
select, only a sequence of events you'd have to replay. That's _why_ the book
needs a second, separate read-model repository (`CatalogRepository`) — the
write-side event stream is structurally incapable of answering read queries, so
a denormalized projection has to exist somewhere or nothing could ever be listed
or searched.

We don't have that problem, because ADL 0002 already solved it a different way:
our `stores`/`products` tables in Postgres _are_ current-state rows — the same
table Step 2 just created is directly queryable right now
(`SELECT * FROM products WHERE store_id = ?` works today, no projection
required). So the read model isn't a separate thing we still need to build —
it's the same table our `StoreRepository`/`ProductRepository` already point at.
The question isn't "do we need a second repository," it's narrower: does
listing/searching need to go through the aggregate-shaped `Load`/`Save`
interface at all, or can it be its own simpler query path returning flat
rows/DTOs directly from the same table, alongside (not instead of)
`Load`/`Save`?

That reframes your original concern precisely: it's not that `Load`/`Save` is
too shallow as an aggregate repository — it's correctly shaped for that job.
It's that _some_ read-query surface (list products by store, search by SKU,
etc.) is still genuinely missing, and it doesn't belong bolted onto
`StoreRepository`/`ProductRepository` as extra methods returning
`*Store`/`*Product` — a "list all products in a store" doesn't need full
aggregate rehydration, domain event capability, or invariant-checked
construction, it needs rows. Does that framing match what you were getting at,
and if so — do we want that query surface addressed now in Phase 2, or deferred
until Phase 4 when the ConnectRPC API actually defines what queries the
storefront needs?

### Step 4 — Postgres adapter: Store

Good — that keeps `StoreRepository`/`ProductRepository` as `Save`/`FindByID`
exactly as drafted in Step 3, with no `Delete` or list methods added, and no
CQRS split introduced at this stage. That interface is already compiling cleanly
against the domain types, so Step 3 is done.

Moving to Step 4: the Postgres adapter for `Store`. Before writing it, one thing
needs deciding per ADL 0002 — whether `Store`'s concurrency control is row-level
locking (`SELECT ... FOR UPDATE`) or optimistic version-column CAS, decided by
its actual write pattern rather than defaulted.

`Store`'s writes are `Rebrand`, `EnableParticipation`, `DisableParticipation` —
infrequent, low-contention, typically triggered by a single store owner acting
on their own store. That's a different shape from, say,
`Product.IncreasePrice`/`DecreasePrice`, where a race is at least plausible (two
admins editing pricing concurrently). Given `court-reservation` already proved
out row-level locking successfully and `Store`'s contention risk is low, I'd
lean toward `SELECT ... FOR UPDATE` here — simpler code, no extra schema column,
and appropriate to the actual risk.

#### Adapter

Here's the adapter. Row-level locking only means something if the row stays
locked across the read-modify-write sequence — a single `SELECT ... FOR UPDATE`
issued through a connection pool with no explicit transaction locks and
immediately releases, which makes the lock pointless. So the adapter needs to
accept either a plain pool (for reads with no locking intent) or a transaction
(where the lock actually holds until commit) — both `*pgxpool.Pool` and `pgx.Tx`
satisfy the same method set in pgx v5, so a small `Executor` interface lets the
same adapter code work with either, and Phase 4's command handlers will be the
ones deciding which to hand in.

```go
// 01-stores/internal/postgres/store_repository.go
```

`Save` is a genuine upsert — `ON CONFLICT (id) DO UPDATE` — so it covers both
the first-time creation from `NewStore` and later updates from
`Rebrand`/`EnableParticipation` with the same call, matching the interface's
single-`Save` design from Step 3. `created_at` is deliberately left out of both
the column list assignment and the `SET` clause on conflict, so it's only ever
written once by its `DEFAULT now()` at insert and never touched again.
`FindByID`'s `FOR UPDATE` is real but currently inert when called through a bare
`*pgxpool.Pool` — it'll only actually hold a lock once Phase 4 constructs this
repository with a `pgx.Tx` spanning the load, the domain method call, and the
`Save`, then commits. That wiring doesn't exist yet, so for now this adapter is
correctly shaped but the locking is dormant.

For a smoke test, here's a throwaway script — adjust the import path if your
`01-stores/go.mod` module name differs:

```go
// 01-stores/cmd/repo-smoketest/main.go
```

Run it with `go run ./cmd/repo-smoketest` from `01-stores/`. Expected output is
`loaded: &{ID:store-001 Name:Corner Bakery Location:Belo Horizonte Participating:false}`
— let me know what you actually get, and whether the module path needed
adjusting.

#### Resolving Timestamp issues

That's not actually a bug in the row — it's a display artifact of two different
sessions using two different `TimeZone` settings, and it's worth confirming that
distinction carefully rather than assuming the timestamp itself is wrong.
`TIMESTAMPTZ` in Postgres always stores an absolute instant internally as UTC;
what varies per-session is only how it's _rendered_ back to you, governed by
that session's `TimeZone` GUC. Your `storesdb` container is rendering in UTC
(Docker's official Postgres image defaults to UTC unless told otherwise), while
your local host's Postgres instance apparently has its session `TimeZone` set to
`America/Sao_Paulo` (`-03`), matching your actual location. `11:51:56+00` and
`08:52:20-03` aren't disagreeing about _when_ something happened — those are the
same instant, just labeled in two different zones.

Worth confirming directly, since it's a quick check: run `SHOW TimeZone;`
against both the `storesdb` container and your local `test` database, and you
should see `UTC` and something like `America/Sao_Paulo` respectively.

The actual decision is whether we want the container itself defaulting to UTC
display or to local time. I'd lean toward leaving it as UTC — it's the standard,
boring choice for a server that (eventually, per the roadmap) is meant to run
the same way regardless of which machine or region it's deployed in, and every
stored value converts to any zone correctly on the client/application side
regardless of the server's display default. Explicitly pinning it also removes
ambiguity: right now we're relying on the image's default rather than a value
we've deliberately set, which is exactly the kind of
implicit-default-vs-explicit-decision distinction ADL 0006 already flagged for
`pg_hba.conf`.

If you'd like, I can add `timezone = 'UTC'` to `hardening.conf` (alongside the
existing `password_encryption`/logging/`max_connections` settings) so it's a
stated decision rather than an inherited default — same file, same pattern, no
new mount needed. Want that, or would you rather set the container to
`America/Sao_Paulo` instead so raw `psql` inspection during development reads in

local time?

#### Local .env file for 01-store

Good call — a hardcoded connection string was exactly the wrong pattern to
establish right after we spent Step 1 careful about not leaking credentials into
git. Two changes.

First, the `hardening.conf` addition:

```conf
# postgres/config/hardening.conf — add alongside existing settings
timezone = 'UTC'
```

That's an `include_dir`-mounted config change, so per ADL 0006's own finding, a
restart or reload won't pick it up — it needs `docker compose up` to recreate
the container.

Second, the smoke test rewritten against `config.Load()`:

```go
// 01-stores/cmd/repo-smoketest/main.go
```

This needs `01-stores/.env` present with `DB_DATABASE=storesdb` and
`DB_USER=eda_user`/`DB_PASSWORD` matching what we set up in Step 1 —
`config.Load()` calls `godotenv.Load()` against the working directory, so it
needs its own `.env` alongside `01-stores/go.mod`, separate from the workspace
root's.

One more thing worth locking in given your actual concern — a hardcoded URI in
one file is a one-time mistake, but _any_ throwaway smoke test getting committed
is a recurring risk as long as we keep creating them under normal source paths.
Worth adding a rule to `.gitignore` now rather than remembering to
`git rm --cached` after the fact each time:

```gitignore
# .gitignore
**/cmd/*smoketest*/
```

That pattern catches any future `cmd/<name>-smoketest/` directory in any
project, workspace-wide, so a throwaway script is unreachable by `git add` by
construction rather than by discipline. Want that added, and should I also
verify `01-stores/.env` exists with the right values before you re-run this?

### **Step 5 — Postgres adapter: `Product`**

Good — consistent with `Store`, and the contention risk is real enough to
justify it. Here's the `Product` adapter:

```go
// 01-stores/internal/postgres/product_repository.go
```

Same shape as `Store`'s adapter — upsert `Save`, `FOR UPDATE` `FindByID` —
reusing the same `Executor` interface, so no new abstraction needed. On the
`float64`↔`NUMERIC(10,2)` conversion: `pgx/v5` natively converts between
`NUMERIC` and Go's `float64` on both `Exec` binding and `Scan`, so no manual
string formatting or `pgtype.Numeric` handling is needed here — the driver pays
that cost internally. That's fine for this direction (Go float in, Postgres
numeric out and back) precisely because the `CHECK (price >= 0)` constraint and
`NUMERIC(10,2)`'s fixed-point storage on the Postgres side are what actually
prevent drift; the float64 round-trip through pgx isn't doing arithmetic, just
representation conversion at the boundary.

For the smoke test, extend `repo-smoketest/main.go` to add a product under
`store-001` and reload it:

```go
// 01-stores/cmd/repo-smoketest/main.go — add after the store FindByID block
```

Run it and confirm both the console output (price should print as `8.5`, no
precision drift) and the raw row via `psql` (`SELECT * FROM products;` — `price`
should read `8.50`). Let me know what you get.
