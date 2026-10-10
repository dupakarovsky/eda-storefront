# 11. NATS containers rely on client-side retry, not Docker healthchecks

Date: 2026-08-30

## Status

Accepted

## Context

`docker-compose.yml`'s `postgres` service has a working `pg_isready`-based
healthcheck, letting dependent services use
`depends_on: condition: service_healthy` to wait for genuine readiness, not just
container start. The equivalent was attempted for `nats` while containerizing
`stores` (Phase 4, Step 9) using `wget --spider` against NATS's own `/healthz`
monitoring endpoint — a real, working endpoint, confirmed reachable via `curl`
from the host. The healthcheck nonetheless failed on every attempt.

Direct experimentation (`docker run --entrypoint sh nats:2.12`, then
`--entrypoint ls`) confirmed the official `nats:2.12` image ships with no shell
and no common utilities at all — not `wget` specifically failing, but every
command tried failing identically with "executable file not found." Docker
healthchecks using the `CMD-SHELL` form require a shell to run the test string
through; there is no way to express a healthcheck against this image using that
mechanism, regardless of which tool the check itself tries to invoke.

## Decision

`nats` gets no Docker healthcheck. `depends_on` for any service depending on
`nats` uses `condition: service_started` (container process launched) rather
than `service_healthy`. The actual readiness guarantee instead comes from
`nats.go`'s `RetryOnFailedConnect(true)` option, set in
`foundation/nats.Connect` — a dependent service's initial connection attempt no
longer fails outright if NATS isn't accepting connections yet; it retries in the
background per the existing `MaxReconnects`/ `ReconnectWait` configuration, and
any real operation issued against the connection (e.g. `EnsureStream`'s
`CreateOrUpdateStream` call) naturally blocks until the connection completes.

## Consequences

- Every future service depending on `nats` in `docker-compose.yml` inherits this
  same constraint — do not attempt to add a `wget`/`curl`/`nc`-based healthcheck
  to the `nats` service block without first re-confirming whether a given NATS
  image version has changed this (a future `nats:2.x` release could plausibly
  ship a shell again); absent that confirmation, assume it still doesn't.
- `RetryOnFailedConnect` is arguably more robust than a startup-only healthcheck
  gate regardless of the image's tooling — it also covers NATS restarting
  independently of a running dependent service later (e.g. a bare
  `docker compose restart nats` on an otherwise-live system), a case
  `depends_on`'s ordering guarantee never covered even when a healthcheck was
  possible.
- Postgres and NATS now have genuinely different readiness strategies in this
  system — Postgres via container-level healthcheck, NATS via client-level retry
  — which is a deliberate asymmetry driven by what each image actually supports,
  not an inconsistency to "fix" by forcing one pattern onto both.

## Consequences (addendum, 2026-10-09)

- `RetryOnFailedConnect(true)` covers only the initial connect. Once connected,
  the client's reconnect loop is bounded by `MaxReconnects`, and
  `foundation/config` defaulted it to 5 with a 2s wait. A NATS outage longer
  than roughly 10 seconds therefore closed every service's connection
  permanently. `/health` returned 500, subscribers silently stopped consuming,
  and `restart: unless-stopped` did nothing because the process never exited.
  This contradicted the claim above that the client-level retry covers NATS
  restarting independently of a running service.
- Found by `customers`' Phase 3 outage test (retries exhausted with
  `nats: connection closed`), then confirmed on the live stack: all four
  services returned 500 after a 10-second NATS stop.
- Resolved by changing `foundation/config`'s `NATS_MAX_RECONNECTS` default to -1
  (unlimited), so every current and future service inherits it. Verified three
  ways: a test that holds NATS down for 15s and checks the connection survives
  and publishes again (fails with the old value, passes with the new one); all
  four live services returning 200 after a 15s outage; and the subscribers
  resuming, shown by a new store appearing in both catalog caches.
- Side effect to remember: with unlimited reconnects the client buffers
  publishes made while disconnected (8MB by default) and flushes them on
  reconnect, even after the caller gave up. An event whose retries were
  "exhausted" can still arrive later if the process survives. This is not a
  guarantee: a crash loses the buffer, and it can overflow. It doesn't change
  the outbox question in ADL 0007.
- `NATS_MAX_RECONNECTS=0` does not mean "never reconnect":
  `foundation/nats.Connect` skips the option on zero and the library default
  applies.
