# 6. Postgres hardening baseline: explicit HBA over entrypoint defaults

Date: 2026-08-14

## Status

Accepted

## Context

The official `postgres` image's entrypoint auto-generates `pg_hba.conf` at
first init with a `trust`-heavy default: `local` (Unix-socket) connections
and `host` connections from `127.0.0.1`/`::1` are all trusted with no
password, regardless of `POSTGRES_HOST_AUTH_METHOD` — that variable only
governs the catch-all `host all all all` line the entrypoint appends
afterward, not the `local` line `initdb` writes independently of it, nor
the loopback-specific `host` lines. Testing during this project's Phase 6
also established that Docker's port-forwarding does not preserve
`127.0.0.1` as a literal source address once a connection crosses into a
container's network namespace, making those auto-generated loopback
`trust` lines dead weight in this topology rather than a real path to
guard — but not something to leave unexamined on that assumption alone.

This system's services carry real (if currently synthetic) customer and
order data across multiple projects, each connecting to its own Postgres
instance. Treating local development infrastructure with the same
deliberateness as a cloud deployment — rather than relying on convenient
defaults that happen to be low-risk in a single-developer local setup — was
judged worth the fixed setup cost, once, in `foundation`, rather than
re-decided per project or discovered missing during an actual deployment.

## Decision

Adopt an explicit, hand-written Postgres authentication configuration,
replacing the entrypoint's auto-generated `pg_hba.conf` outright rather
than layering env-var overrides on top of it:

- A bind-mounted `docker-entrypoint-initdb.d/init.sh` appends
  `include_dir = '/etc/postgresql/conf.d'` to `postgresql.conf` on first
  init.
- A bind-mounted `hardening.conf` sets `password_encryption =
scram-sha-256` explicitly (matches the Postgres 14+ default; stated
  rather than relied upon), real connection/disconnection logging, a
  `max_connections` cap, and points `hba_file` at a custom rules file
  instead of the auto-generated one.
- A bind-mounted `pg_hba.rules` replaces the default rule set: `peer`
  authentication for the local Unix socket (scoped to the actual database
  superuser role name, not the OS user — see Consequences), `scram-sha-256`
  required for loopback and the Docker bridge subnet, and explicit `reject`
  rules for every other address as a legible, auditable fallback rather
  than relying on implicit no-match-means-refused behavior.
- The bridge network's subnet (`eda-net`) is pinned explicitly in
  `docker-compose.yml`'s `networks:` block rather than left to Docker's
  IPAM pool to assign, since the HBA rules reference that subnet directly
  and an unpinned, drifting subnet would silently invalidate them on some
  future network recreation.

## Consequences

- Every later project that stands up its own Postgres instance inherits
  this same pattern (`init.sh`/`hardening.conf`/`pg_hba.rules` trio, pinned
  subnet) rather than re-deciding hardening per service — the pattern is
  designed to be a reusable mental model, not a one-off for this project's
  instance specifically.
- **Mount-path and `include_dir` changes require container recreation, not
  restart.** `docker compose restart` and even a manual `SELECT
pg_reload_conf();` only re-read settings from files Postgres already
  knows about — they do not discover a newly-mounted directory, a
  newly-added `include_dir` line, or a corrected mount path. Only `docker
compose up` (which diffs and recreates the container against current
  volume definitions) picks up mount-level changes. This was found the
  hard way during this project: an initial mount path pointed at a
  directory one level too deep, and neither a restart nor a reload
  surfaced the mismatch — only inspecting `pg_hba_file_rules` directly
  (which reports the actual file Postgres is consulting) revealed that the
  custom HBA file had never loaded at all, despite the container running
  without error.
- **`peer` authentication cannot be exercised via `docker compose exec`.**
  `peer` matches the connecting process's OS user against the requested
  database role; the official Postgres image always runs as OS user
  `postgres` regardless of which role name is created via
  `POSTGRES_USER`. A `local ... <role> peer` line is real protection for
  genuine local-socket clients, but verifying it requires a connection
  that actually originates as the matching OS user — `docker compose exec
postgres psql -U <role>` will correctly fail with "Peer authentication
  failed," which is the mechanism working as intended, not a
  misconfiguration.
- `init.sh` only ever runs during first-time `initdb` bootstrap
  (`docker-entrypoint-initdb.d/` scripts are unconditionally skipped once
  `PGDATA` is already populated). Applying this pattern to an
  already-initialized volume requires manually replicating what `init.sh`
  would have done (appending `include_dir`, then reloading) rather than
  expecting the script to run — confirmed necessary and sufficient during
  this project's own recovery from a pre-existing volume.
- The local-socket `trust`-vs-`peer` distinction only matters in the first
  place because something could plausibly connect over that socket; in
  the current all-container, no-shared-socket topology it's low-severity
  either way, but the explicit `peer` line costs nothing and closes the
  gap for any future situation where that assumption changes.
