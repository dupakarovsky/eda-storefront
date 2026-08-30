# 8. Query path routes through the application layer, never repository-direct

Date: 2026-08-25

## Status

Accepted

## Context

Phase 4's read path (`GetStore`, `ListProducts`) was the first genuinely new
kind of code this project introduced — every prior use case existed to
coordinate something (validate, mutate, save, publish), while a query is, at
first glance, a pure pass-through: `repo.FindByID`, translated to wire format,
nothing else. The initial proposal for Step 6 was to let `connectapi.Server`
call `StoreRepository`/`ProductRepository` directly for reads, reserving the
`application` layer for writes only — avoiding a `GetStoreQuery` type whose
entire body would be a single repository call with no logic of its own.

This was reconsidered specifically because `connectapi.Server` is the
service's one public network entry point. Every byte it receives originates
outside this process's trust boundary. The system's existing discipline —
established implicitly through Phases 1-3 and made explicit here — has been
that `application` is the layer responsible for ensuring anything reaching a
repository has already been validated/sanitized; a repository's own code has
never defended itself against malformed input, because it has never needed
to. Wiring `connectapi.Server` directly to a repository for any RPC, read or
write, creates a second path into persistence that bypasses that assumption
— not exploitable today, since neither query does anything but a keyed
lookup, but a structural gap that would only get worse as queries grow
(pagination, filtering, sorting parameters) without ever being deliberately
decided to grow that way.

## Decision

Every RPC `connectapi.Server` exposes — command or query — calls into a
corresponding `application` layer type, which is the only thing permitted to
call a repository. Query use cases (`GetStore`, `ListProducts`) are as thin
as the operation actually is today — no validation logic exists yet because
none is needed yet — but they exist as real, separate types with the same
structural shape as every command use case, not as an exception carved out
for reads.

## Alternatives considered

**`connectapi.Server` calling repositories directly for reads only.**
Rejected — see Context. Would have meant two different trust boundaries
depending on HTTP verb-equivalent (command vs. query), for a distinction a
network caller has no reason to respect, and no future maintainer would have
reason to expect without reading this file.

## Consequences

- Every future service's API layer faces this same fork the moment it adds
  its first query RPC — this ADL is the answer, not something to re-derive
  per service.
- `application`-layer query types remain intentionally minimal until a query
  actually needs logic beyond "call the repository" — the moment one does
  (pagination, a join across repositories, an authorization check), that is
  the natural, already-anticipated place for it to land, not a reason to
  question the boundary itself.
- This does not change `ProductRepository`/`StoreRepository`'s own shape or
  ADL 0002's `Load`/`Save`-plus-query-methods design — it constrains who is
  allowed to call them, not what they expose.
