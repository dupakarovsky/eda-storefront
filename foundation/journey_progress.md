# 🟢 Event-Driven Storefront — Foundation — Project Plan

## Project Concept

Build `foundation/`, the shared Go module every later service in the
Event-Driven Storefront system imports — env-based config, structured
logging, a reusable graceful-shutdown harness, the hand-rolled HTTP
`Router`/middleware stack, Postgres pool wiring, and NATS JetStream wiring.
No service-specific domain logic is built in this project; the deliverable
is the kit itself, proven runnable via `docker-compose` and a throwaway
smoke-test service — not `stores`, which becomes its own later project.

## Concepts We'll Cover

- Go workspaces (`go.work`) tying independent modules together for local
  cross-module development without publishing/versioning.
- `envconfig`/`.env`-based configuration, replacing the CLI-flag convention
  used in the single-binary prior projects.
- Extracting a reusable graceful-shutdown/run harness instead of inlining
  it per-service.
- NATS JetStream stream/subject conventions and a publisher, net-new to
  this series.
- `docker-compose.yml`/`Makefile` as living, incrementally-grown
  deliverables, starting here.

## Folder Structure

```txt
eda-storefront/
├── go.work
├── go.work.sum
├── docker-compose.yml
├── Makefile
├── foundation/
│ ├── journey_progress.md
│ ├── go.mod
│ ├── config/
│ ├── logging/
│ ├── shutdown/
│ ├── http/
│ ├── postgres/
│ └── nats/
└── docs/
```

## Phase Plan

**Phase 1 — Workspace Setup & Direct Ports**
Set up `go.work`, initialize the `foundation` module, and port the
`logging` and `postgres` packages in as-is (both already generic, no
`court-reservation`-specific code to strip out). By the end, `foundation`
compiles and imports cleanly from a scratch consumer.

**Phase 2 — Config**
Rebuild `cmd/api`'s `flag`-based config into `foundation/config`, backed by
`envconfig` + `.env`, per ADL 0003.

**Phase 3 — Shutdown**
Extract `court-reservation`'s inline `run()` graceful-shutdown logic out of
`main.go` into a reusable `foundation/shutdown` harness any service's own
`main.go` can call into.

**Phase 4 — HTTP Router & Middleware**
Port `Router`/`Middleware` and the `Logging`/`Recovery`/`Timeout`
middleware into `foundation/http` as-is.

**Phase 5 — NATS JetStream**
Net-new: JetStream connection setup, stream/subject conventions, and a
publish helper.

**Phase 6 — Compose, Makefile & Smoke Test**
Scaffold `docker-compose.yml` (Postgres, NATS, a log volume) and
`Makefile`; prove the whole kit end-to-end via a trivial throwaway service
that uses every `foundation` package at once.

## Phase 1 Summary — COMPLETE

**Status:** Complete

### What Was Built

Initialized the `go.work` workspace and the `foundation` Go module
(`github.com/dupakarovsky/eda-storefront/foundation`, Go 1.25.0). Ported
`logging` (the `slog`-based `multiHandler` fanning out to a text console
handler and a JSON file handler) and `postgres` (`Config`, `SSLMode`,
`NewPool`) in from `court-reservation` unchanged at the code level — only
their import paths moved. Added `pgx/v5/pgxpool` as a real dependency and
confirmed `go build ./...` succeeds. Validated the workspace wiring itself
with a throwaway `tmp-smoketest` module that imported `foundation/logging`
purely through `go.work`'s local `use` resolution, with no `go get` and no
`replace` directive — confirmed both the console and file log sinks
produce correct output, then removed the scaffold module and dropped it
from `go.work`.

### Issues Found & Resolved

None. Both ported packages were already generic (no `court-reservation`
domain coupling), so this phase was pure setup and verification — no
debugging required.

### State at End of Phase 1

`foundation/logging` and `foundation/postgres` are in place, compiling,
and proven importable across a module boundary via `go.work`. `foundation`
still has empty `shutdown/`, `http/`, `config/`, `nats/` directories
awaiting their phases. No `docker-compose.yml`, `Makefile`, or `docs/`
changes yet — those come in Phase 6 and at project close, respectively.

### Carried Into Phase 2

Phase 2 rebuilds `config` from `court-reservation`'s `flag`-based CLI
parsing into `envconfig` + `.env`, per ADL 0003. The existing `config.go`
(server/logger/db struct groups, `loadConfig`, `addr()`) is the reference
point for what fields need to survive the migration — the shape of the
`config` struct stays useful, only how it's populated changes.

## Phase 2 Summary — COMPLETE

**Status:** Complete

### What Was Built

Added `foundation/config`, replacing `court-reservation`'s flag-based
`config` struct with `Config` (exported `ServerConfig`, `LoggerConfig`,
`DBConfig` sub-structs, tagged for `envconfig`). `Load()` combines
`godotenv.Load()` (best-effort `.env` ingestion into the real environment,
non-fatal if absent) with `envconfig.Process` to populate `Config`, returning
an error on malformed values rather than crashing. `Addr()` moved from a
free function to a method on `ServerConfig`. Verified end-to-end via a
throwaway `tmp-smoketest` module: confirmed clean fallback to defaults with
no `.env` present, then confirmed `.env` overrides (`SRV_PORT`,
`LOG_TERM_LEVEL`, `DB_USER`) applied correctly once the file was placed at
the process's actual working directory.

### Issues Found & Resolved

`godotenv.Load()` resolves `.env` relative to the process's runtime working
directory, not the source package's location — `go run ./tmp-smoketest/`
still executes with the shell's CWD, so `.env` had to live at the workspace
root, not inside `tmp-smoketest/`, for it to be picked up. Not a bug; noted
as relevant again once real services run inside containers with their own
`WORKDIR` in Phase 6.

### State at End of Phase 2

`foundation/config` is complete, compiling, and proven to load from both a
real `.env` file and pure defaults. `foundation/shutdown`, `foundation/http`,
`foundation/nats` remain empty, awaiting their phases. `tmp-smoketest`
served its purpose and can be removed before Phase 3 (or repurposed —
your call).

### Carried Into Phase 3

Phase 3 extracts `court-reservation`'s inline `run()` graceful-shutdown
logic out of `main.go` into a reusable `foundation/shutdown` harness. The
existing `run(cfg, logger, handler)` function — signal channel, error
channel, `select` on shutdown vs. listener failure, timeout-bounded
`srv.Shutdown` — is the reference point for what needs to generalize into
a harness any service's `main.go` can call, not just `court-reservation`'s.
