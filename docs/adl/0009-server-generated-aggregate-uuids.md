# 9. Server-generated aggregate IDs

Date: 2026-08-22

## Status

Accepted

## Context

Every `application`-layer constructor for a root aggregate
(`domain.NewStore`, `domain.NewProduct`) takes an `id` as a parameter, a
shape inherited from Phase 3's `CreateStore.Execute`, which was only ever
exercised by a smoketest supplying an arbitrary string
(`cmd/repo-smoketest/main.go`). Phase 4 introduced the first genuine external
caller — `connectapi.Server` — and with it a real decision that had been
implicit until now: who is responsible for choosing an aggregate's ID, the
caller or the service.

## Decision

The service generates every aggregate ID itself, via `uuid.NewString()`
inside the relevant Connect handler (`Server.CreateStore`,
`Server.AddProduct`), immediately before calling into the corresponding
`application` use case. `CreateStoreRequest`/`AddProductRequest` carry no ID
field; the generated ID is returned to the caller as part of the response.

## Consequences

- Every future service's own `Create`-shaped RPC faces this same fork — this
  ADL is the default to follow unless a concrete reason to deviate exists
  (e.g. a service where the caller must supply a pre-existing external
  identifier it doesn't control, which is a different problem than this one).
- `domain.NewStore`/`NewProduct` keep taking `id` as a plain parameter rather
  than generating it internally — construction stays testable with
  known, fixed IDs (as every domain test from Phase 1 onward already
  assumes), and ID generation remains a concern of the layer that owns
  talking to the outside world, not the aggregate itself.
- `uuid.NewString()` in the handler is the only place in the codebase that
  currently needs the `github.com/google/uuid` dependency; the
  `application` and `domain` layers have no reason to import it.
