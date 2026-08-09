# 3. Modernized technology stack

Date: 2026-08-09

## Status

Accepted

## Context

The source book's `go.mod` pins `go 1.18` and dependencies from 2022 —
`pgx/v4`, `zerolog`, `grpc-gateway` v2.11, OpenTelemetry v1.10, `goose`,
core NATS (no JetStream), raw `protoc` invocations. Both prior projects
(`order-fulfillment`, `court-reservation`) already established working
conventions for several of the equivalent choices, using current tooling.

## Decision

| Concern                 | Source book                       | This system                                                                                                                                                                                      |
| ----------------------- | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Go version              | 1.18                              | current stable                                                                                                                                                                                   |
| Postgres driver         | `pgx/v4`                          | `pgx/v5` + `pgxpool` — already proven in both prior projects                                                                                                                                     |
| Event transport         | core NATS (fire-and-forget)       | **NATS JetStream** — durable, replayable streams; a better fit generally, and specifically pairs with a possible future notifications/audit replay need                                          |
| Logging                 | `zerolog`                         | `log/slog` (stdlib) — already proven in both prior projects; kept in the shared `foundation/` package rather than rebuilt as a custom composition each time                                      |
| HTTP routing/middleware | `chi`                             | hand-rolled `Router`/`Middleware`, following `court-reservation`'s Phase 6 pattern, built once in `foundation/` and reused                                                                       |
| Migrations              | `goose`, auto-run on service boot | `golang-migrate`, explicit migration step — already proven in both prior projects                                                                                                                |
| gRPC↔HTTP gateway       | `grpc-gateway` v2.11              | current `grpc-gateway`, or ConnectRPC if it proves simpler once we're actually building a service that needs it — left as an open choice, to be decided in Project 1 rather than assumed here    |
| Protobuf tooling        | raw `protoc`                      | `buf`                                                                                                                                                                                            |
| Observability           | OpenTelemetry v1.10               | current OpenTelemetry SDK — real API rework required, not a version bump, since the SDK has had breaking changes across this span                                                                |
| Config                  | `envconfig` + `.env` files        | kept as-is — genuinely better suited to a multi-service, containerized deployment than the CLI-flag convention used in the single-binary prior projects; not changed just for consistency's sake |

## Consequences

- The shared `foundation/` package built in Project 1 is where most of
  these decisions actually get implemented once; every later project
  inherits them rather than re-deciding.
- The ConnectRPC-vs-grpc-gateway choice is explicitly deferred to Project 1,
  not pre-decided here — recorded as an open question rather than silently
  resolved, so it isn't forgotten.
- OpenTelemetry integration is expected to be genuinely more effortful than
  the "modernize the version number" framing might suggest, given real
  breaking API changes since v1.10; scoped as its own piece of work
  whichever project first needs tracing across a service boundary.
