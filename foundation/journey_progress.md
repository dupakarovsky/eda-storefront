# 🟢 Event-Driven Storefront — Foundation — Project Plan

## Project Concept

Build `foundation/`, the shared Go module every later service in the
Event-Driven Storefront system imports — env-based config, structured logging, a
reusable graceful-shutdown harness, the hand-rolled HTTP `Router`/middleware
stack, Postgres pool wiring, and NATS JetStream wiring. No service-specific
domain logic is built in this project; the deliverable is the kit itself, proven
runnable via `docker-compose` and a throwaway smoke-test service — not `stores`,
which becomes its own later project.

## Concepts We'll Cover

- Go workspaces (`go.work`) tying independent modules together for local
  cross-module development without publishing/versioning.
- `envconfig`/`.env`-based configuration, replacing the CLI-flag convention used
  in the single-binary prior projects.
- Extracting a reusable graceful-shutdown/run harness instead of inlining it
  per-service.
- NATS JetStream stream/subject conventions and a publisher, net-new to this
  series.
- `docker-compose.yml`/`Makefile` as living, incrementally-grown deliverables,
  starting here.

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

**Phase 1 — Workspace Setup & Direct Ports** Set up `go.work`, initialize the
`foundation` module, and port the `logging` and `postgres` packages in as-is
(both already generic, no `court-reservation`-specific code to strip out). By
the end, `foundation` compiles and imports cleanly from a scratch consumer.

**Phase 2 — Config** Rebuild `cmd/api`'s `flag`-based config into
`foundation/config`, backed by `envconfig` + `.env`, per ADL 0003.

**Phase 3 — Shutdown** Extract `court-reservation`'s inline `run()`
graceful-shutdown logic out of `main.go` into a reusable `foundation/shutdown`
harness any service's own `main.go` can call into.

**Phase 4 — HTTP Router & Middleware** Port `Router`/`Middleware` and the
`Logging`/`Recovery`/`Timeout` middleware into `foundation/http` as-is.

**Phase 5 — NATS JetStream** Net-new: JetStream connection setup, stream/subject
conventions, and a publish helper.

**Phase 6 — Compose, Makefile & Smoke Test** Scaffold `docker-compose.yml`
(Postgres, NATS, a log volume) and `Makefile`; prove the whole kit end-to-end
via a trivial throwaway service that uses every `foundation` package at once.

## Phase 1 Summary — COMPLETE

**Status:** Complete

### What Was Built

Initialized the `go.work` workspace and the `foundation` Go module
(`github.com/dupakarovsky/eda-storefront/foundation`, Go 1.25.0). Ported
`logging` (the `slog`-based `multiHandler` fanning out to a text console handler
and a JSON file handler) and `postgres` (`Config`, `SSLMode`, `NewPool`) in from
`court-reservation` unchanged at the code level — only their import paths moved.
Added `pgx/v5/pgxpool` as a real dependency and confirmed `go build ./...`
succeeds. Validated the workspace wiring itself with a throwaway `tmp-smoketest`
module that imported `foundation/logging` purely through `go.work`'s local `use`
resolution, with no `go get` and no `replace` directive — confirmed both the
console and file log sinks produce correct output, then removed the scaffold
module and dropped it from `go.work`.

### Issues Found & Resolved

None. Both ported packages were already generic (no `court-reservation` domain
coupling), so this phase was pure setup and verification — no debugging
required.

### State at End of Phase 1

`foundation/logging` and `foundation/postgres` are in place, compiling, and
proven importable across a module boundary via `go.work`. `foundation` still has
empty `shutdown/`, `http/`, `config/`, `nats/` directories awaiting their
phases. No `docker-compose.yml`, `Makefile`, or `docs/` changes yet — those come
in Phase 6 and at project close, respectively.

### Carried Into Phase 2

Phase 2 rebuilds `config` from `court-reservation`'s `flag`-based CLI parsing
into `envconfig` + `.env`, per ADL 0003. The existing `config.go`
(server/logger/db struct groups, `loadConfig`, `addr()`) is the reference point
for what fields need to survive the migration — the shape of the `config` struct
stays useful, only how it's populated changes.

## Phase 2 Summary — COMPLETE

**Status:** Complete

### What Was Built

Added `foundation/config`, replacing `court-reservation`'s flag-based `config`
struct with `Config` (exported `ServerConfig`, `LoggerConfig`, `DBConfig`
sub-structs, tagged for `envconfig`). `Load()` combines `godotenv.Load()`
(best-effort `.env` ingestion into the real environment, non-fatal if absent)
with `envconfig.Process` to populate `Config`, returning an error on malformed
values rather than crashing. `Addr()` moved from a free function to a method on
`ServerConfig`. Verified end-to-end via a throwaway `tmp-smoketest` module:
confirmed clean fallback to defaults with no `.env` present, then confirmed
`.env` overrides (`SRV_PORT`, `LOG_TERM_LEVEL`, `DB_USER`) applied correctly
once the file was placed at the process's actual working directory.

### Issues Found & Resolved

