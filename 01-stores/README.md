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

---

## **Phase 3 — Event Publishing**

Here's the reset Phase 3 Step plan:

**Step 1 — Composition root and service boot.** Build `module.go` (config load →
logger → Postgres pool → NATS connect) and `cmd/service/main.go` wiring it
together, using `foundation/config`, `foundation/logging`,
`foundation/postgres`, and `foundation/nats` — the same pieces
`foundation-smoketest` already proved work, now assembled for real as
`01-stores`'s own entry point. No domain logic touched. Verified by starting the
service and confirming it connects to both live containers cleanly (and shuts
down gracefully via `foundation/shutdown`).

**Step 2 — The `stores` stream and subject scheme.** Define the subject naming
convention (`stores.store.*` / `stores.product.*`, per Option A already agreed)
and wire the `EnsureStream` call into `module.go`'s NATS setup. Verified against
the live `stores` stream via `nats stream info`.

**Step 3 — Event → payload mapping.** `internal/events/publisher.go`: marshal a
domain `Event` into its NATS wire payload, distinct from the domain event
struct.

**Step 4 — Wire aggregate actions to publish.** After a repository `Save`
succeeds, publish the resulting event. Happy path only.

**Step 5 — Failure modes.** Decide and implement what happens when publish fails
after save succeeds.

**Step 6 — Smoke test.** Exercise the real service boot path (from Step 1)
end-to-end: create a `Store`, save it, publish `StoreCreated`, verify it lands
on NATS for real.

This also means Step 1 is doing double duty — it's the piece Phase 4's "Wiring,
Compose, and Verification" description in `journey_progress.md` was going to
need anyway, built now instead of deferred, since Phase 3 can't publish anything
without a live NATS connection to publish through.

One thing I need before laying out Step 1's code: do you want `module.go`'s
`NewModule`-style constructor (or whatever we call it) to own the
`context.Context` used for connecting to Postgres/NATS at startup, or should
`main.go` build the context and pass it in? `court-reservation`'s pattern is
worth checking for precedent here if you have it handy — otherwise I'll default
to `main.go` owning it, since that's what `shutdown.Run`'s signature already
implies (it takes a `*http.Server`, not a context, and does its own signal-based
context creation internally for the shutdown phase specifically, not the startup
phase).

### Step 1 Module.go

```go
// 01-stores/module.go
```

```go
// 01-stores/cmd/service/main.go
```

