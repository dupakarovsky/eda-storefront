<!--
CONTRACT.md template — see docs/adl/0004-cross-project-artifact-boundary.md
for why this file exists and what it's for.

Copy this into each project folder (e.g. stores/CONTRACT.md) and fill it
in as the service's public surface solidifies — realistically finalized at
the same point journey_progress.md's "State at End of Project" section and
docs/system-state.md are updated.

What belongs here: only what a DIFFERENT service, built in a DIFFERENT
project/session, would need to know to consume this one — without reading
its source. Event names, payload shapes, synchronous endpoints.

What does NOT belong here: internal domain model, aggregate design,
database schema, repository implementation, or anything else covered by
journey_progress.md. If you're describing HOW something works internally
rather than WHAT another service can rely on, it belongs in
journey_progress.md instead, not here.

Delete this comment block once the file is filled in.
-->

# Contract — `<service-name>`

**Status:** Not started / In progress / Live
**Project folder:** `NN-<project-dir>/`
**Last updated:** YYYY-MM-DD

One-sentence description of what this service owns/is authoritative for.

## Published events

Events this service publishes onto NATS that another service may subscribe
to. One entry per event.

### `EventName`

- **Subject:** `<nats.subject.pattern>`
- **Published when:** plain-language trigger — what real-world/domain
  action causes this to fire
- **Payload:**
    ```json
    {
        "field_name": "type — one-line description",
        "another_field": "type — one-line description"
    }
    ```
- **Delivery guarantees:** e.g. at-least-once via JetStream, consumer
  group expectations, ordering guarantees (or explicitly: none)

_Repeat per event._

## Synchronous API surface

Any REST/gRPC endpoints another service is expected to call directly,
rather than react to via an event. Most services in this system should
have a short list here — most cross-service interaction should happen via
published events, not synchronous calls. If this section is long, it's
worth asking whether some of these should be events instead.

### `MethodOrRoute`

- **Protocol:** gRPC / REST
- **Request:** shape
- **Response:** shape
- **When a consumer would call this** rather than rely on a published event

_Repeat per endpoint, or state "None — this service is event-only" if
there isn't one._

## Data this service owns

One or two lines on what this service is the source of truth for — mainly
so a consumer knows what it must NOT try to own a duplicate/competing copy
of (e.g. "Stores is the source of truth for product existence and pricing;
consumers may cache a read-only projection but never write it back").

## Explicit non-goals of this contract

Anything a consumer might reasonably assume is available but isn't —
e.g. no bulk/batch query endpoint, no historical event replay beyond
JetStream's retention window, no synchronous "does X exist" check. Keeps a
downstream project from discovering a gap mid-implementation instead of
before starting.
