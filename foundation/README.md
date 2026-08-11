# EVENT DRIVE ARCHITECTURE STOREFRONT IN GO: FOUNDATION

## **Revised Project 1 — Foundation only:**

**Project Concept:** Build `foundation/`, the shared Go module every later
service imports — env-based config, structured logging, a reusable
graceful-shutdown harness, the hand-rolled HTTP `Router`/middleware stack,
Postgres pool wiring, and NATS JetStream wiring. No service-specific domain
logic in this project; the deliverable is the kit itself, proven runnable via
`docker-compose` and a throwaway smoke-test service, not `stores`.

## **Concepts we'll cover:**

Go workspaces (`go.work`); `envconfig`/`.env`-based configuration versus CLI
flags; extracting a reusable shutdown/run harness instead of inlining it
per-service; NATS JetStream stream/subject conventions and a publisher; and
getting `docker-compose.yml`/`Makefile` started as living deliverables.

**File structure:**

```
eda-storefront/
├── go.work
├── go.work.sum
├── docker-compose.yml
├── Makefile
├── foundation/
│   ├── journey_progress.md
│   ├── go.mod
│   ├── config/
│   ├── logging/
│   ├── shutdown/
│   ├── http/            (Router, Middleware, Logging/Recovery/Timeout)
│   ├── postgres/
│   └── nats/
└── docs/                 (already in place)
```

## **Phase plan** (unchanged from before, just dropping what was Phase 7 onward):

- Phase 1 — Workspace setup; port `logging` and `postgres` in as-is; prove the
  module compiles and imports cleanly.
- Phase 2 — `config`: rebuild from `flag`-based to `envconfig` + `.env`.
- Phase 3 — `shutdown`: extract the `run()` pattern from `court-reservation`'s
  `main.go` into a reusable harness.
- Phase 4 — `http`: port `Router`/`Middleware`/`Logging`/`Recovery`/`Timeout` in
  as-is.
- Phase 5 — `nats`: new — JetStream connection setup, stream/subject
  conventions, a publish helper.
- Phase 6 — `docker-compose.yml` + `Makefile` scaffold (Postgres, NATS, log
  volume), end-to-end smoke test of the whole kit via a trivial throwaway
  service.

One consequence worth flagging now, even though the ADL itself waits until the
end: this pushes every later project's number and directory prefix up by one —
`stores` becomes Project 2 (`02-stores/`), `depot`/`baskets` becomes Project 3
(`03-depot-baskets/`), and so on through `notifications` at Project 9. I won't
touch `roadmap.md` or `system-state.md` yet — per the working agreement, that
update belongs at the end of this project alongside the new ADL — but wanted you
aware of the ripple before we're deep into Phase 6 and it surfaces as a
surprise.

Ready for me to write `foundation/journey_progress.md` with this plan and start
Phase 1, Step 1?

### Phase 1, Step 1: workspace and module setup

This is the skeleton step: no logic yet, just the scaffolding everything else in
this phase plugs into.

From your `eda-storefront/` workspace root, run:

```bash
go work init
mkdir -p foundation/logging foundation/postgres foundation/shutdown foundation/http foundation/config foundation/nats
cd foundation
go mod init github.com/<your-module-path>/eda-storefront/foundation
cd ..
go work use ./foundation
```

Replace `<your-module-path>` with whatever you used for the prior two projects'
GitHub paths (I'm guessing `dupakarovsky` based on `court-reservation`'s import
paths in `main.go` — correct me if this project lives under a different
namespace). `go work init` creates an empty `go.work` at the root;
`go work use ./foundation` registers the new module inside it, so once other
project directories (like a future `stores`) exist as their own modules,
`go.work` will let them resolve `foundation` via a local filesystem path instead
of a published version, network-free.

Run those, then `cat go.work` and `cat foundation/go.mod` and paste me the
output so I can confirm the module path and workspace wiring landed the way we
expect before we drop the first real package in.

### Phase 1, Step 2: dropping the two already-reviewed packages in as-is

Workspace confirmed. Both `go.work` and `foundation/go.mod` look exactly as
expected — `go 1.25.0` picked up as current stable, module path matches your
existing GitHub namespace.

Step 2 — dropping the two already-reviewed packages in as-is. `logging` needs no
changes at all; `postgres` needs none either — nothing in `config.go`/`pool.go`
referenced `court-reservation` domain types, they're pure infra.

```go
// foundation/logging/logging.go
package logging

```

```go
// foundation/postgres/config.go
package postgres
```

```go
// foundation/postgres/pool.go
package postgres
```

Both packages are dropped in unmodified — only their import path changes (from
`court-reservation`'s internal path to `foundation/logging` and
`foundation/postgres`). `pgx/v5` isn't a dependency yet, so this won't compile
until it's fetched.

From `foundation/`, run:

```bash
go get github.com/jackc/pgx/v5/pgxpool
go build ./...
```

`go build ./...` with no output means both packages compile cleanly as part of
the module. Paste me the output (or the error, if `go get` or the build trips on
anything) and we'll move to Step 3 — a tiny throwaway `main.go` outside
`foundation/` that actually imports both packages across the workspace boundary,
just to prove `go.work`'s local resolution works end-to-end before we build
anything else.

### Phase 1, Step 3: a throwaway consumer

A throwaway consumer to prove `go.work` actually resolves `foundation` locally,
across module boundaries, before we build anything else on top of it. This isn't
part of the permanent structure — just scaffolding to validate the workspace
wiring, and we'll delete it once it's proven itself.

From the workspace root:

```bash
mkdir -p tmp-smoketest
cd tmp-smoketest
go mod init github.com/dupakarovsky/eda-storefront/tmp-smoketest
cd ..
go work use ./tmp-smoketest
```

Then the smoke test itself:

```go
// tmp-smoketest/main.go
package main
```

The interesting part here isn't the logging call itself — it's that
`tmp-smoketest` never ran `go get` for `foundation`, and there's no `replace`
directive anywhere. `go.work`'s `use` list is what lets the Go toolchain resolve
`github.com/dupakarovsky/eda-storefront/foundation` to the local `./foundation`
directory instead of trying to fetch it from a remote host, which is exactly the
mechanism every later service module (`stores`, `depot`, etc.) will lean on to
import `foundation` without it ever being published anywhere.

From the workspace root, run:

```bash
go run ./tmp-smoketest
```

You should see `smoke test passed` printed, plus the `slog` text-handler line on
stdout, and a new `tmp-smoketest/smoketest.log` file containing the JSON version
of the same record. Paste the output (and `cat tmp-smoketest/smoketest.log`)
and, once that's confirmed, we'll tear `tmp-smoketest` down and move to Phase 2
— rebuilding `config` around `envconfig`.

Both sinks confirmed working — the terminal handler's custom `ReplaceAttr`
cleanly compresses the source into `[main.go:16]`, and the file handler's
untouched JSON version keeps the full structured `source` object (function,
file, line) intact, which is exactly the tradeoff that handler was designed for:
compact for a human scanning the terminal, complete for a machine parsing the
file later. More importantly, the workspace resolution worked exactly as
expected — `tmp-smoketest` pulled in `foundation/logging` with zero network
calls, purely off `go.work`'s local `use` mapping.

That confirms the mechanism every later service will depend on, so let's tear
the scaffold down:

```bash
rm -rf tmp-smoketest
```

Then edit `go.work` to drop the `use ./tmp-smoketest` line — open it and remove
that line manually, or run:

```bash
go work edit -dropuse=./tmp-smoketest
cat go.work
```

Paste me `go.work`'s contents after that so I can confirm it's back down to just
`use ./foundation`, and then Phase 1 is done — I'll write up the phase summary
and append it to `journey_progress.md` before we move into Phase 2.

## Phase 2 — `config`

Here's how I'd like to break **Phase 2 — Config** into Steps:

**Step 1 — Config struct shape.** Define the `Config` struct in
`foundation/config`, carrying over the same `server`/`logger`/`db` grouping from
`court-reservation`'s `config.go`, but with `envconfig` struct tags instead of
`flag` calls.

