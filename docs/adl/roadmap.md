# Event-Driven Storefront — Multi-Project Roadmap

## Vision

A modernized adaptation of the system built in _Event-Driven Architecture in
Golang_ (Packt) — a multi-service, event-driven commerce platform (catalog,
cart, ordering, fulfillment, payments, accounts) coordinated over NATS, built
service-by-service as a sequence of independent learning projects rather than
all at once. Each project produces one or two real, live, deployable services.
No service is ever stubbed to unblock a later project — every project that
depends on another service depends on a genuinely running instance of it.

This continues the discipline established across `order-fulfillment` and
`court-reservation`: small, deliberately-scoped projects, phase-by-phase
progress logs, real verification (curl/integration tests against live
infrastructure, not assumption), and design decisions made explicitly and
recorded rather than defaulted into.

## What's different this time

Both prior projects were single-process modular services. This system is
deliberately larger in a new dimension: multiple independent Go services,
each its own project, coordinating over a real message broker and real
network boundaries. That introduces two structural risks neither prior
project had to manage:

1. **Losing the mental model of the whole system** as it spans more and more
   project directories and sessions.
2. **Losing track of what's actually alive** — since later projects need
   earlier services running for real, "what's built" and "what's currently
   running" are different questions that both need answers.

The documentation model below exists specifically to manage those two risks.

## Workspace structure

A single Go workspace (`go.work`) ties every service module together for
local, cross-module development without needing to publish or version
anything. Proposed layout:

```
eda-storefront/                    (workspace root)
├── go.work
├── go.work.sum
├── docker-compose.yml             (grows by one service block per project)
├── Makefile                       (grows by one set of targets per project)
├── foundation/                    (shared kit: config, graceful shutdown,
│                                    logging, NATS/Postgres wiring — built
│                                    once during Project 1, reused everywhere)
│   └── go.mod
├── 01-stores/
│   ├── journey_progress.md
│   └── go.mod
├── 02-depot-baskets/
│   ├── journey_progress.md        (one log covering both services,
│   │                                same convention as order-fulfillment's
│   │                                Order+Inventory pairing)
│   ├── depot/
│   │   └── go.mod
│   └── baskets/
│       └── go.mod
├── 03-ordering/
│   ├── journey_progress.md
│   └── go.mod
├── 04-payments/
│   ├── journey_progress.md
│   └── go.mod
├── 05-customers/
│   ├── journey_progress.md
│   └── go.mod
├── 06-storefront/
│   ├── journey_progress.md
│   └── go.mod
├── 07-search/
│   ├── journey_progress.md
│   └── go.mod
├── 08-notifications/
│   ├── journey_progress.md
│   └── go.mod
└── docs/
    ├── roadmap.md                 (this file)
    ├── system-state.md            (live status of every service — see below)
    └── adl/
        ├── 0001-service-build-order.md
        ├── 0002-aggregate-persistence-model.md
        ├── 0003-modernized-technology-stack.md
        └── (future decisions, numbered sequentially)
```

## The documentation model — three layers

**1. Per-project `journey_progress.md`** — unchanged from `order-fulfillment`
and `court-reservation`. Phase-by-phase: what was built, issues found and how
they were diagnosed, key concepts reinforced, state at end of phase, what's
carried into the next phase. Lives inside each project's own directory.
Answers: _"what happened in this project, in detail?"_

**2. System-level Architecture Decision Log (`docs/adl/`)** — new. Captures
decisions that apply across the whole system rather than to one bounded
context: build order, persistence model, technology stack choices, Compose/
workspace conventions. Numbered, dated, following the
Context → Decision → Consequences shape (the same pattern the source book
itself uses in its own `docs/ADL/`). Answers: _"why did we decide that,
system-wide, and would we still decide it the same way?"_

**3. `docs/system-state.md`** — new, and the most important addition for
this specific project. A single, small, boring table: every service, whether
it's actually live right now, what it publishes/consumes, which project
directory owns it, when it was last touched. Updated at the end of every
project. Its whole job is answering _"what's actually running, and how does
it fit together"_ at a glance, without reconstructing the answer by
re-reading eight separate progress logs.

**Working agreement:** no project starts without first updating
`system-state.md` to confirm what it can assume is genuinely live, and no
project ends without updating it again to reflect what's now live as a
result.

## Project sequence

| #   | Project            | Service(s)                | Must already be live                                                                      | What's genuinely new                                                                                                                                                                        |
| --- | ------------------ | ------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `01-stores`        | `stores`                  | — (root)                                                                                  | The `foundation/` package (config, graceful shutdown, logging, NATS/Postgres wiring), built once here and reused by every later project; first real NATS-publishing service                 |
| 2   | `02-depot-baskets` | `depot`, `baskets`        | `stores`                                                                                  | Consuming real events off a real broker and projecting them into a local read cache — done twice, against two different domains, in one project                                             |
| 3   | `03-ordering`      | `ordering`                | `stores`, `depot`, `baskets`                                                              | Fan-in coordination: one service reacting to two independent upstream event sources; the real event-driven analogue of `order-fulfillment`'s in-process `PayOrder`/`Inventory` coordination |
| 4   | `04-payments`      | `payments`                | all of the above                                                                          | Simplest remaining consumer (single upstream); closes the core commerce loop end-to-end                                                                                                     |
| 5   | `05-customers`     | `customers`               | — (root, but built now rather than earlier, since nothing needed it until the storefront) | Second and last standalone root service                                                                                                                                                     |
| 6   | `06-storefront`    | web UI (Templ + Datastar) | `stores`, `baskets`, `ordering`, `payments`, `customers`                                  | First frontend project; Datastar/SSE integration with real backend events; real customer identity from day one (no placeholder auth)                                                        |
| 7   | `07-search`        | `search`                  | `stores`, `ordering`, `customers`                                                         | Multi-source read-model / CQRS projection pattern; feeds a real "order history" page back into the storefront                                                                               |
| 8   | `08-notifications` | `notifications`           | `customers`, `ordering`                                                                   | The one piece the source book leaves unimplemented — real integration with an email/SMS provider behind a `Notifier` port, verified against a real inbox (Mailpit) before a real provider   |

See `docs/adl/0001-service-build-order.md` for the reasoning behind this
order, including the alternatives that were considered and rejected —
particularly why `customers` was moved earlier than its position in the
book's own dependency graph would suggest.

## Working agreements carried into every project

- **No stubbing of service dependencies.** Every "must already be live"
  column above means a real container, running via the shared
  `docker-compose.yml`, not a mock or an in-memory fake standing in for
  another service.
- **The shared `docker-compose.yml` and `Makefile` are living deliverables**,
  extended by every project, not rebuilt per project.
- **Every design decision with system-wide consequences gets an ADL entry**,
  not just a mention in a single project's progress log.
- Aggregate design, testing discipline, and verification-over-assumption
  (real curl checks, real concurrent-load tests, real `git log --stat`
  swap-test checks) carry forward unchanged from `order-fulfillment` and
  `court-reservation`.
