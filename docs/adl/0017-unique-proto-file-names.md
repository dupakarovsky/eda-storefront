# ordering/../docs/adl/0017-unique-proto-file-names-per-module.md

# 17. Every service's proto file needs a globally-unique name, not a generic `api.proto`

Date: 2026-09-29

## Status

Accepted

## Context

`ordering`'s Phase 6 build (its first synchronous ConnectRPC surface) was the
first point in this system where one service's binary imports another
service's generated protobuf package directly — `ordering`'s `depotclient`
imports `depot/depotpb`, alongside `ordering`'s own `orderingpb`, in the same
process. This immediately panicked at startup:
`proto: file "api.proto" is already registered`.

Go's protobuf runtime keeps a single, process-wide registry of every `.proto`
file it has seen, keyed by that file's path relative to its own module's
root — not by Go import path, not by proto package name. Every service's
`buf.yaml` declares its own generated-code directory (`depotpb/`,
`orderingpb/`, etc.) as that service's module root, and every service's
source file was named identically: `api.proto`. As long as every service ran
as its own isolated process, this was invisible — `stores`, `depot`, and
`baskets` each registered exactly one `"api.proto"` in their own process,
with no possibility of collision. `ordering` was the first to break that
isolation by linking a second service's generated package into its own
binary as a live RPC client, not just a reference read from `CONTRACT.md`.

## Decision

Every service's `.proto` source file must be named uniquely across the whole
system, not generically (`api.proto`). Concretely: `stores/storespb/api.proto`,
`depot/depotpb/api.proto`, `baskets/basketspb/api.proto`, and
`ordering/orderingpb/api.proto` are each renamed to `<service>.proto`
(`stores.proto`, `depot.proto`, `baskets.proto`, `ordering.proto`) — content,
message names, and service names all unchanged, only the source filename
(and therefore the registered proto path, and the generated `<service>.pb.go`/
`<service>.connect.go` filenames under `paths=source_relative`).

## Consequences

- Every future service that exposes a `.proto` file must give it a name
  unique across the system from the start — this ADL is the default to
  follow, not something to rediscover the next time two generated packages
  end up sharing a process.
- This collision only ever surfaces when two services' generated packages
  are linked into the same binary. `stores` and `baskets` haven't hit it
  yet, since nothing currently imports their generated packages directly —
  but the same fix would be needed the moment a future service does what
  `ordering` did with `depot`.
- No Go application code changed as a result of this fix — only the source
  `.proto` filename and the regenerated output. Every existing call site,
  message reference, and RPC handler continues to compile unchanged.