**Step 2 — Load function.** Build `Load()` using `godotenv` (to read `.env`) +
`envconfig.Process`, replacing `loadConfig()`'s flag-parsing body, with the same
sensible defaults `court-reservation` had baked into its flags.

**Step 3 — Helper methods & scratch verification.** Port `addr()` over, then
write a tiny throwaway `main.go` that calls `Load()` against a sample `.env` and
prints the result, to prove the whole path works end-to-end.

Each Step gets its own check-in before moving to the next, per the usual loop.
Ready for me to start Step 1, or do you want to adjust the Step breakdown first?

### Phase 2, Step 1: Config struct shape

Here's the struct, carrying over `court-reservation`'s three groupings but with
`envconfig` tags instead of `flag` registrations:

```go
// foundation/config/config.go
package config
```

What's happening here: this is a straight reshape, not a rewrite of intent.
`court-reservation`'s `config.go` had one flat `config` struct with three
anonymous nested structs (`server`, `logger`, `db`); here they become named,
exported types (`ServerConfig`, `LoggerConfig`, `DBConfig`). That's not cosmetic
— `envconfig.Process` walks struct tags via reflection, and reflection needs
exported fields to read into, so the lowercase/anonymous-struct trick that
worked fine for `flag.StringVar` (which just takes a pointer) can't survive the
move to `envconfig`.

Every `flag.XxxVar(&field, "flag-name", default, usage)` call becomes a struct
tag pair: `envconfig:"ENV_VAR_NAME"` for the lookup key, `default:"value"` for
the fallback — the same defaults `court-reservation` hardcoded as flag defaults,
just relocated. One deliberate omission: I left `DB.User` and `DB.Password` with
no `default` tag, matching the original, which pulled those from `os.Getenv`
rather than a hardcoded flag default — there's no sane default for credentials,
and `envconfig` will just leave them as the zero value (empty string) if unset,
same failure mode as before.

To try this, you'll want `go get github.com/kelseyhightower/envconfig` inside
`foundation/`. Once that's added and this file is saved, `go build ./...` should
still succeed even though nothing calls `Config` yet — it's an inert type at
this point. Let me know once that's in place, or if anything about the field/tag
mapping looks off before we move to Step 2 (the `Load()` function that actually
populates this from `.env`).

### Phase 2, Step 2 — Load() function

```go
// foundation/config/config.go
```

What's happening here: `godotenv.Load()` reads a `.env` file from the working
directory and injects its key/value pairs into the real process environment via
`os.Setenv` — it's a pre-step, not a config source of its own. That's why a
missing `.env` isn't treated as fatal, just printed and ignored: in production
(containers, CI) you'd expect real environment variables to already be set with
no `.env` file present at all, and `court-reservation`'s own flag defaults had
exactly this same "works with zero external config" philosophy — `.env` is a
local-dev convenience layered on top, not a hard dependency.

`envconfig.Process("", &cfg)` is the actual population step — the empty first
argument is envconfig's "prefix" parameter (e.g. passing `"APP"` would make it
look for `APP_SRV_PORT` instead of `SRV_PORT`); we don't need one here since our
tag names are already specific enough not to collide with anything else in a
container's environment. It walks `cfg`'s fields via reflection, matching each
`envconfig` tag to an environment variable, falling back to `default` when unset
— this single call replaces the entire fifteen-line `flag.XxxVar(...)` block
from the original `loadConfig()`.

One thing to flag: `envconfig.Process` can return an error (e.g. a value that
fails to parse into its target type — a non-numeric `DB_PORT`, say), which is a
genuinely new failure mode `court-reservation`'s version didn't really have,
since `flag` mostly just crashed with its own usage message on a bad value. Here
it's a normal Go error the caller decides how to handle, which is actually a
better fit for a library used by several services rather than one CLI binary.

