# 4. Cross-project artifact boundary: contracts travel forward, implementations don't

Date: 2026-08-09

## Status

Accepted

## Context

Each service is built in its own project folder/session, specifically to
keep any single session's context window bounded regardless of how large
the overall system grows (see `docs/roadmap.md` — this is the whole reason
the system is split into projects in the first place, not an incidental
side effect).

The natural question once a second project depends on a first is: what,
concretely, gets carried forward into the new project's folder? Re-uploading
each prior service's full implementation was considered and would directly
undermine the reason for splitting into projects — by the later services
(`search`, `notifications`), a session would be carrying seven other
services' worth of source before writing a line of its own.

It's also unnecessary given this system's specific architecture. Unlike a
single-process modular monolith (`order-fulfillment`, `court-reservation`),
these services are decoupled by a real network/event boundary, not by
shared source. `depot`/`baskets` don't call into `stores`' Go code — they
consume events `stores` publishes onto NATS. Building a consumer requires
knowing what a prior service _exposes_ (event names, payload shapes, any
synchronous API surface) — not how it's implemented internally.

One exception: `foundation/` is a genuine shared Go module every service
actually imports (config, graceful shutdown, logging, NATS/Postgres
wiring). That's a real code dependency, not just useful context, and is
categorically different from "what did `stores` publish."

Separately: verification of a live service (curl checks, concurrency
tests) happens on the user's own machine against real running containers —
Claude does not have direct access to the user's local Docker/NATS/Postgres
setup from this interface. The user runs the checks and reports terminal
output back into the session.

## Decision

What travels forward into a new project's folder, per prior dependency:

- **`foundation/`** — carried forward in full, every time. It's a real
  import, not reference material.
- **Every other prior service** — only its `CONTRACT.md` (see the
  template in `docs/templates/CONTRACT.md`), not its implementation.
  `CONTRACT.md` lists exactly what a service publishes and exposes:
  event names, payload shapes, and any REST/gRPC endpoints another
  service might call directly.
- **`.proto` files**, if/when protobuf is adopted for event or API
  contracts (still open per `docs/adl/0003-modernized-technology-stack.md`)
  — promoted to the same tier as `foundation/` once adopted, since they're
  used for real code generation in the consuming service, not just read as
  reference.
- **`journey_progress.md`** stays local to its own project folder and is
  never carried forward. It answers "what happened in this project," which
  is only relevant to someone continuing _that_ project, not to a
  downstream consumer.

Each project produces its own `CONTRACT.md` as a deliverable, alongside
`journey_progress.md`, finalized at the same point `docs/system-state.md`
is updated at the end of the project.

Live-service verification remains manual: the user runs curl/integration
checks against their own running containers and reports the results back
into the session, the same verification discipline used throughout
`order-fulfillment` and `court-reservation`, just executed outside the
chat interface rather than via a tool call.

## Alternatives considered

**Carrying forward each prior service's full source.** Rejected — directly
defeats the context-window-minimization purpose the multi-project split
exists for, and is unsupported by the actual coupling: consumers depend on
a network contract, not on the producer's internals.

**Relying on the memory feature instead of explicit contract files.**
Considered and deferred, not rejected outright. Memory distills general
impressions across past conversations — well suited to retaining working
style, poorly suited to carrying an exact, auditable contract (event names,
payload field shapes) forward with fidelity. Explicit versioned files
remain the source of truth; memory, if enabled, is at most a style-level
supplement.

## Consequences

- Every project folder from `02-depot-baskets` onward attaches: the full
  `docs/` folder (roadmap + ADLs + system-state, per the existing
  convention), `foundation/` in full, and only the `CONTRACT.md` (plus any
  `.proto` files) of each service listed as a live dependency in that
  project's `docs/roadmap.md` row — not those services' source trees.
- A project's context budget stays roughly constant regardless of how many
  services now exist elsewhere in the system, since the per-dependency cost
  is one small contract file, not a codebase.
- `CONTRACT.md` needs to be kept honest and current — if a later phase of a
  project changes a published event's shape, `CONTRACT.md` must be updated
  in the same phase, or downstream projects will be built against a stale
  contract. This is the same "don't let documentation drift from reality"
  discipline `system-state.md` already depends on.
- If a future service ever needs something from a prior service beyond its
  published contract (e.g. a genuine reason to read its domain logic), that
  is itself worth surfacing explicitly rather than defaulting to re-upload —
  likely a sign the contract boundary was drawn in the wrong place.

## Consequences (addendum, 2026-10-15)

- The Decision above covers projects that build a new service on top of live
  ones: only `CONTRACT.md` (plus `.proto` files) travels forward. It did not
  anticipate a project that modifies live services. `contract-evolution`
  (ADL 0020) changes `stores`, `baskets`, `depot` and `ordering`, so for those
  services the source is a real dependency, not reference material, and the
  full source of every modified service is attached.
- Services a modifying project does not change still travel as contract only.
  `customers` is the example: `ordering` will import `customerspb`, so the
  generated `customerspb` package is attached too, on the same reasoning that
  applies to `foundation/` and to `.proto` files adopted for code generation
  (ADL 0017).
- `journey_progress.md` is unchanged by this: it stays local to its project.
  A modifying project's progress log lives in its own folder
  (`contract-evolution/journey_progress.md`), and each modified service's
  `CONTRACT.md` is updated in the same phase that changes its contract, as
  the Consequences above already require.