`godotenv.Load()` resolves `.env` relative to the process's runtime working
directory, not the source package's location — `go run ./tmp-smoketest/` still
executes with the shell's CWD, so `.env` had to live at the workspace root, not
inside `tmp-smoketest/`, for it to be picked up. Not a bug; noted as relevant
again once real services run inside containers with their own `WORKDIR` in Phase
6\.

### State at End of Phase 2

`foundation/config` is complete, compiling, and proven to load from both a real
`.env` file and pure defaults. `foundation/shutdown`, `foundation/http`,
`foundation/nats` remain empty, awaiting their phases. `tmp-smoketest` served
its purpose and can be removed before Phase 3 (or repurposed — your call).

### Carried Into Phase 3

Phase 3 extracts `court-reservation`'s inline `run()` graceful-shutdown logic
out of `main.go` into a reusable `foundation/shutdown` harness. The existing
`run(cfg, logger, handler)` function — signal channel, error channel, `select`
on shutdown vs. listener failure, timeout-bounded `srv.Shutdown` — is the
reference point for what needs to generalize into a harness any service's
`main.go` can call, not just `court-reservation`'s.

## Phase 3 Summary — COMPLETE

**Status:** Complete

### What Was Built

Added `foundation/shutdown`, exposing
`Run(logger *slog.Logger, srv *http.Server, shutdownTimeout time.Duration) error`.
Unlike `court-reservation`'s original `run(cfg, logger, handler)`, `Run` doesn't
build the `*http.Server` itself — callers construct and configure their own
server (`Addr`, `Handler`, timeouts) and hand it in already-built, since server
construction is inherently service-specific while the shutdown sequence itself
is not. Internally, `Run` ports `court-reservation`'s pattern directly:
`ListenAndServe` runs in its own goroutine against an error channel, raced in a
`select` against an `os/signal`-driven channel (SIGINT/SIGTERM), and once a
signal arrives, a fresh `shutdownTimeout`-bounded context drives
`srv.Shutdown(ctx)`, falling back to `srv.Close()` if it doesn't complete in
time. Verified end-to-end via `tmp-smoketest`: confirmed the server starts and
serves a real request via curl, confirmed `Ctrl+C` produces the full log
sequence (start → signal received → listener goroutine terminated → shutdown
complete) and a prompt, clean process exit.

### Issues Found & Resolved

None — this was a direct, working port of an already-proven pattern.

### Design Questions Deferred (Open — Revisit in Project 1)

Two questions came up during this phase that need a real service to test against
rather than a speculative answer now, per the project's no-stubbing discipline:

1. **Multi-listener shutdown.** The source book's `internal/waiter` uses
   `errgroup` + `signal.NotifyContext` to wait on multiple concurrent
   `WaitFunc`s at once (HTTP, gRPC, NATS stream), which is more granular in how
   it can express a shutdown _sequence_ (grace period, then a timed force-close)
   than the book's context-cancellation-only approach, but is only proven here
   for a single `*http.Server`. `stores` (HTTP + NATS publish only) won't
   exercise this gap; `depot`/`baskets` (Project 2, HTTP + a concurrent NATS
   subscription loop) is the first real candidate. Revisit then — possibly a
   hybrid: our explicit `Shutdown`/`Close` sequencing, generalized to run
   per-listener via something `errgroup`-shaped.
1. **A typed `Server` wrapper.** Currently `Run` takes a bare `*http.Server` and
   logs only what `*http.Server` itself exposes. Once a service is actually
   running in a container, it'd be worth wrapping it in our own type (embedding
   `*http.Server`, adding `Service()`, `Hostname()`, `ShutdownTimeout()`) so
   `Run`'s startup log line can identify which service and host it's running as.
   Deferred rather than guessed now, since the right fields are clearer once
   there's a real containerized service to shape them against.

### State at End of Phase 3

`foundation/shutdown` is complete, compiling, and proven via `tmp-smoketest`
against a real single-server lifecycle. `foundation/logging`,
`foundation/postgres`, `foundation/config`, and `foundation/shutdown` are all
done. `foundation/http` (Router/Middleware) and `foundation/nats` remain empty,
awaiting Phases 4 and 5.

### Carried Into Phase 4

Phase 4 ports `Router`/`Middleware` and the `Logging`/`Recovery`/`Timeout`
middleware into `foundation/http`, as-is — no open design questions carried
forward from this phase, since `Router` doesn't depend on anything `shutdown`
does.

## Phase 4 Summary — COMPLETE

**Status:** Complete

### What Was Built