You'll need `go get github.com/joho/godotenv` and
`go get github.com/kelseyhightower/envconfig` (if you haven't already pulled the
second one for Step 1's tags to resolve). Once both are in `go.mod`,
`go build ./...` should still pass — `Load()` is exported now but still
unconsumed. Let me know how the build goes, or ask anything before we move to
Step 3, the `addr()` helper plus the throwaway `main.go` that actually proves
this loads from a real `.env`.

### Phase 2, Step 3: addr() helper and a scratch verification

First, the helper — a direct port, just moved onto `ServerConfig`:

```go
// foundation/config/config.go
```

Now a throwaway `main.go`, outside `foundation/` itself (same pattern as Phase
1's `tmp-smoketest` module), just to prove `Load()` actually reads a `.env` file
end-to-end:

```go
// tmp-smoketest/main.go
package main
```

And a `.env` sitting next to it, with a couple of values deliberately overridden
to distinguish "loaded from file" from "fell back to default":

```env
# tmp-smoketest/.env
SRV_PORT=9090
LOG_TERM_LEVEL=warn
DB_USER=journey_user
```

What's happening here: `Addr()` is unchanged in behavior from the original free
function — same `net.JoinHostPort` call, same IPv6-bracketing rationale from the
original comment — just relocated to a method on `ServerConfig` since that's the
type that now owns `Host`/`Port`, rather than living on the whole top-level
`config`. That's a small but real improvement: `Addr()` no longer has access to
(and can't accidentally depend on) fields it has nothing to do with, like
`DB.Password`.

The `.env` here is the actual test. `SRV_PORT` and `LOG_TERM_LEVEL` and
`DB_USER` are set explicitly, so you should see those three values reflected in
the printed `Config`, while everything else — `SRV_READ_HEADER_TIMEOUT`,
`DB_HOST`, `DB_MAX_CONNS`, etc. — should show up as whatever default we tagged
in Step 1, proving the fallback path works too, not just the override path.

To run it: create `tmp-smoketest/` at the workspace root (sibling to
`foundation/`), add it to `go.work` with `go work use ./tmp-smoketest`, run
`go mod init` inside it plus `go mod tidy` to pick up the `foundation` module
locally, then `go run .` from inside `tmp-smoketest/`. You should see the full
`Config` struct printed with those three overridden values visible, plus
`addr: :9090`. Let me know what you get.

#### error on go.work

Good catch — and good repro. That behavior is correct, not a bug in the code,
but worth understanding since it'll matter later when this runs in containers.

`godotenv.Load()` doesn't look next to the Go source file or the package being
run — it looks in the **process's current working directory**, resolved at
runtime. When you ran `go run ./tmp-smoketest/`, the compiled binary still
executes with your shell's CWD, which was the workspace root, not
`tmp-smoketest/`. `go run <path>` only tells the Go toolchain where to find the
source to build — it doesn't `cd` into that directory before running the
resulting binary. So `.env` was being searched for at `eda-storefront/.env`, not
`eda-storefront/tmp-smoketest/.env`, and moving the file confirmed exactly that.

This is actually a preview of something you'll want to keep in mind once real
services are involved: in Phase 6, each service will run inside a Docker
container with its own `WORKDIR`, and `.env` (or real env vars injected by
`docker-compose.yml`) will need to live wherever that container's working
directory is — the same "runtime CWD, not source location" rule applies there
too, just resolved by Compose instead of your shell.

Both runs otherwise behaved exactly as expected: no `.env` found → clean
fallback to every default with no crash; `.env` found → `SRV_PORT`,
`LOG_TERM_LEVEL`, and `DB_USER` overridden while everything else stayed at
default. That's the full round trip Step 3 was meant to prove.

Go ahead and remove `tmp-smoketest/` entirely (`main.go`, `.env`, `go.mod`)
along with dropping it from `go.work` via
`go work edit -dropuse=./tmp-smoketest`, same cleanup discipline as Phase 1's
scaffold module. Nothing in `foundation/` depends on it.

## Phase 3 Plan

Here's the Phase 3 plan, broken into Steps:

**Step 1 — Skeleton.** Define the `shutdown` package's public surface: a
`Run(logger *slog.Logger, srv *http.Server, shutdownTimeout time.Duration) error`
function with a stub body. Wire it into a throwaway `tmp-smoketest` module
immediately, just to confirm the import and signature compile cleanly across the
`go.work` boundary — no real logic yet.

**Step 2 — Startup and signal wiring.** Fill in the goroutine that calls
`srv.ListenAndServe()`, the error channel, the `os/signal.Notify` setup, and the
`select` that races a listener failure against a shutdown signal — ported from
`court-reservation`'s `run()`.

**Step 3 — Graceful shutdown.** Implement the timeout-bounded
`srv.Shutdown(ctx)` branch, including the force-close fallback (`srv.Close()`)
if shutdown doesn't complete in time, and correct error wrapping.

**Step 4 — Real smoke test.** Swap `tmp-smoketest`'s stub for a trivial real
`http.Server` (one handler), run it, hit it with curl, then send it a
`SIGINT`/Ctrl+C and confirm you see the graceful-shutdown log sequence rather
than an abrupt kill.

This follows the library placeholder pattern — interface first, proven
importable, then real logic filled in piece by piece, with the smoke test as the
thing that proves each step actually works.

One design question before I write Step 1's signature: `court-reservation`'s
original `run()` took a whole `cfg` struct and built the `*http.Server` itself
internally. Since `foundation` shouldn't know about any particular service's
`config` shape, I'd rather have the caller build the `*http.Server` (already
configured with its own `Addr`, `ReadHeaderTimeout`, etc.) and hand it to
`shutdown.Run` along with just the `shutdownTimeout` it needs for the `Shutdown`
call. Does that split make sense to you, or did you want it to work differently?

#### Each server will have their own http.Server instance

Exactly that. Each service's `main.go` builds its own `*http.Server` — `Addr`,
`Handler` (the `Router.Build()` output), `ReadHeaderTimeout`, `WriteTimeout`,
`IdleTimeout`, all service-specific values pulled from that service's own
config. Once that struct is built and ready, it gets handed to
`shutdown.Run(logger, srv, shutdownTimeout)`, and `shutdown.Run` owns everything
from that point on: starting the listener in a goroutine, watching for either a
listener error or an OS signal, and running the timeout-bounded graceful
shutdown sequence when a signal arrives.

The split is deliberate: `*http.Server` construction is inherently
service-specific (different services will want different timeouts, maybe
different `TLSConfig` later), so `foundation` has no business owning that. But
the shutdown _sequence_ — signal handling, the error/shutdown race,
timeout-bounded `Shutdown` with a `Close()` fallback — is identical across every
service, which is exactly the kind of thing that belongs in `foundation`, built
once.

Ready to start on Step 1 — the skeleton signature and the `tmp-smoketest`
wiring?

### Phase 3, Step 1: Skeleton

```go
// foundation/shutdown/shutdown.go
```

```go
// tmp-smoketest/main.go
package main

```

What's happening here: the `shutdown` package's only public symbol right now is
`Run`, and its signature is the whole design decision from earlier made concrete
— it takes a logger (so it can log its own lifecycle events, same convention
`foundation/logging` already established), an already-built `*http.Server` it
doesn't own the construction of, and a `shutdownTimeout` it does own, since
that's genuinely part of the shutdown _sequence_, not the server's static
config. The body is a stub — it does nothing and returns immediately, which is
enough to prove the function compiles, the package resolves through `go.work`,
and the call site in `tmp-smoketest` type-checks, before any real
signal-handling logic exists.

Go ahead and create both files, add `tmp-smoketest` as its own module in
`go.work` the same way you did for Phase 2 (`go mod init` inside
`tmp-smoketest/`, then `go work use ./tmp-smoketest`), and run
`go run ./tmp-smoketest/`. You should see it print `main exited` almost
immediately, since `Run` currently returns `nil` without blocking on anything.
Let me know what you get.

### Phase 3, Step 2: Startup and signal wiring

```go
// foundation/shutdown/shutdown.go
```

What's happening here: this is a direct port of `court-reservation`'s `run()`
shape — an error channel and a signal channel raced against each other in a
`select`, with the actual `srv.ListenAndServe()` call pushed into its own
goroutine so it can't block the `select` from ever being reached. The one thing
not yet ported is everything after the signal arrives — right now the function
just logs the signal and returns `nil`, which is deliberately incomplete; that's
Step 3's job. Note also that `Run` no longer builds the `*http.Server` itself
the way `court-reservation`'s `run(cfg, ...)` did — it just starts the one it
was handed, per the split we agreed on.

Update `tmp-smoketest/main.go`'s server to actually serve something so you have
a way to confirm it's alive — add a trivial handler, e.g.
`srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })`
before the `shutdown.Run` call. Then run `go run ./tmp-smoketest/`, hit
`curl localhost:8081` in another terminal to confirm you get `ok` back, then
send it a `Ctrl+C` and confirm you see the "starting server" and "shutdown
signal received" log lines, followed by `main exited` — but note the process
won't actually terminate cleanly yet, since there's no real `Shutdown` call. Let
me know what you see.

### Phase 3, Step 3: Graceful Shutdown

```go
// foundation/shutdown/shutdown.go
package shutdown

```

What's happening here: once a signal is caught, we build a fresh context bounded
by `shutdownTimeout` — deliberately a new `context.Background()`-derived
context, not derived from anything tied to the signal itself, since we want a
clean, fully-available timeout window starting from the moment shutdown begins.
`srv.Shutdown(ctx)` stops accepting new connections and waits for in-flight
requests to finish, up to that deadline; if it can't finish in time,
`srv.Close()` forces every connection closed immediately rather than leaving the
process hung indefinitely on a shutdown that isn't converging. This is the exact
same shape as `court-reservation`'s `run()` — the only real change is that `Run`
now owns the whole lifecycle from a handed-in `*http.Server`, rather than `main`
building the server inline around this logic.

Same test as before — run it, `curl localhost:8081`, then `Ctrl+C` — but this
time you should see a `"shutdown complete"` log line before `main exited`, and
the process should actually terminate promptly rather than hang. Let me know
what you get.

#### On creating a custom Server type

Good catch, and good instinct to defer it the same way — this is the same shape
of decision as the `errgroup`/`Waiter` question: real, useful, but easier to
design correctly once there's an actual containerized service (with a real
`Service()` name, a real `Hostname()` that matters once DNS is
container-to-container rather than `localhost`) to shape it against, rather than
guessing the right fields now from a throwaway smoke test. I'll fold it into the
same Phase 3 summary note as the multi-listener deferral, so both are sitting
together as open questions for Project 1 (`stores`) to revisit.

The output otherwise confirms the full lifecycle works end to end — signal
caught, `ListenAndServe`'s goroutine unblocks and logs its own termination once
`Shutdown` closes the listener, and `"shutdown complete"` fires only after
`Shutdown` genuinely finishes, all before `main exited`. That's the
graceful-shutdown sequence doing exactly what it's supposed to.

That also folds in what was going to be Step 4 — swapping in a real server,
`curl`-ing it, and confirming the signal-to-clean-exit sequence — since you've
already done exactly that here with your added logging making the sequence even
clearer than the plan called for. So functionally, **Phase 3 is complete**:
`foundation/shutdown` has a working `Run(logger, srv, shutdownTimeout)` that any
later service's `main.go` can call.

Two things left before we close it out:

1. Update `journey_progress.md` with the Phase 3 summary — what was built, the
   deferred `errgroup`/`Waiter` multi-listener question, and the deferred
   `Server` wrapper-type idea, both flagged for reconsideration once `stores`
   (or `depot`/`baskets`) gives us something real to test against.
1. Clean up `tmp-smoketest` the same way you did after Phase 2, or repurpose it
   for Phase 4.

Want me to draft the Phase 3 summary text now for you to drop into
`journey_progress.md`?

## Phase 4 Plan

Here's how I'd break Phase 4 into Steps:

**Step 1 — `Router` core.** Port the `Middleware` type, `Router` struct,
`NewRouter`, `Use`, `Handle`, and `Build` into `foundation/http/router.go`. This
is the routing/middleware-chaining skeleton with no concrete middleware yet.

**Step 2 — `LoggingMiddleware`.** Port `logging.go` (the `responseWriterWrapper`
and `LoggingMiddleware`) into `foundation/http/logging.go`.

**Step 3 — `RecoveryMiddleware`.** Port `recovery.go` into
`foundation/http/recovery.go`.

**Step 4 — `TimeoutMiddleware`.** Port `timeout.go` into
`foundation/http/timeout.go`.

**Step 5 — Smoke test.** Wire all four together in `tmp-smoketest`, register a
trivial route, and confirm the middleware chain (log → recover → timeout →
handler) behaves as expected — including deliberately triggering a panic and a
timeout to see `RecoveryMiddleware` and `TimeoutMiddleware` actually do
something, not just compile.

Since this code is a near-verbatim port (only the package name `interfaceshttp`
and import paths change — no `court-reservation`-specific coupling to strip,
same as Phase 1), the phase should move quickly, leaving more room to discuss
the _why_ behind the design in the check-ins rather than debugging.

### Phase 4, Step 1: `Router` core

Here's `foundation/http/router.go`:

```go
// foundation/http/router.go
```

What's happening here: this is a straight, unmodified port from
`court-reservation`'s `interfaceshttp` package, just landing in `foundation`'s
own `http` package instead. One thing worth re-flagging since it matters even
more now that this is shared code used by every future service: the
outermost-first ordering convention. `Use(A, B)` makes `A` the outermost wrapper
— the first thing a request hits and the last thing a response passes through on
the way out — and `Handle`'s per-route `mw` follows the identical rule via the
same `slices.Backward` trick in both `Handle` and `Build`. Keeping `Handle` and
`Build` symmetric like this means a service author only has to learn the
ordering rule once, whether they're stacking global middleware or a one-off
route-specific wrapper.

One naming note: the package is just `http` here (shadowing stdlib `net/http` by
name, disambiguated by import alias at the call site) rather than
`interfaceshttp` — did you want to keep `court-reservation`'s `interfaceshttp`
naming for consistency, or are you fine with services importing this as
`foundationhttp "github.com/.../foundation/http"` or similar? Worth deciding now
since every later service's `main.go` will reference this import.

Only the `package` line changed — the file still lives at
`foundation/http/router.go`, since Go package names and directory names don't
have to match, only the declared `package` identifier matters for imports. A
consuming service will pull it in as something like
`foundationhttp "github.com/dupakarovsky/eda-storefront/foundation/http"`.

Go ahead and create that file and run `go build ./...` from the workspace root
to confirm it compiles clean. Let me know how it goes, or if you have questions
on the `Router` design before we move to Step 2 (`LoggingMiddleware`).

#### go.work resolution from root directory

`GOWORK` is correctly resolving to your workspace file, so it's not an
environment problem — this is actually expected Go workspace behavior, not a
bug. The workspace root itself (`.`) isn't a module; `go.work`'s
`use ./foundation` means only the `foundation/` directory is a workspace module.
`./...` expanded from the root asks Go to find packages starting at `.`, but `.`
isn't inside any module the workspace knows about — hence "directory prefix .
does not contain modules listed in go.work."

Try this instead, from the workspace root:

```bash
go build ./foundation/...
```

That pattern explicitly scopes into the module workspace actually owns. Once we
add more services later (`01-stores`, etc.), each with its own `use` line, this
same pattern generalizes to `go build ./...` working correctly from root —
because at that point `.` genuinely does contain workspace-listed module
directories as immediate children in a way Go's pattern matching resolves. Right
now, with only one `use` entry, building from inside that module (like you did a
moment ago) or scoping the pattern like above are the two working options.

Give `go build ./foundation/...` a run from the root and confirm it's clean.

### Phase 4, Step 2: LoggingMiddleware

Here's `foundation/http/logging.go`:

```go
// foundation/http/logging.go
```

What's happening here: another as-is port, same package. The one thing worth
re-anchoring, since it's easy to forget once this is buried in a shared package
used everywhere — `responseWriterWrapper` exists purely because
`http.ResponseWriter` is write-only by design; there's no `GetStatusCode()` on
the interface. The wrapper embeds the real `ResponseWriter` (so every other
method — `Write`, `Header` — passes through untouched) and only intercepts
`WriteHeader` to capture the code as a side effect, defaulting to `200` since
`WriteHeader` is never called explicitly for the common success path. This
pattern — wrap-and-intercept-one-method — is one you'll likely reuse anywhere
you need to observe something an interface doesn't expose directly.

Go ahead and build again with `go build ./foundation/...`. It should stay clean
since `LoggingMiddleware` only depends on stdlib. Any questions before Step 3
(`RecoveryMiddleware`)?

### Phase4, Step 3: `RecoveryMiddleware`

Here's `foundation/http/recovery.go`:

```go
// foundation/http/recovery.go
package foundationhttp
```

What's happening here: same as-is port, and it's the first middleware in this
batch that actually returns the named `Middleware` type from Step 1 rather than
the raw `func(http.Handler) http.Handler` signature — cosmetically different,
functionally identical, since `Middleware` is just a defined type over that
exact signature. The mechanics worth re-anchoring: `recover()` only does
anything meaningful inside a deferred function, and only catches a panic on the
same goroutine it's deferred on — which is exactly why this has to wrap
`next.ServeHTTP` directly rather than being, say, a background watcher. The
`Connection: close` header is set before `http.Error` deliberately, since
`WriteHeader` (which `http.Error` calls internally) freezes the header map — any
`Set` call after that point is silently dropped, not an error, which makes it
exactly the kind of ordering bug that's invisible until you go looking for why a
header didn't take effect.

Build again with `go build ./foundation/...` and confirm clean. Any questions
before Step 4 (`TimeoutMiddleware`)?

### Phase 4, Step 4: `TimeoutMiddleware`

```go
// foundation/http/timeout.go
package foundationhttp

```

What's happening here: last as-is port for this phase, and it's the one whose
_position_ in the stack matters most, so it's worth re-anchoring rather than
skipping past. `TimeoutMiddleware` doesn't itself enforce anything — it only
attaches a deadline to the request's `context.Context`. Enforcement is opt-in:
it happens wherever downstream code actually checks `ctx.Done()` or passes `ctx`
into something that respects it (a database call via `pgxpool`, for instance).
That's exactly why `court-reservation` placed it innermost, closest to the
handler: `Recovery` needs to still be able to catch a panic that happens while a
timeout-bounded operation is unwinding, and `Logging` needs to observe the
request's true final status even if it timed out — both need to wrap `Timeout`,
not sit inside it. We'll see this ordering made concrete in the smoke test's
`Use()` call.

Build once more with `go build ./foundation/...`, then let me know if you're
ready for Step 5 — wiring all four into `tmp-smoketest` and actually triggering
a panic and a timeout to watch them work.

### Phase 4, Step 5: Smoke Test

Here's the Step 5 smoke test. First, if you removed `tmp-smoketest` after Phase
2/3 like the journal notes suggested you might, recreate it and register it in
the workspace:

```bash
mkdir -p tmp-smoketest
cd tmp-smoketest
go mod init github.com/dupakarovsky/eda-storefront/tmp-smoketest
cd ..
go work use ./tmp-smoketest
```

Then create `tmp-smoketest/main.go`:

```go
// tmp-smoketest/main.go
```

Notice `/slow` explicitly selects on `r.Context().Done()` — this is the opt-in
behavior Step 4 flagged. `TimeoutMiddleware` cancels the context after 2
seconds, but it's this handler's own `select` that turns that cancellation into
an actual `504` response. If the handler didn't check `ctx.Done()` at all, the
middleware's timeout would silently do nothing, and the handler would just run
to completion 5 seconds later.

Run it with `go run ./tmp-smoketest`, then from another terminal try these three
in order and tell me what you see (status code + body) for each, plus what gets
logged in the server's terminal:

```bash
curl -i http://localhost:8080/ok
curl -i http://localhost:8080/panic
curl -i http://localhost:8080/slow
```

**Phase 4 complete — HTTP Router & Middleware**

We built `foundation/http` (package `foundationhttp`, renamed from
`court-reservation`'s `interfaceshttp` to avoid shadowing stdlib `net/http`):
`Router`/`Middleware` with outermost-first ordering, `LoggingMiddleware` (using
the wrap-and-intercept-one-method pattern to observe status codes through a
write-only interface), `RecoveryMiddleware` (panic → structured 500, with the
`Connection: close` header-before-`WriteHeader` ordering constraint), and
`TimeoutMiddleware` (context deadline attachment — enforcement is opt-in,
downstream code has to actually check `ctx.Done()`). All four are direct as-is
ports, same as `logging` and `postgres` in Phase 1 — no
`court-reservation`-specific coupling to strip out, so the phase stayed focused
on re-anchoring the _why_ behind each rather than debugging new code. Verified
end-to-end via `tmp-smoketest`: `/ok` (plain 200), `/panic` (caught, logged,
500, connection closed), and `/slow` (timeout context canceled at 2s, handler's
own `select` on `ctx.Done()` turned that into a 504) — with the smoke test's
terminal log confirming the middleware ordering itself, not just the individual
pieces.

One deliberate deviation worth recording: the package name changed from
`interfaceshttp` to `foundationhttp`, anticipating that services built later
will need both this package and stdlib `net/http` in the same file.

`foundation/logging`, `foundation/postgres`, `foundation/config`,
`foundation/shutdown`, and now `foundation/http` are all complete. Only
`foundation/nats` remains, plus the `docker-compose.yml`/`Makefile`/final smoke
test in Phase 6.

Here's the block to append to `journey_progress.md` (right after the Phase 3
section, before "Carried Into Phase 4" gets replaced):

## Phase 5 Plan

**Step 1 — Connection & JetStream context.** A `foundation/nats` package with a
`Config` (URL, connection name, timeouts — same "package knows nothing about CLI
flags/env" separation `postgres.Config` uses) and a `Connect` function that
dials `*nats.Conn` and returns a `jetstream.JetStream` context. I'd use the
newer `github.com/nats-io/nats.go/jetstream` package rather than the older
`nc.JetStream()` API, since that's the current idiomatic way to talk to
JetStream and fits ADL 0003's "modernized stack" intent.

**Step 2 — Stream & subject conventions.** Decide and codify the subject naming
scheme (e.g. `<service>.<aggregate>.<event>`, matching the event names already
listed in `system-state.md` like `StoreCreated`) and a small idempotent
`EnsureStream` helper (create-or-update) — since every publishing service will
want "the stream exists with the right subjects" to be safe to call on every
boot, not just once.

**Step 3 — Publish helper.** A thin `Publisher` wrapping the
`jetstream.JetStream` context with a `Publish(ctx, subject, payload)` method,
handling the ack/error path from JetStream's synchronous publish.

One thing to flag before Step 1: unlike the previous phases, this one needs a
real NATS server to actually connect to and prove the code works against —
`docker-compose.yml` doesn't exist yet (that's Phase 6). Do you already have a
NATS container running locally, or do you want a quick one-off `docker run`
command to stand one up just for these smoke tests, which we'll fold into the
real `docker-compose.yml` in Phase 6?

Let me know if this Step breakdown looks right, or if you'd want stream/subject
conventions decided before or differently — then we'll start on Step 1.

### Phase 5, Step 1: Connection & JetStream context

Good, that container's fine as-is — `-js` flag means JetStream is already
enabled, and the port mapping (`6969:4222`) is just a connection detail we'll
put in config, nothing to change on the container itself. Just make sure it's
running when you get to the smoke test below.

Let's do Step 1 — connection & JetStream context.

One naming note up front, consistent with the call we made in Phase 4: the
official client package is called `nats` (`github.com/nats-io/nats.go`), so our
own package can't also be named `nats` without forcing every consumer to alias
one of them. Same fix as `interfaceshttp` → `foundationhttp`: this package will
be `foundationnats`.

```go
// foundation/nats/config.go
package foundationnats
```

```go
// foundation/nats/connect.go
package foundationnats
```

What's happening here:

`Connect` returns two things: the raw `*nats.Conn` (the actual socket/protocol
connection — you'll want this later for lifecycle management, e.g. `nc.Close()`
in shutdown) and a `jetstream.JetStream` value, which is a _context_ for
JetStream operations (streams, consumers, publishing) built on top of that
connection, not a separate connection. This mirrors the split you'd expect:
`nats.Connect` gives you the transport, `jetstream.New` gives you the
higher-level API layered on it. Note this is the newer
`nats-io/nats.go/jetstream` subpackage, not the older `nc.JetStream()` method on
`*nats.Conn` — the new one has a cleaner, context-aware API (`ctx` params
throughout) and is the currently recommended path, which fits ADL 0003's
"modernized stack" framing.

The `if cfg.X > 0` guards follow the exact same convention `pool.go` already
established for `MaxConns`/`MinConns` — only override the client library's own
sane defaults when the caller actually supplied something. One nuance worth
flagging honestly: this pattern is slightly imprecise for `MaxReconnects`
specifically, because `0` is a _meaningful_ value in nats.go (it means "don't
reconnect at all," distinct from `-1` which means "reconnect forever"). With
`!= 0` as the guard, a caller who deliberately wants `0` can't express that —
it'll look identical to "unset" and fall through to nats.go's own default (60).
For `MaxConns` in `pool.go` this ambiguity doesn't exist (0 max connections is
never a real intention), but here it's a real, if minor, limitation. I'm leaving
it as-is for now since no caller needs "disable reconnects" yet, but it's the
kind of thing worth remembering if that ever comes up — the fix would be a
`*int` or a documented sentinel like `-1` for "explicitly disabled."

To try this out, create a throwaway `tmp-smoketest/main.go` at the workspace
root (same pattern as prior phases) that imports `foundation/nats` and calls
`Connect` with `Config{URL: "nats://localhost:6969", Name: "tmp-smoketest"}`,
then prints `nc.ConnectedUrl()` and `nc.Status()` to confirm you actually got a
live connection, not just a nil-error return. Something like:

```go
// tmp-smoketest/main.go
package main
```

Make sure `nats-js` is running, add `tmp-smoketest` to `go.work`, and run it
with `go run ./tmp-smoketest`. You should see the connected URL echoed back and
`status: CONNECTED`. Let me know what you get, or paste the error if something
breaks.

### Phase 5, Step 2: Stream and subject conventions

**Subject convention.** I'll go with `<service>.<aggregate>.<event>` — e.g.
`stores.store.created`, `stores.product.added` — matching the event names
already listed in `system-state.md`. This is a flat, predictable three-segment
shape any consumer can pattern-match against (`stores.>` for "everything
`stores` publishes", `stores.product.>` for "just product events").

**Stream convention.** One JetStream stream per publishing service (e.g. a
`STORES` stream owns every `stores.>` subject), rather than one giant shared
stream or one stream per event type. This keeps the "who owns this stream"
question trivially answerable — it maps 1:1 to the service that publishes into
it — and matches ADL 0002's decision that publishing stays per-service even
though the aggregate's own persistence doesn't use event sourcing.

```go
// foundation/nats/subject.go
package foundationnats
```

```go
// foundation/nats/stream.go
package foundationnats

```

What's happening here:

`Subject` is deliberately trivial — just string formatting — but having it as a
named function rather than every service hand-rolling `fmt.Sprintf` inline means
the three-segment convention lives in one place. If we ever needed to change the
shape (say, add a version segment), it's one function, not a grep-and-replace
across eight services.

`EnsureStream` leans on `CreateOrUpdateStream`, which the `jetstream` package
gives us specifically so callers don't have to hand-roll "try create, if
AlreadyExists then update instead" themselves — it's idempotent by construction,
safe to call on every service boot the same way `golang-migrate` running its
migration step is. That matters because a service shouldn't care whether this is
its first-ever boot (stream doesn't exist) or its hundredth (stream exists,
maybe with subjects that changed since); the call site looks identical either
way.

One thing I deliberately left at zero-value defaults for now: `StreamConfig` has
real knobs beyond `Name`/`Subjects` — retention policy, max age, storage type
(file vs. memory). JetStream's own defaults (file storage, limits-based
retention, keep-forever-within-limits) are reasonable and match ADL 0003's
"durable, replayable streams" intent, so I didn't hardcode anything more
opinionated here. If a specific service later needs different retention (say,
`notifications` wanting a short-lived working queue vs. `stores` wanting long
retention for replay), that's a per-service `StreamConfig` decision to make
explicitly then, not something to guess system-wide right now.

To test, add this to `tmp-smoketest/main.go` after the existing connect code:

```go
// tmp-smoketest/main.go
```

You'll need to add `"context"` to the import block. Run it with
`go run ./tmp-smoketest`, you should see
`stream: TEST_STREAM subjects: [test.>]` and
`sample subject: stores.store.created`. Then run it a second time — same output,
no error — to confirm `CreateOrUpdateStream` really is idempotent rather than
failing on the second run. Let me know what you get.

### Phase 5, Step 3: Publish helper

Idempotency confirmed — same stream, no error on the second call. That closes
out Step 2.

Now Step 3 — the publish helper.

```go
// foundation/nats/publish.go
package foundationnats

```

What's happening here:

`Publisher` is a thin wrapper around a `jetstream.JetStream` context — one
field, one method. Worth asking directly: why wrap it at all, if `js.Publish`
already does the work? Two reasons. First, every service's own code should
depend on _our_ `Publisher` type, not reach for `jetstream.JetStream` directly —
same reasoning as `Router` wrapping `http.ServeMux` in Phase 4: it gives us one
seam to add cross-cutting behavior later (retries, metrics, a consistent
error-wrapping convention) without touching every call site. Second,
`%w`-wrapping the error here means a caller three layers up (an application
service's use case, say) gets a message like
`publishing to "stores.store.created": ...` instead of a bare NATS error with no
idea which publish call failed — the same "make failures traceable, not just
returned" instinct behind `pool.go`'s wrapped errors.

Note this uses `js.Publish`, JetStream's _synchronous_ publish — it blocks until
the broker acks the message (or the context deadline hits), returning a
`*jetstream.PubAck` with the stream name and the sequence number it was
assigned. That's a deliberate choice over `PublishAsync`: synchronous publish
means "the function returned successfully" already implies "JetStream durably
has this message," which is the simpler, safer default for a shared foundation
package. If a specific service later needs the throughput of fire-and-forget
async publishing with batched ack handling, that's an opt-in decision for that
service, not something to default to system-wide.

One thing deliberately _not_ here yet: `Publish` takes a raw `[]byte`.
Serialization (JSON vs. protobuf, given ADL 0003 leaves `.proto` tooling as an
open question) is a decision for the calling service's event-publishing code,
not for this generic package — `foundation/nats` shouldn't need to know or care
what's inside the payload.

To test, replace the `EnsureStream`/`Subject` block in `tmp-smoketest/main.go`
with:

```go
// tmp-smoketest/main.go
```

You'll still need the `EnsureStream` call before this (JetStream will reject a
publish to a subject no stream is configured to accept — that's expected and
worth seeing once if you want to comment it out and confirm the error). With the
stream still in place from Step 2, run it and you should see something like
`published to: TEST_STREAM seq: 1` (or a higher sequence number if you're
running it more than once). Let me know what you get.

#### error on stream

Good catch on catching this via the smoke test — but there are actually two
things going on here, and it's worth pulling them apart.

**The main one, causing the error:** `no response from stream` is JetStream's
way of saying "I published this message, and waited, but no stream on the server
is configured to accept a message on this subject." JetStream's synchronous
publish doesn't just fire into the void — the broker actually looks for a stream
whose subject filter matches, and if none exists, the publish call times out
with exactly this error rather than silently succeeding. When you swapped the
`tmp-smoketest` block over for Step 3, the `EnsureStream` call from Step 2 came
out along with it — so as far as the NATS server is concerned, there's still
only `TEST_STREAM`, which only claims `test.>`. Nothing on the server currently
claims `stores.>`. Question for you: given that, what would you add back (or add
new) to `main.go` so a stream actually exists to catch this subject before the
publish call runs?

That still leaves the main issue: no stream on the server currently claims the
`stores.>` subject space. Add the `EnsureStream` call back in before the
publish, something like:

```go
// tmp-storefront/main.go
```

placed after `ctx := context.Background()` and before the
`pub := foundationnats.NewPublisher(js)` line. Give it a run with both fixes in
and let me know what comes back.

## Phase 6 Plan

You're right to flag this, and your assumption is correct — the same principle
applies to NATS JetStream, for the same underlying reason it applies to
Postgres.

By default, a NATS container's filesystem is exactly as ephemeral as Postgres's.
Core NATS (fire-and-forget, no JetStream) never persisted anything to begin with
— a subscriber had to be connected at publish time or it simply missed the
message, full stop. That's actually one of the reasons ADL 0003 chose JetStream
over core NATS: JetStream streams have a configurable storage type, and ours
(`EnsureStream`'s `StreamConfig`) doesn't currently set `Storage` explicitly,
which means it's defaulting to `FileStorage` rather than `MemoryStorage`. File
storage writes each stream's messages to disk inside the container — but "inside
the container" is the trap. Without a bind mount or named volume backing that
path, "on disk" still means the container's writable layer, which
`docker compose down` (or a crash, or an image rebuild) wipes out exactly like
it would wipe an un-volumed Postgres data directory.

So the fix is structurally identical for both services: give NATS a named volume
mapped to its JetStream storage directory (typically `/data`, via the
`-sd /data` flag or `jetstream { store_dir: /data }` in a config file), the same
way Postgres gets a volume mapped to `/var/lib/postgresql/data`.

This matters more here than it might in a typical toy setup, for a reason
specific to your roadmap: Project 2 (`depot`/`baskets`) is the first real
_consumer_ of the `STORES` stream, and JetStream's whole value proposition for
that use case is replay — a consumer that starts up late (or restarts) can catch
up on everything it missed, because the broker held onto it. If the stream's
storage isn't actually durable across a container restart, you don't have that
property yet — you'd have core NATS's fire-and-forget behavior wearing a
JetStream costume, and wouldn't find out until Project 2 tried to rely on it and
got nothing.

Given that, I'd fold this into the Phase 6 plan rather than treat it as an
afterthought — here's the revised step list:

**Step 1 — `docker-compose.yml`** Postgres and NATS (JetStream, with an explicit
`store_dir`), each with a named volume, plus the log volume.

**Step 2 — `Makefile`** `up`, `down`, `logs`, `run` targets.

**Step 3 — Smoke-test service skeleton** Wires `config`, `logging`, `postgres`,
`nats` together.

**Step 4 — Wire in HTTP + shutdown + NATS publish** Full six-package smoke test.

**Step 5 — Durability verification (new)** Publish a message and write a row,
`docker compose down` (not just stop the process — the container itself), bring
the stack back up, and confirm both the Postgres row and the JetStream message
are still there via a fresh consumer read. This is the step that actually proves
the volumes work, not just that they're declared.

**Step 6 — Full end-to-end verification** Curl, graceful shutdown via Ctrl+C,
the rest of what we discussed.

One thing worth deciding now rather than mid-step: do you want `EnsureStream` to
keep leaving `Storage` unset (defaulting to file) with the explicit expectation
that compose always supplies a volume, or would you rather make
`StreamConfig.Storage: jetstream.FileStorage` explicit in the code itself so
it's not an implicit default someone could accidentally lose track of later? I
lean toward making it explicit — it costs one line and turns "durable by default
configuration" into "durable by visible declaration," which fits the project's
general discipline of not leaving important behavior to an unstated default (the
same spirit as `pool.go`'s comment about only overriding `pgxpool`'s defaults
when a value is actually supplied).

Let me know if this revised step plan works, and your call on the `Storage`
field question, and we'll start on Step 1.

## PHASE 6 Plan

By default, a NATS container's filesystem is exactly as ephemeral as Postgres's.
Core NATS (fire-and-forget, no JetStream) never persisted anything to begin with
— a subscriber had to be connected at publish time or it simply missed the
message, full stop. That's actually one of the reasons ADL 0003 chose JetStream
over core NATS: JetStream streams have a configurable storage type, and ours
(`EnsureStream`'s `StreamConfig`) doesn't currently set `Storage` explicitly,
which means it's defaulting to `FileStorage` rather than `MemoryStorage`. File
storage writes each stream's messages to disk inside the container — but "inside
the container" is the trap. Without a bind mount or named volume backing that
path, "on disk" still means the container's writable layer, which
`docker compose down` (or a crash, or an image rebuild) wipes out exactly like
it would wipe an un-volumed Postgres data directory.

So the fix is structurally identical for both services: give NATS a named volume
mapped to its JetStream storage directory (typically `/data`, via the
`-sd /data` flag or `jetstream { store_dir: /data }` in a config file), the same
way Postgres gets a volume mapped to `/var/lib/postgresql/data`.

This matters more here than it might in a typical toy setup, for a reason
specific to your roadmap: Project 2 (`depot`/`baskets`) is the first real
_consumer_ of the `STORES` stream, and JetStream's whole value proposition for
that use case is replay — a consumer that starts up late (or restarts) can catch
up on everything it missed, because the broker held onto it. If the stream's
storage isn't actually durable across a container restart, you don't have that
property yet — you'd have core NATS's fire-and-forget behavior wearing a
JetStream costume, and wouldn't find out until Project 2 tried to rely on it and
got nothing.

Given that, I'd fold this into the Phase 6 plan rather than treat it as an
afterthought — here's the revised step list:

**Step 1 — `docker-compose.yml`** Postgres and NATS (JetStream, with an explicit
`store_dir`), each with a named volume, plus the log volume.

**Step 2 — `Makefile`** `up`, `down`, `logs`, `run` targets.

**Step 3 — Smoke-test service skeleton** Wires `config`, `logging`, `postgres`,
`nats` together.

**Step 4 — Wire in HTTP + shutdown + NATS publish** Full six-package smoke test.

**Step 5 — Durability verification (new)** Publish a message and write a row,
`docker compose down` (not just stop the process — the container itself), bring
the stack back up, and confirm both the Postgres row and the JetStream message
are still there via a fresh consumer read. This is the step that actually proves
the volumes work, not just that they're declared.

**Step 6 — Full end-to-end verification** Curl, graceful shutdown via Ctrl+C,
the rest of what we discussed.

One thing worth deciding now rather than mid-step: do you want `EnsureStream` to
keep leaving `Storage` unset (defaulting to file) with the explicit expectation
that compose always supplies a volume, or would you rather make
`StreamConfig.Storage: jetstream.FileStorage` explicit in the code itself so
it's not an implicit default someone could accidentally lose track of later? I
lean toward making it explicit — it costs one line and turns "durable by default
configuration" into "durable by visible declaration," which fits the project's
general discipline of not leaving important behavior to an unstated default (the
same spirit as `pool.go`'s comment about only overriding `pgxpool`'s defaults
when a value is actually supplied).

Let me know if this revised step plan works, and your call on the `Storage`
field question, and we'll start on Step 1.

### Step 1

Agreed — let's fold that into Step 1 since the volume mount and the explicit
`Storage` field are two halves of the same guarantee.

**First, the `EnsureStream` update:**

```go
// foundation/nats/stream.go
```

That's the only change — `Storage: jetstream.FileStorage` where nothing was set
before. `jetstream.StreamConfig` also has a `MemoryStorage` option, which is
legitimate for some use cases (a scratch stream you're fine losing), but for
`STORES` — the stream Project 2's `depot`/`baskets` will depend on for catch-up
replay — file storage backed by a real volume is the only choice that matches
the durability guarantee JetStream is supposed to be giving you.

**Now `docker-compose.yml`:**

```yaml
# docker-compose.yml
```

Your instincts on all of these are good, and yes — your read on the env vars is
exactly right. `POSTGRES_USER`/`POSTGRES_PASSWORD` set the identity and password
of the initial superuser role created at cluster init time, and `POSTGRES_DB`
names the default database created alongside it (if omitted, it defaults to the
same value as `POSTGRES_USER`, not literally `postgres`, but close enough to the
same problem). All three only take effect on a _fresh_ volume — if `pg-data`
already has data in it from a previous run, changing these in `.postgres.env`
later won't retroactively change the role's password, since `initdb` only runs
once. Worth remembering the first time you go to "fix" a credential and nothing
happens — you'd need to drop the volume and reinit, not just edit the env file.
And yes, `.postgres.env` (git-ignored) instead of inline `environment:` in the
committed compose file is the right call — that's the actual point of `env_file`
existing as a separate mechanism rather than just sugar.

Two small things to flag on what you've got so far, then I'll get into the
SCRAM/`pg_hba` question.

`networks.eda-net.external: true` means compose expects that network to already
exist and will error out
(`network eda-net declared as external, but could not be found`) if it doesn't.
You mentioned you already created it manually — that's fine for you personally,
but it means the Makefile's `up` target (Step 2) needs a
`docker network create eda-net || true` (or an idempotent equivalent) ahead of
`docker compose up`, or the compose file becomes unrunnable for anyone who
hasn't run that command by hand first. I'll fold that into Step 2 rather than
now, just flagging it so it doesn't get lost.

On `.gitignore`: `.postgres.env` needs to actually be listed there before
anything gets committed, otherwise the exact leak you're trying to avoid happens
on the first `git add .`. Worth adding a `.postgres.env.example` (committed,
with placeholder values) alongside it, so the repo documents what variables are
expected without exposing real ones — the convention most projects use for
exactly this reason.

**Now, SCRAM and `pg_hba` — you're pointing at something real, but it's worth
being precise about what's already true by default versus what still needs an
explicit decision.**

Password _encryption_ (how the password is hashed and stored server-side, and
how the challenge-response handshake works) has defaulted to `scram-sha-256`
since Postgres 10, and it's been the _default_ default (not just available)
since Postgres 14 — so on `postgres:17`, `password_encryption = scram-sha256` is
already what you get with zero configuration. You don't need to force this
explicitly; it would be redundant, not wrong.

`pg_hba.conf` is the separate, more interesting piece — it's not about how the
password is hashed, it's about _which authentication method is required at all,
for connections arriving from where_. The official Postgres image's entrypoint
script auto-generates a `pg_hba.conf` on first init, and its default behavior is
worth knowing precisely rather than assuming: it authorizes connections over the
network (`host` lines) using `scram-sha-256`, but authorizes connections over
the local Unix socket (`local` lines) using `trust` — no password required at
all — _unless_ you explicitly set the `POSTGRES_HOST_AUTH_METHOD` environment
variable to override that. Given that our Postgres only lives inside a container
reachable by `depot`/`baskets`/etc. over TCP, not by arbitrary local shell users
on a socket, that default `trust`-for-local behavior is arguably harmless in
this specific setup — but "arguably harmless" is exactly the kind of default
this project's discipline (per `pool.go`'s comment about not leaving pool tuning
to an unstated default) says we shouldn't leave implicit if we can make it
explicit cheaply. Setting `POSTGRES_HOST_AUTH_METHOD=scram-sha-256` in
`.postgres.env` closes that gap for one line of cost.

So, concretely, here's what I'd add to `.postgres.env`:

```
# .postgres.env
POSTGRES_USER=edasuper
POSTGRES_PASSWORD=supersecure
POSTGRES_DB=edadb
POSTGRES_HOST_AUTH_METHOD=scram-sha-256
```

Nothing changes in `docker-compose.yml` itself for this — it's purely an env var
the entrypoint script reads at first init to decide how to write `pg_hba.conf`.
If `pg-data` already exists from an earlier run before this variable was added,
it won't retroactively rewrite `pg_hba.conf` either, for the same reason
mentioned above — worth a clean-volume test run if you've already brought
Postgres up once.

One question before we lock this in: since this variable only takes effect on
first init, do you want to test this now with a fresh `pg-data` volume (i.e.,
confirm you haven't run `docker compose up` yet, or are willing to
`docker compose down -v` first), or would you rather note it and verify it
specifically as part of Step 5's durability test, where you're already tearing
the stack down and back up anyway?

Here's Step 1 finalized with everything settled so far: your additions,
`eda-net` created by the Makefile (Step 2) rather than externally, and
`POSTGRES_HOST_AUTH_METHOD` added to close the `local`-socket `trust` gap
without the fuller hardening pattern.

```yaml
# docker-compose.yml

services:
    postgres:
        image: postgres:17
        container_name: eda-postgres
        restart: unless-stopped
        env_file:
            - ./.postgres.env
        ports:
            - 127.0.0.1:5433:5432
        volumes:
            - pg-data:/var/lib/postgresql/data
        networks:
            - eda-net

    nats:
        image: nats:2.12
        container_name: eda-nats
        restart: unless-stopped
        command: '--jetstream --http_port 8222 --store_dir /data'
        ports:
            - 127.0.0.1:4223:4222
            - 127.0.0.1:8222:8222
        volumes:
            - nats-data:/data
        networks:
            - eda-net

volumes:
    pg-data:
    nats-data:

networks:
    eda-net:
```

```
# .postgres.env
POSTGRES_USER=edasuper
POSTGRES_PASSWORD=supersecure
POSTGRES_DB=edadb
POSTGRES_HOST_AUTH_METHOD=scram-sha-256
```

Two things to actually do on your end before running this, not just files to
save. First, add `.postgres.env` to `.gitignore` (and commit a
`.postgres.env.example` with placeholder values alongside it) — the file above
has a real password in it, and this is the last point before it could get swept
into a `git add .`. Second, since `eda-net` no longer has `external: true`,
compose will happily create it itself on `docker compose up` — you don't need to
`docker network create` by hand anymore, and I'll drop that from the Makefile
plan in Step 2 accordingly.

You mentioned removing the old volumes manually — once that's done and the
network's `external: true` is gone, bring it up with `docker compose up -d`,
then:

- `docker compose ps` — confirm both `eda-postgres` and `eda-nats` show `Up`
- `docker volume ls` — confirm `pg-data` and `nats-data` exist (they'll be
  prefixed with the project/folder name, e.g. `eda-storefront_pg-data`)
- `docker network ls` — confirm `eda-net` exists, created fresh this time
- `docker compose logs nats` — should report JetStream enabled and show `/data`
  as the store directory
- `docker compose logs postgres` — should show normal init output ending in
  `database system is ready to accept connections`, with no `FATAL` lines

Let me know what you get, and any questions on the `eda-net` change or the env
file before we move to Step 2 (the `Makefile`).

#### bypass of `trust` rules

This confirms the `host` line is working — but the `pg_hba.conf` output reveals
something more interesting than expected, worth working through together rather
than skipping past.

Look at what actually happened: you connected to `127.0.0.1:5433`, which Docker
forwards into the container as a connection to Postgres's port 5432. If
`pg_hba.conf` matches top-to-bottom and stops at the first match, you'd expect
this connection to hit `host all all 127.0.0.1/32 trust` — the second entry in
the auto-generated block — and connect without a password prompt at all. But it
_did_ prompt for a password. Why do you think that line didn't match, given you
genuinely connected from `127.0.0.1`?

The answer is a Docker networking detail worth internalizing now, since it'll
matter again the moment `depot`/`baskets` try to reach this same Postgres
container in Project 2: `127.0.0.1` inside the _container's_ network namespace
is not the same `127.0.0.1` you typed on your host. Docker's port-forwarding
(`127.0.0.1:5433->5432`) doesn't preserve the source address as literal loopback
once the packet crosses into the container — from Postgres's point of view, the
connection arrives from whatever address the Docker network's proxy/gateway
assigns it (something in the `eda-storefront_eda-net` bridge subnet, not
`127.0.0.1`). So the `127.0.0.1/32` and `::1/128` trust lines are effectively
dead code in this setup — they'd only ever match a process running _inside_ the
container itself connecting to its own loopback, which nothing here does. Your
connection instead fell through to the catch-all
`host all all all scram-sha-256` line at the bottom, which is why you got
prompted — correctly, but for a reason different from what the file's structure
might suggest at a glance.

This is actually a second real finding for the end-of-project hardening review,
not just a curiosity: those two `trust` lines aren't doing anything useful for
us, but they're also not obviously harmless dead weight either — worth deciding
explicitly (remove them via the custom HBA approach, or leave them since they're
inert in this topology) rather than carrying forward unexamined. I'd rather log
this precisely now, while we've just worked out why, than try to reconstruct the
reasoning later.

So, current state: SCRAM is genuinely enforced for every real connection path
this system will use, and the one gap (local Unix-socket trust) is inert as long
as nothing runs inside the container itself. That's a reasonable place to stop
for Phase 6 and move on — Step 1 is functionally done.

Quick checklist before we close out Step 1: are you good with where Postgres's
HBA landed for now, and ready to also spin up the NATS-side durability check
(publishing a test message and confirming `nats-data` survives a
`docker compose down`/`up`), or would you rather fold that into Step 5 as
originally planned and move straight to Step 2 (the `Makefile`)?

### Step 2: Makefile

Good — Step 1 is closed. Here's Step 2: the `Makefile`.

Per the roadmap, this is meant to be a living deliverable that grows by one set
of targets per project, so I'll keep this first version focused on what Phase 6
actually needs: bringing the stack up and down, tailing logs, and running the
smoke-test service we'll build in Steps 3–4.

```makefile
# Makefile

COMPOSE := docker compose

.PHONY: up down logs ps smoke

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

smoke:
	go run ./foundation-smoketest
```

A few things worth flagging before you run anything.

`down` here intentionally does _not_ pass `-v` — that's the flag that would also
delete the named volumes (`pg-data`, `nats-data`). Given the entire point of
Steps 5–6 is proving those volumes survive a container teardown, a `down` target
that quietly wipes them on every run would sabotage the exact thing we're trying
to verify. When we actually get to Step 5's durability test, we'll run
`docker compose down` (no `-v`) deliberately, then `up` again, and check the
data's still there — `make down` as defined gives you that by default. If you
ever _want_ a clean slate (e.g., testing the SCRAM/`pg_hba` behavior from
scratch again like we just did), that stays a manual `docker compose down -v`,
not something the Makefile should make a single `make` command away from by
accident.

`smoke` assumes the throwaway service will live at `./foundation-smoketest`
relative to the workspace root, as a `go run`-able package — matching the
`tmp-smoketest` convention from Phases 1–5, just promoted from "temporary,
deleted after" to "the thing Phase 6 actually builds and keeps around to prove
the kit works end-to-end." If you'd rather name or place it differently, this is
the moment to say so, since Step 3 will scaffold whatever path we agree on here.

I left out an explicit `network create` target, since dropping `external: true`
last step means `docker compose up` creates `eda-net` itself now — no separate
step needed.

One open question before we lock this in: do you want a `migrate` target as a
placeholder now (even as a no-op or a comment marking where `golang-migrate`
will plug in later, per ADL 0003), or hold off since nothing in `foundation`
itself owns a schema yet — that's arguably `stores`' concern starting in Project
1, not `foundation`'s? I'd lean toward holding off, since adding a target with
nothing behind it yet is the kind of unstated-placeholder pattern the project's
discipline has been pretty careful to avoid elsewhere (e.g. `pool.go`'s comment
about not assigning from zero values). Let me know your take, and give `make up`
/ `make down` / `make ps` / `make logs` a try against the stack you already have
running to confirm the Makefile itself works before we move to Step 3.