What's happening here: `main.go` still owns `context.Background()` exactly as
you described — `stores.New` receives it as a parameter rather than constructing
its own, and anything downstream that eventually needs a timed context (the
router's `TimeoutMiddleware`, per request) derives its own from `r.Context()` at
request time, not from this startup context. This mirrors `court-reservation`'s
split cleanly: `main()` handles config/logger/composition, and everything past
that point is either handed off to `module.go` (startup-time dependency
construction) or to the router/middleware stack (per-request concerns) — no
long-lived startup context leaking into request handling.

The bigger design choice worth flagging is that `Module` doesn't just hold
`Pool`/`NC`/`JS`/`Publisher` — it already wires `StoreRepo`/`ProductRepo` too,
using the exact repository constructors Phase 2 built. Nothing in Phase 3
strictly needs the repositories wired yet, but this is genuinely the composition
root's job description, and building it here means Step 6's smoke test (and
eventually Phase 4's ConnectRPC handlers) can just reach for `mod.StoreRepo`
instead of constructing it a second time somewhere else.

`/health` exists purely to make "are all components online" something you can
actually observe from outside the process, rather than trusting the startup log
lines alone — same motivation as `foundation-smoketest`'s `/ping`, but checking
liveness only (`pool.Ping`, `nc.Status()`) rather than exercising a real write
path, since that's this step's job, not Step 1's.

One thing to double check on your end before running: your `.env` (or real
environment) needs `DB_USER`/`DB_PASSWORD` pointed at `eda_user` against
`storesdb`, per what Phase 2 already set up — not the superuser or `eda_dev`,
since this is runtime traffic, not migrations.

To verify: run `go run ./cmd/service` from `01-stores/`. You should see the
`🗄️ postgres pool ready` and `nats connected` log lines, then
`🚀 starting server` from `shutdown.Run`. In another terminal,
`curl localhost:8080/health` (adjust the port to whatever `SRV_PORT` resolves
to) should return `ok` with a 200. Then `Ctrl+C` the running service and confirm
you see the graceful `shutdown complete` log line rather than an abrupt kill.

Let me know what you get, or paste whatever error surfaces if something doesn't
connect.

#### .Env file and NATS Name

Good — that keeps `foundation-smoketest` self-contained the same way `01-stores`
now is, and the reasoning on `.postgres.env` is spot on: it's superuser-scoped
specifically for `docker-entrypoint-initdb.d` bootstrap work (creating roles,
databases, the `eda_dev`/`eda_user` split), a different concern entirely from a
Go service's own runtime credentials.

To recap where that leaves things: move the root `.env` into
`foundation-smoketest/`, add `NATS_NAME=stores-service` to `01-stores/.env`, and
apply the `cd`-into-directory fix to the `stores-run` Makefile target. Once
those three are in place, we're back to verifying Step 1 for real.

Go ahead and make those changes, then run `make stores-run` from the repo root.
You should see the `🗄️ postgres pool ready`, `nats connected`, and
`🚀 starting server` log lines in sequence, and `curl localhost:8080/health`
(adjust the port to whatever `SRV_PORT` resolves to in your `.env`, default
`8080`) should return `ok`. Then `Ctrl+C` the running process and confirm you
get a clean `shutdown complete` line rather than an abrupt kill. Let me know
what you get, or paste the error if something doesn't come up cleanly.

### **Step 2 — The `stores` stream and subject scheme.**

Picking back up from the agreed Option A scheme, here's the subjects file and
the `EnsureStream` wiring into `module.go`.

```go
// 01-stores/internal/events/stream.go
```

And the addition to `module.go`, right after the NATS connection is established
and before the repositories are constructed:

```go
// 01-stores/module.go (addition after NATS connect, before repository construction)
```

This needs a new import in `module.go`:

```go
// 01-stores/module.go (new import)
storesevents "github.com/dupakarovsky/eda-storefront/stores/internal/events"
```

What's happening here: I aliased the import `storesevents` rather than plain
`events`, consistent with the `foundation`-prefix convention you set up in Step
1 — `events` alone risks colliding with a stdlib-adjacent or third-party
`events` package down the line, and the alias makes it unambiguous at the call
site that this is `01-stores`'s own event-subject definitions, not
`foundation`'s NATS layer.

Notice the error path explicitly closes `nc` and `pool` before returning — this
matches the pattern already established for the NATS-connect failure case (where
`pool.Close()` runs before returning), just extended one step further now that
there's a third thing that can fail after two things have already succeeded. If
`EnsureStream` fails, we don't want a half-initialized `Module` leaking a live
Postgres pool and NATS connection that nothing will ever call `Close()` on,
since the constructor is returning `nil, err` rather than a `*Module` the caller
could clean up itself.

The stream itself: `EnsureStream` uses `CreateOrUpdateStream`, so this is safe
to call every time the service boots — first run creates the `stores` stream
with `FileStorage`, every subsequent boot just confirms the config still matches
(or updates it in place if the subject list ever changes) rather than erroring
on "stream already exists." That's what lets us treat this as a boot-time step
rather than a separate one-off migration-style command.

To verify: run `make stores-run` again. You should see the same log sequence as
before, plus a new `🌊 stream ensured` line showing `name=stores` and both
subjects. Then, in another terminal, check it landed for real — either
`docker compose exec nats nats stream info stores` (you may need the `nats` CLI
installed inside or alongside the container) or the NATS monitoring UI at
`http://127.0.0.1:8222` (per your `docker-compose.yaml`'s exposed `8222` port)
under the JetStream section — either should show a `stores` stream with
`stores.store.*` and `stores.product.*` as its configured subjects.

Let me know what you get.

That's enough to confirm success, even without the CLI. The server log's
`🌊 stream ensured` line already tells us `CreateOrUpdateStream` returned without
error for exactly the config we specified, and `/jsz`'s `"streams": 1`
independently confirms JetStream now has one real stream registered on the
server side — not just a client-side success, but server-side state that
persisted.

The `docker compose exec nats stream info stores` failure is just because the
official `nats` image only ships the `nats-server` binary, not the `nats` CLI
client — two separate binaries, and the container only has the first. Not worth
installing the CLI into the container for this; the monitoring HTTP endpoints
already give us what we need.

If you want to see the actual subjects rather than just the count, `/jsz` takes
query params — this'll show the full stream config:

```bash
curl "localhost:8222/jsz?streams=true"
```

That should return a `stream_detail` (or similar, depending on server version)
block showing `name: stores` and both `stores.store.*` / `stores.product.*`
subjects, which would be the closest thing to `stream info`'s output without the
CLI. Optional at this point though — between the clean boot log and `/jsz`'s
stream count, Step 2 is confirmed.

Any questions before we move to Step 3 — building
`internal/events/publisher.go`, the piece that maps a domain `Event` to its
actual NATS wire payload?

### **Step 3 — Event → payload mapping.**

Before writing the publisher itself, there's a real snag worth stopping on — not
a design choice this time, but a discrepancy between what Phase 1 already built
and what the Step 2 subject scheme assumes.

Look at `domain/events.go`'s constants:

```go
StoreCreatedEvent               = "stores.StoreCretated"
StoreParticipationEnabledEvent  = "stores.StoreParticipationEnabled"
StoreParticipationDisabledEvent = "stores.StoreParticipationDisabled"
StoreRebrandedEvent             = "stores.StoreRebrandEvent"

ProductAddedEvent          = "stores.ProductAdded"
ProductRebrandedEvent      = "stores.ProductRebranded"
ProductPriceIncreasedEvent = "stores.ProductPriceIncreased"
ProductPriceDecreasedEvent = "stores.ProductPriceDecreased"
ProductRemovedEvent        = "stores.ProductRemoved"
```

Three problems if the publisher naively calls `event.EventName()` to build the
subject's final segment:

1. **A typo**: `StoreCreatedEvent` is literally `"stores.StoreCretated"` —
   missing the `a`.
1. **An inconsistency**: `StoreRebrandedEvent` is `"stores.StoreRebrandEvent"` —
   the only one with a trailing `Event`, and it doesn't match the struct name
   `StoreRebranded` the way every other constant matches its struct.
1. **A structural mismatch with Step 2's own scheme**: every one of these
   already has a `"stores."` prefix baked in. If the publisher does
   `foundationnats.Subject("stores", "store", event.EventName())`, you get
   `stores.store.stores.StoreCretated` — a doubled, typo'd, ugly subject — not
   the clean `stores.store.StoreCreated` we designed and already registered on
   the stream in Step 2.

This wasn't visible until now because `EventName()` has only ever been used
internally (tests asserting the right event type fired) — Step 3 is the first
place these strings actually leave the process and become a real wire contract.

Two ways to resolve it:

**Option A — Fix `domain/events.go` directly.** Strip the `"stores."` prefix and
the `Event` suffix inconsistency, so the constants become just the bare type
names: `StoreCreatedEvent = "StoreCreated"`,
`StoreRebrandedEvent = "StoreRebranded"`, etc. Then `event.EventName()` cleanly
slots into `foundationnats.Subject("stores", "store", event.EventName())` with
no special-casing anywhere. This is the same discipline Phase 1 already used
once — recall the `<=`/`>=` price-comparison fix, caught and corrected before it
became load-bearing anywhere else. Nothing external depends on these values yet
(no consumer service exists, nothing's been published), so it's a free fix right
now and gets expensive the longer it waits.

```go
// 01-stores/internal/domain/events.go (corrected constants block)
const (
    StoreCreatedEvent               = "StoreCreated"
    StoreParticipationEnabledEvent  = "StoreParticipationEnabled"
    StoreParticipationDisabledEvent = "StoreParticipationDisabled"
    StoreRebrandedEvent             = "StoreRebranded"

    ProductAddedEvent          = "ProductAdded"
    ProductRebrandedEvent      = "ProductRebranded"
    ProductPriceIncreasedEvent = "ProductPriceIncreased"
    ProductPriceDecreasedEvent = "ProductPriceDecreased"
    ProductRemovedEvent        = "ProductRemoved"
)
```

Everything else in the file — the `Event` interface, `eventBase`, every concrete
struct and its `EventName()` method — stays untouched, since they all just
return the constant, not a hardcoded string. This is purely a constants-block
edit.

What's happening here: since every `EventName()` method returns one of these
constants rather than a literal string, fixing the constants alone propagates
correctly everywhere `EventName()` is called — no other file needs touching.
This is also exactly why the earlier "does the test suite still pass" question
resolves itself cleanly: if `store_test.go`/`product_test.go` assert
`event.EventName() == domain.StoreCreatedEvent` (comparing against the same
constant, not a hardcoded string), the tests are still internally consistent
after this change and require no edits either. If any test instead hardcoded the
string `"stores.StoreCretated"` directly rather than referencing the constant,
that would be the one place needing an update — worth a quick check on your end.

Go ahead and apply that edit to `events.go`, then run your existing Phase 1 test
suite (`go test ./internal/domain/...` from `01-stores/`) to confirm
everything's still green with the corrected constants. Once that's confirmed, we
can move into the actual publisher — mapping a domain `Event` into the JSON
payload that goes out over NATS.

#### Added the Payload types for Event Publishing

Good instinct — and the two things you flagged aren't cosmetic, they're actually
two faces of the same underlying issue: reusing the domain event struct directly
for JSON encoding conflates two things that should be independent — the domain's
internal shape and the wire contract another service will code against.
PascalCase leaking through is just the visible symptom; the reconstitution
concern is the real one.

Here's the fix: a dedicated payload type per event, living in the `events`
package (not `domain`), with real `json` tags — and a mapping step that converts
a `domain.Event` into its corresponding payload type. `domain/events.go` stays
exactly as it is, untouched by any transport concern.

```go
// 01-stores/internal/events/payloads.go
package events
```

And the conversion — a type switch mapping each concrete `domain.Event` to its
payload:

```go
// 01-stores/internal/events/payload_mapper.go
```

And `NewEnvelope` changes to call it instead of marshaling `event` directly:

```go

```

What this fixes and why it matters beyond casing: the `default` case in
`toPayload` is the real payoff. Every domain event now needs an explicit,
deliberate payload mapping before it can ever be published — there's no path
where a ninth event type gets added to `domain/events.go` later and silently
starts flowing over NATS with a Pascal-case, unreviewed shape, because
forgetting to add its case here means the publisher returns an error instead of
guessing. That's a much better failure mode than "it worked, but the wire format
is an accident of Go's default marshaling."

It also directly answers your reconstitution concern: nothing in this design
ever unmarshals back into `domain.StoreCreated`. A consumer decodes `Envelope`,
sees `event_name: "StoreCreated"`, and unmarshals `Data` into whatever local
shape it wants — it never needs to import `stores`' domain package at all, which
is exactly the boundary ADL 0004 already draws (contracts travel forward,
implementations don't). The payload types in `events.go` are themselves the
contract — literally what `CONTRACT.md`'s payload section will end up
documenting almost verbatim.

I've only written `StoreCreatedPayload`/`ProductAddedPayload` above as the
pattern to validate first — want to re-run your round-trip test against just
`StoreCreated` to confirm the `data` field now comes out as
`{"name":"...","location":"..."}` in real snake_case, before I write out the
remaining seven payload types and their switch cases?

### Step 4 — Wiring aggregate actions to publish (happy path)

One more small design decision before the code: `CreateStore`'s `Execute` needs
to build a NATS subject for the event it just published, which means it needs to
know the aggregate segment (`"store"` vs `"product"`) for whatever event came
back. Hardcoding `foundationnats.Subject("stores", "store", event.EventName())`
directly in `CreateStore` works today, but it's the same kind of fragile,
easy-to-typo string-building we already caught and fixed twice this phase (the
`events.go` constants, the PascalCase payload issue). Better to centralize
subject resolution in the `events` package itself, next to `toPayload`, using
the same exhaustive-switch-with-error-default discipline:

```go
// 01-stores/internal/events/subject.go
```

Now the use case itself:

```go
// 01-stores/internal/application/create_store.go
```

And wiring it into the composition root:

```go
// 01-stores/module.go (new field on Module)
CreateStoreUC *application.CreateStore
```

```go
// 01-stores/module.go (construction, after storeRepo/productRepo are built)
createStoreUC := application.NewCreateStore(storeRepo, publisher, logger)
```

```go
// 01-stores/module.go (add to returned Module literal)
CreateStoreUC: createStoreUC,
```

What's happening here: `Execute` is deliberately sequential and unguarded — save
first, then publish, no rollback, no transaction spanning both. That's
intentional for this step; it's exactly the happy-path-only scope we agreed to,
and it's what makes Step 5 a real, separate design decision rather than
something quietly baked in here. Right now, if `Save` succeeds and `Publish`
fails, the store exists in Postgres but no `StoreCreated` ever reaches NATS — a
real gap we're choosing to see clearly before deciding how to close it, rather
than papering over it with a guess.

Notice `CreateStore` depends on `domain.StoreRepository` (the interface) and
`*foundationnats.Publisher` (the concrete type) — not `domain.Store` directly
and not an interface for the publisher. The repository interface is already
established by Phase 2; the publisher stays concrete because `foundation`
doesn't define a `Publisher` interface for us to depend on instead, and
introducing one here, in `01-stores`, would mean inventing an abstraction
`foundation` itself doesn't own — better to accept the concrete type for now
than to add a speculative interface with exactly one implementation.

Since this touches real Postgres and NATS, it's worth a small throwaway smoke
command to verify — separate from Step 6's more complete smoke test, just enough
to confirm `Execute` actually works end to end:

```go
// 01-stores/cmd/createstore-smoketest/main.go
```

This matches Phase 2's `repo-smoketest` naming pattern, so it's already covered
by the `**/cmd/*smoketest*/` `.gitignore` rule you added back then — no extra
ignore-list edits needed.

Run it with `go run ./cmd/createstore-smoketest` from `01-stores/` (make sure
the real service from `make stores-run` isn't already holding the port or NATS
name conflicting — this is a separate short-lived process, so that shouldn't
matter, but worth knowing). You should see `🗄️ postgres pool ready`,
`nats connected`, `🌊 stream ensured`, then `event published` with the subject
`stores.store.StoreCreated` and a stream/sequence number, then `store created`.
Afterward, `curl "localhost:8222/jsz?streams=true"` should show the stream's
message count incremented by one from wherever Step 2 left it.

Let me know what you get.

#### Inspectign NATS

The `/jsz` output confirms it independently — `messages: 1`, `first_seq: 1`,
`last_seq: 1`, `bytes: 230`, `num_subjects: 1` all line up exactly with what the
log already told us. Two independent sources (Postgres row, JetStream state) now
agree Step 4 worked end to end.

On the network name: almost certainly not a real failure, just a naming
mismatch. Docker Compose namespaces networks by prefixing them with the project
name (normally your directory name, `eda-storefront`) unless a network is
explicitly given a top-level `name:` override. Your `docker-compose.yaml`
declares `eda-net` under `networks:`, but the network Docker actually created is
probably `eda-storefront_eda-net`. Quick check:

```bash
docker network ls | grep eda
```

That'll show you the real name to pass to `--network` on the next attempt.

On your suggestion — I'd lean away from replacing the `nats:2.12` image in
`docker-compose.yaml` itself. That image is specifically the well-tested,
minimal server image; swapping it for a CLI-bundled variant (or building a
custom one) means now maintaining a custom image just for occasional debugging
convenience, and any future NATS version bump has to be re-applied to your
custom build instead of just bumping a tag. Better to add `nats-box` as its own
**separate, non-serving service** in `docker-compose.yaml` — no ports, no
dependency on it starting, just sitting on `eda-net` so it's always one
`docker compose exec` away:

```yaml
# docker-compose.yaml (new service, added to existing services: block)
nats-box:
    image: natsio/nats-box:latest
    container_name: eda-nats-box
    restart: unless-stopped
    networks:
        - eda-net
```

Then anytime you want to inspect NATS, it's just:

```bash
docker compose exec nats-box nats stream info stores --server nats://nats:4222
```

Using the compose _service name_ (`nats`) as the hostname rather than the
container name (`eda-nats`) — Compose's internal DNS resolves service names
automatically on the shared network, which is one less thing to remember
correctly versus the explicit `container_name`. Worth adding this now since it's
cheap, permanent, and every future project (`depot`, `baskets`, and onward) will
want the same debugging visibility into their own streams.

That's optional infrastructure polish though — not blocking. Step 4 itself is
fully confirmed: `CreateStore.Execute` saves to Postgres, builds the envelope,
resolves the subject, and publishes to JetStream, verified against two
independent live sources.

Any questions before Step 5 — deciding what happens when `Publish` fails after
`Save` already succeeded?

## Step 5 — Failure modes: publish fails after save succeeds

This is worth treating carefully — not because Phase 3 needs a perfect solution,
but because whatever we decide here is a pattern that will repeat in every
future service (`depot`, `baskets`, `ordering`, and onward all face the
identical shape of problem: local write succeeds, integration event needs to
reach NATS). Getting the reasoning right once, now, is worth more than getting
`01-stores` narrowly correct.

The core problem: `CreateStore.Execute` currently does `Save` then `Publish`,
sequentially, with no coordination between them. Four real ways to fail:

1. `Save` fails — no problem, nothing was persisted, `Execute` returns the
   error, caller retries the whole operation. Already handled correctly today.
1. `Publish` fails — the store now exists in Postgres, but the event that was
   supposed to tell the rest of the system about it never left the building.
   `depot`/`baskets` (once they exist) will never know this store exists,
   silently and permanently, unless something notices and retries.
1. The process crashes between `Save` returning and `Publish` being called —
   same outcome as #2, just triggered by a crash instead of a NATS error.
1. `Publish` succeeds but the ack times out or is lost — genuinely rare with
   JetStream's ack semantics, but worth naming as distinct from a hard publish
   failure.

Three real options for handling #2/#3, in increasing order of correctness and
complexity:

**Option A — Log and continue.** `Publish` fails, `Execute` logs it at `Error`
level with everything needed to manually reconcile (aggregate ID, event name,
timestamp) and returns the error to the caller anyway — the store row still
exists, but the caller now knows integration didn't fully complete. No automatic
recovery; a human (or a future tool) has to notice the error log and manually
replay. Simplest to build, weakest guarantee — genuinely fine for a
learning-journey pace where no real consumer exists yet to notice the gap, but
it's a permanent silent-drop risk if left as the final answer.

**Option B — Retry with backoff, inline.** Before giving up, `Execute` retries
`Publish` a few times with backoff (NATS being briefly unreachable is the
realistic failure mode here, more than a permanent one). Closes the
transient-blip case cleanly, but does nothing for the
crash-between-save-and-publish case (#3) — if the process dies mid-retry, the
retry loop dies with it, and nothing durable remembers the event needed to go
out. Moderate complexity, partial guarantee.

**Option C — Transactional outbox.** Instead of publishing directly, `Execute`
writes the event into an `outbox` table in the _same database transaction_ as
the aggregate save — so either both the store row and the outbox row land, or
neither does; there's no window where one exists without the other, even across
a crash. A separate process (a goroutine started in `module.go`, or later a
dedicated worker) polls the outbox table and publishes pending rows to NATS,
marking them published (or deleting them) once the ack comes back. This is the
pattern that actually survives all four failure cases above, including a crash
mid-operation, because durability comes from Postgres's own transaction
guarantees rather than from the NATS call succeeding in the same breath as the
save. It's real added complexity for this phase, though — a new table, a new
migration, a poller/worker loop, and a decision about polling interval vs.
something event-driven (e.g. `LISTEN`/`NOTIFY`).

Given ADL 0002's own framing — current-state persistence was chosen specifically
because event sourcing wasn't needed for this system, but that ADL explicitly
kept cross-service event-driven integration as the one thing that must stay
fully real — I think this is worth naming honestly: **Option C is the
actually-correct answer for a system whose whole premise is real, durable,
event-driven integration**, and it's also a pattern that will pay for itself the
moment `depot`/`baskets` exist and start depending on never missing a
`StoreCreated`. But it's also a meaningfully bigger scope increase within Phase
3 than Steps 1–4 have been, and there's a reasonable argument for landing Option
A now (cheap, honest about its own limitation) and treating the outbox pattern
as a deliberate, separate future decision — possibly its own ADL — once a second
service actually exists to make the gap concrete rather than hypothetical.

Given the scope of this project (a learning journey, not production
infrastructure under real load) and that no consumer exists yet to be harmed by
the gap, which direction do you want to take: build the outbox pattern now
(Option C), take the cheaper retry approach (Option B), or log-and-continue with
the gap explicitly documented as a deferred decision (Option A)?

Good call — landing the honest, cheaper fix now and treating the outbox as a
deliberate future decision (with its own ADL noting _why_ it was deferred, not
just that it was) keeps this phase moving without pretending Option B is more
than it is.

Here's the retry wrapper, then the updated `Execute`.

```go
// 01-stores/internal/application/publish_retry.go
```

```go
// 01-stores/internal/application/create_store.go (Execute, updated)
```

What's happening here: `publishWithRetry` retries up to three times total, with
delays of 200ms, then 400ms (`baseDelay * 2^(attempt-1)`) — enough to ride out a
brief NATS blip (a reconnect window, a momentary network hiccup) without making
a failed request hang for an unreasonable amount of time. It respects `ctx`
throughout — if the caller's context is canceled or times out mid-retry, we stop
waiting immediately rather than sleeping through a delay pointlessly.

The more interesting design choice is in `Execute`'s final error branch: **it
returns `store, err` — a non-nil store alongside a non-nil error** —
deliberately breaking the usual Go convention of "error means the other return
value is worthless." Here, the store genuinely was saved; only the publish
failed. A caller that only checks `err != nil` and discards everything still
does the safe thing (treats the whole operation as failed), but a caller that
wants to distinguish "nothing happened" from "the write succeeded but
integration didn't" can inspect `store` even when `err` is non-nil. That
distinction is exactly what Option A's "log and continue" was gesturing at, now
captured as a real return-value signal rather than only a log line.

The `Error`-level log line is the honest acknowledgment of this option's real
limitation — after three attempts, if NATS is still unreachable, this line is
the only trace that a store now exists without a corresponding event ever
reaching the stream. Nothing automatically retries it further; a human reading
logs (or, later, a real outbox) is the only recovery path. Worth searching for
eventually if this pattern needs a permanent fix.

For the ADL, here's a draft — next number after `0006` in your `docs/adl/`
sequence:

```markdown

```

To verify the retry path actually does something (rather than trusting it by
reading the code), the cleanest test is forcing a real publish failure: stop the
NATS container, run the smoke test, watch it retry and eventually fail with the
`Error` log line, then restart NATS.

```bash
docker compose stop nats
```

Then, from `01-stores/`, run `go run ./cmd/createstore-smoketest` with a fresh
store ID (not `store-smoketest-1`, since that one already exists) — you should
see `postgres pool ready` succeed, then the NATS _connect_ itself likely fail
immediately at startup (since `Connect` happens before `CreateStore.Execute` is
ever called) rather than getting as far as the retry loop at all. That's worth
noting as a real limitation of this specific test: a stopped NATS container
fails the initial connection, not the publish call — so it won't actually
exercise `publishWithRetry`'s retry loop, just the earlier `module.New` failure
path.

To genuinely exercise the retry loop itself, we'd need NATS reachable at startup
but the publish call specifically failing partway through — harder to simulate
with a simple `docker compose stop`. Given that, would you rather (a) accept
this as a known testing limitation and move on, since the code path itself is
straightforward enough to trust by inspection, or (b) take a bit more time to
engineer a real publish-specific failure (e.g., temporarily pointing `NATS_URL`
at a valid-but-wrong port so the connection succeeds but publishes fail, or
another approach you have in mind)?

### Testing stopping NATS conatiner

Good instinct — this gives us a real, repeatable way to force a publish failure
without guessing, and building it incrementally (stop first, restart later)
means we can verify each piece works before combining them into the actual retry
test.

Here's the blank test file with just the stop helper:

```go
// 01-stores/internal/application/create_store_test.go
```

What's happening here: `cmd.Dir = "../../.."` is the part most worth
double-checking against your actual layout before running. Go tests execute with
the package's own directory as the working directory — this test file lives in
`01-stores/internal/application/`, and `docker-compose.yaml` lives three levels
up at the repo root (`application/` → `internal/` → `01-stores/` → root). If
that path is wrong, `docker compose` will fail immediately with something like
"no configuration file provided," which is actually a useful signal, not a real
bug — it'll tell us right away if the relative path assumption is off rather
than silently doing the wrong thing.

`cmd.Stdout`/`cmd.Stderr` both point at the same `bytes.Buffer` rather than
being wired to the OS's real stdout/stderr — this is deliberate for a test:
`t.Fatalf`/`t.Logf` are the correct way to surface output inside Go's test
framework (it's what shows up under `-v`, or only on failure otherwise), rather
than letting a child process write directly to the terminal in a way `go test`
doesn't capture or attribute to the right test.

`t.Helper()` marks this function as a test helper so that if `t.Fatalf` fires,
the reported line number points at the _caller_
(`TestStopNATSContainer_Helper`'s call site) rather than at the line inside
`stopNATSContainer` itself — minor, but it's the right convention for any
function that exists purely to be called from within a test.

One thing to flag before you run this: after this test passes, your NATS
container will genuinely be stopped and will stay that way — there's no restart
in this file yet, that's the next piece we'll add. So right after running it,
you'll want to manually confirm it stopped (`docker compose ps` should show
`nats` as `Exited` or absent), and then manually bring it back with
`docker compose start nats` before doing anything else that needs a live NATS
connection.

Go ahead and run
`go test ./01-stores/internal/application/... -run TestStopNATSContainer_Helper -v`
and let me know what you get — including the captured output in the log line,
since that's the real confirmation the `docker compose` invocation and working
directory are both correct.

Good call bumping `publishMaxAttempts` to 5 — gives us more room to land the
restart inside the retry window instead of racing it.

Let's refactor the stop helper into something reusable for both directions, then
add a delayed-restart helper on top of it.

```go
// 01-stores/internal/application/create_store_test.go
```

What's happening here, and why it looks a bit different from
`stopNATSContainer`:

`restartNATSContainerAfter` launches its `docker compose start` call inside a
`go func()`, not inline — the whole point is that the test needs to keep running
(eventually calling `CreateStore.Execute`, once we wire this into the real retry
test) while the container comes back on its own timeline in the background,
rather than the test blocking on `time.Sleep(delay)` before doing anything else.

That goroutine uses `t.Errorf`, not `t.Fatalf`, and this isn't a style choice —
it's a real Go testing constraint. `t.Fatalf` calls `runtime.Goexit()`
internally, which only terminates the calling goroutine; if that goroutine isn't
the main test goroutine, `Goexit` just silently kills the background goroutine
without ever failing the test or printing anything useful. `t.Errorf` is
explicitly documented as safe to call from any goroutine — it just needs the
test itself to still be running (hasn't returned yet) when the goroutine
eventually calls it, which is exactly why `TestRestartNATSContainer_Helper` ends
with a `time.Sleep(4 * time.Second)` — one second longer than the restart delay,
giving the goroutine a guaranteed window to finish and report before the test
function returns and Go's test runner considers `t` done.

This sleep-past-the-goroutine pattern is a bit crude, and worth flagging as
something we'd want to tighten once this becomes the real retry test — a
`sync.WaitGroup` or a completion channel would let the test proceed the instant
the goroutine's done rather than always waiting the full padded duration, but
for validating the helper in isolation right now, a plain sleep is the simplest
thing that lets us see it work.

Before running: your NATS container should currently be stopped from the last
test (per `make ps`'s output only showing `postgres`), so this test will attempt
to `stop` an already-stopped container first — `docker compose stop` on a
stopped container is a no-op that still exits 0, so that shouldn't fail, but
worth confirming your `make ps` output going in so we know the starting state.

Run
`go test ./internal/application/create_store_test.go -run=TestRestartNATSContainer_Helper -v`
and paste what you get — specifically both log lines (the stop, and the
restart-after-delay one) and their timing relative to each other.

#### Testing the NATS publishWithRetry

Now the timing tells a coherent, physically sensible story. Every gap sits close
to
`attemptTimeout (1.5s, since the connection is fully closed and NATS returns immediately) + the backoff sleep before the *next* attempt`
— the first gap (~1.7s) matches 1.5s + 200ms, the second (~1.9s) matches roughly
1.5s + 400ms, and so on, climbing predictably rather than erratically. Total
elapsed time was 10.36 seconds — right in line with the ~10.5-second ceiling we
calculated (5 attempts × up to 1.5s, plus 200+400+800+1600ms of backoff), not
the unbounded, ambiguous multi-second block we saw before the fix. The store was
saved, the retry loop genuinely exhausted all 5 attempts on real, immediate
`"connection closed"` errors, and `Execute` correctly returned an error
alongside the saved store — exactly the failure-mode behavior ADL 0007
describes.

This confirms two things worth calling out explicitly: first, the earlier
successful 2-second-outage test really was proving our retry-with-backoff logic
(not NATS client buffering papering over it) — we now have a _contrasting_ run
showing what happens when the outage genuinely outlasts the retry budget, which
is exactly the counterfactual needed to trust the first result. Second, the
real, now-measured retry budget is ~10.5 seconds, not the original ~3-second
estimate — worth updating in the ADL before it's written down as final.

One update to ADL 0007's "Decision" section, replacing the vague "three
attempts, 200ms/400ms delays" (already stale since you bumped
`publishMaxAttempts` to 5 mid-session) with the real, tested number:

```markdown
## Decision
```

Also worth appending to Context, a line making clear this wasn't just theorized
but reproduced twice with different outage lengths — a 2-second outage inside
the retry budget (succeeds, single real retry observed) and a 15-second outage
exceeding it (exhausts all 5 attempts, fails cleanly with the aggregate still
saved) — both verified against a real, forcibly-stopped-and-restarted NATS
container, not simulated.

With this confirmed, Step 5 is done: the retry-with-backoff design is
implemented, its real timing behavior is measured and understood (not assumed),
and both the success and failure paths have been proven against a genuinely
interrupted NATS connection. That leaves Step 6 — the "real" smoke test
exercising the full service boot path end-to-end, which at this point might
honestly just be the `createstore-smoketest` command already built in Step 4,
since it already does exactly that. Want to treat Step 6 as complete via that
existing smoke test, or is there something more you'd want it to additionally
cover before closing out Phase 3?

## Phase 4 Plan

Per `journey_progress.md`, Phase 4's scope is: define `storespb/api.proto`, run
`buf` codegen, implement the ConnectRPC handlers (commands + queries), and mount
them on `foundation`'s `Router`.

Given the size of this, I'd like to break Phase 4 into these Steps:

**Step 1 — Proto service definition.** Design `storespb/api.proto`: the
`StoresService` with RPC methods for every command (mapping 1:1 to the domain
methods we already have) and queries (`GetStore`, and a catalog/product-listing
query — worth discussing exactly what shape this takes, since it's the first
read path in the project). No code generation yet, just the contract.

**Step 2 — `buf` tooling and codegen.** `buf.yaml` + `buf.gen.yaml`, run
codegen, confirm the generated Go + Connect stubs compile and land where
expected.

**Step 3 — First real handler: `CreateStore`.** Wire the existing `CreateStore`
use case into a Connect handler, mount it on the Router, verify with a real
request (`buf curl` or similar) — this is the "runnable" milestone for the
phase, proving the whole plumbing (proto → generated code → handler →
application layer → Postgres/NATS) end to end before adding the rest.

**Step 4 — Remaining Store command use cases + handlers** (`Rebrand`,
`EnableParticipation`, `DisableParticipation`), following the `CreateStore`
template.

**Step 5 — Product command use cases + handlers** (`Add`, `Rebrand`,
`IncreasePrice`, `DecreasePrice`, `Remove`).

**Step 6 — Query path** (`GetStore`, catalog/product query) — the first
genuinely new kind of code this project, since everything so far has been
write-side.

**Step 7 — Full wiring pass**: `module.go` composition root updated to build and
mount every handler, then a full verification sweep (curl/`buf curl` against
every method) before closing the phase.

A couple of things worth flagging before we start:

- Step 1 involves a real design decision — specifically, what the
  catalog/product query looks like (get one product? list all products for a
  store? both?) — since nothing in `journey_progress.md` commits to that shape
  yet. We'll work that out together in Step 1 rather than me assuming it.
- I'll keep every code snippet in the chat window per your guideline, and you'll
  create the files and run things on your end, reporting back before we move on.

---

### **Step 1 — Proto service definition.**

Both good defaults — server-side ID generation is the more typical REST/RPC
pattern anyway (lets the server own its own key format), and keeping queries
separate keeps `ListProducts` free to evolve independently (pagination, filters)
without dragging `GetStore`'s shape along with it.

Here's the proto service surface. One buf/proto-specific note before you look at
it: buf convention is versioned packages (`stores.v1`) and a `go_package` option
pointing at where the generated code should import from — this replaces the raw
`--go_out`/`--go-grpc_out` flags you'd have passed to `protoc` by hand.

```protobuf

```

What's happening here, and the choices worth flagging:

Every command RPC has its own dedicated request/response pair, even though
several share nearly identical shapes
(`EnableStoreParticipationRequest`/`DisableStoreParticipationRequest` are both
just a `store_id`). This is deliberate, not boilerplate laziness — it's the same
principle Phase 1 applied to domain events (one concrete type per event rather
than one generic type with a variant field). A dedicated message per RPC means
adding a field to one command later can never accidentally ripple into an
unrelated one that happens to share a message type today.

`IncreaseProductPriceRequest`/`DecreaseProductPriceRequest` both carry `price`
as the absolute target value, not a delta — this mirrors
`Product.IncreasePrice(price float64)`/`DecreasePrice(price float64)` in the
domain layer exactly (Phase 1's `journey_progress.md` notes the domain computes
its own delta internally for the event; the API caller never has to know or
supply that delta).

`RemoveProductResponse` is an empty message rather than reaching for
`google.protobuf.Empty`. That well-known type would work fine, but it pulls in
an extra proto import and a bit of buf module configuration (dependency
management for well-known types) that's a genuine buf-specific wrinkle — not
worth introducing on your very first proto file when a trivially empty message
does the same job. We can revisit that later if it bugs you.

Notice there's no `DeleteStore`, no batch/bulk create, and no generic "search"
endpoint — that's intentional and matches this system's stated non-goals (see
the `CONTRACT.md` template's "explicit non-goals" section): most cross-service
interaction happens over events, and this synchronous surface stays deliberately
narrow.

Nothing to run yet — this is just the contract. Take a look and let me know if
anything about the shape feels off (naming, missing fields, a command you'd want
represented differently), or if you're ready to move to Step 2 and get `buf`
actually generating code from this.