Added `foundation/http` (package `foundationhttp`), porting
`court-reservation`'s `Router`/`Middleware` (outermost-first ordering via
`slices.Backward` in both `Handle` and `Build`), `LoggingMiddleware` (with
`responseWriterWrapper` intercepting `WriteHeader` to observe the response
status through `http.ResponseWriter`'s otherwise write-only interface),
`RecoveryMiddleware` (panic recovery, structured error log, 500 response with
`Connection: close` set before `http.Error`), and `TimeoutMiddleware` (context
deadline attachment; enforcement is opt-in per-handler via `ctx.Done()`). All
four ported as-is at the code level — only the package name changed, from
`interfaceshttp` to `foundationhttp`, specifically to avoid shadowing stdlib
`net/http` once services need both imported side by side.

Verified end-to-end via `tmp-smoketest`, exercising all three middleware at
once: `GET /ok` (200), `GET /panic` (caught by `RecoveryMiddleware`, logged,
500), `GET /slow` (2s timeout context canceled by `TimeoutMiddleware`, the
handler's own `select` on `ctx.Done()` converting that into a 504). The smoke
test's terminal log confirmed the middleware ordering itself —
`LoggingMiddleware`'s "response sent" line reported the true final status and
duration in both the panic and timeout cases, proving `Logging` wraps `Recovery`
and `Timeout` rather than the reverse.

### Issues Found & Resolved

One build error, not a code bug: `go build ./...` from the workspace root failed
with "directory prefix . does not contain modules listed in go.work or their
selected dependencies." Cause: with only one `use ./foundation` entry, the
workspace root itself isn't a recognized module, so `./...` can't expand from
`.`. Resolved by building with an explicitly-scoped pattern
(`go build ./foundation/...`) instead — expected to resolve itself naturally
once more services are added as workspace members.

### State at End of Phase 4

`foundation/logging`, `foundation/postgres`, `foundation/config`,
`foundation/shutdown`, and `foundation/http` are all complete. Only
`foundation/nats` remains, awaiting Phase 5.

### Carried Into Phase 5

Phase 5 is net-new (no direct port source from `court-reservation`, which never
used NATS): JetStream connection setup, stream/subject conventions, and a
publish helper. No open design questions carried forward from Phase 4 —
`foundationhttp` has no dependency on messaging.

## Phase 5 Summary — COMPLETE

**Status:** Complete

### What Was Built

Added `foundation/nats` (package `foundationnats`), net-new — no direct port
source from `court-reservation`, which never used NATS. `Config` (`URL`, `Name`,
`ConnectTimeout`, `MaxReconnects`, `ReconnectWait`) and `Connect` establish a
`*nats.Conn` plus a `jetstream.JetStream` context, using the newer
`nats-io/nats.go/jetstream` subpackage rather than the older `nc.JetStream()`
method, per ADL 0003's modernized-stack intent.
`Subject(service, aggregate, event)` codifies a flat three-segment subject
convention (`stores.store.created`) matching the event names already listed in
`system-state.md`. `EnsureStream` wraps `CreateOrUpdateStream` for idempotent
per-service stream provisioning (one stream per publishing service, e.g.
`STORES` claiming `stores.>`), safe to call on every service boot. `Publisher`
wraps `jetstream.JetStream` with a single `Publish` method using JetStream's
synchronous publish — the call only returns successfully once the broker has
durably acked the message — deliberately over `PublishAsync`, favoring the
simpler, safer default for a shared package. `Publisher.Publish` takes a raw
`[]byte`, leaving serialization format (JSON vs. protobuf, still an open
question per ADL 0003) to each publishing service.

Verified end-to-end against a real NATS JetStream broker (a persistent local
`nats:2.12 -js` container, port-mapped to `6969`, predating this project) via
`tmp-smoketest`: confirmed a live connection (`ConnectedUrl`, `Status`),
confirmed `EnsureStream` is genuinely idempotent by running the smoke test twice
with identical output and no error on the second `CreateOrUpdateStream` call,
and confirmed a real publish/ack round trip (`ack.Stream`, `ack.Sequence`)
against a stream provisioned for the `stores.>` subject space.

### Issues Found & Resolved

Two, both caught by the smoke test rather than by inspection. First, `Subject`'s
`fmt.Sprintf` format string had a stray trailing `.` (`"%s.%s.%s."`), producing
subjects like `stores.store.created.` with a dangling empty final token —
cosmetic on its own, but compounding the second issue below since a mismatched
trailing token also wouldn't match a stream's subject filter. Second, and the
actual blocking error: the Step 3 `tmp-smoketest` edit replaced the Step 2
`EnsureStream` call entirely rather than keeping it, so no stream on the server
claimed `stores.>` when `Publish` ran, surfacing as
`nats: no response from stream` — JetStream's synchronous publish requires a
matching stream to durably commit to, unlike core NATS fire-and-forget pub/sub.
Resolved by fixing the format string and restoring the `EnsureStream` call ahead
of the publish.

### State at End of Phase 5

`foundation/logging`, `foundation/postgres`, `foundation/config`,
`foundation/shutdown`, `foundation/http`, and `foundation/nats` are all
complete, compiling, and each independently proven via `tmp-smoketest`. Every
package planned for `foundation/` now exists. No consumer/subscribe side was
built — deliberately out of scope here, since `foundation`'s Phase 5 goal was
publish-only; consuming is the first real thing Project 2 (`depot`/`baskets`)
builds against this same `STORES` stream.

### Carried Into Phase 6

Phase 6 scaffolds the real `docker-compose.yml` (Postgres, NATS, a log volume)
and `Makefile`, then proves the whole kit end-to-end via a single throwaway
service exercising every `foundation` package at once — the first time
`foundation/postgres` and `foundation/nats` get exercised together, and the
first time the project's NATS testing moves off the predating standalone
`nats-js` container onto a compose-managed one.
