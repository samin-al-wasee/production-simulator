# ADR-0017: The application instance as a modelled backend service

**Status:** accepted
**Date:** 2026-10-01

## Context

Phase 11 deepens one component at a time. After the Internet ([ADR-0016](0016-configurable-internet-traffic.md)), the next is the **application instance**: one running replica of a backend web or API service (FastAPI, Django, Rails, Express, Spring Boot, and so on). It is not a worker, a database, a cache, or a broker.

Until v4 the application was a flat capacity: 50 operations per second per replica, times a size factor, with latency `40 ms / (1 − ρ)` and fixed read, write, and storage routing. A learner could not ask why a backend is slow, why requests queue or time out, why CPU saturates, why sync and async differ, or why a slow database slows the application.

## Decision

1. **One canvas component.** Server, middleware, router, handlers, workers, CPU, memory, connections, and the request queue are internal. External services (cache, database, queue, storage) stay separate components, and routes call them.
2. **Configuration is on the node.** `AppConfig` is set with the existing `configure` command (now accepted for application instances), validated in the engine, and replayed. A new instance uses the ruleset's default. It holds:
   * **Labels** with no model effect: name, framework, version, environment, protocol, port, interface, server. A framework is a preset the dashboard offers (ruleset data); it fills in the processing model and workers.
   * **Processing:** `sync` holds a worker per request for its whole duration, including dependency waits; `async` parks waiting requests, so slots are `MaxConcurrency`.
   * Workers, backlog, max connections, request timeout, TLS, and keep-alive.
   * **Middleware** from a fixed catalog: request ID, logging, CORS, authentication, validation, compression, and rate limiting. Each adds milliseconds and CPU-milliseconds to every request; rate limiting rejects above its per-replica limit. There is no arbitrary code.
   * **Routes** keyed by Internet endpoint, plus a required catch-all `*`. Each route has a base time, CPU time, memory per request, request and response size, a handler error rate, and dependencies.
3. **Resources come from the priced size.** small is 1 vCPU, 1 GB, and 100 Mbps; medium is 2, 4, and 250; large is 4, 8, and 500. Free resource knobs would break the economy.
4. **Capacity is not configured; it emerges.** Per tick, each limit is its capacity divided by the mix-weighted use per request, and the smallest limit wins and is reported as the **bottleneck**:
   * **CPU:** `min(workers, vCPU) × 1000` CPU-ms per second, against handler, middleware, and TLS-handshake CPU.
   * **Slots:** workers (sync) or max concurrency (async), against wall time (Little's law).
   * **Connections:** against in-flight requests plus keep-alive idle connections.
   * **Network in and out:** against request and response sizes.
5. **Dependency waits use last tick's latency.** A route's wall time includes its dependencies' latency from the previous tick. That is the same device retries use: deterministic, and a slow database fills sync workers on the next tick.
6. **Overload follows from the queue.**
   * Below capacity, the wait is `mean wall time × ρ / (1 − ρ)` and requests queue up to the backlog.
   * At capacity, the backlog fills, its wait is `backlog / capacity`, and the excess is **rejected**.
   * Waits are taken as exponential: a request **times out** with probability `exp(−(timeout − own − dependencies) / wait)`.
   * A request succeeds when it is served, finishes in time, its handler does not fail, and every dependency call succeeds.
   * The server works on every request it accepted, so timed-out requests still load its dependencies.
7. **Memory is a limit you crash into.** Use is workers × resident memory plus in-flight and queued requests × their memory. Past the size's memory the instance is **out of memory**: it is down for `RestartTicks`, then starts again. Starting costs `StartSeconds` of the tick's capacity.
8. **Health is derived:** stopped (down), unhealthy (out of memory, or more than 20% failing), starting, degraded (ρ > 0.85 or more than 1% failing), otherwise healthy. It is never a setting.
9. **Rates, not request objects.** Outcomes are expected rates per route: success, error, timeout, rejected. Like the Internet model, they are deterministic and checkable by hand.
10. **Route mix without a new vector.** Nothing between the Internet and an application treats endpoints of one class differently, so the instance splits each class back into endpoints in the Internet's shares. This is marked `ponytail:` in the code; a per-endpoint vector replaces it when a component routes or blocks by path.
11. **Dependencies.**
    * `cache` falls back to the database when no cache is connected.
    * `db-write` goes through a connected queue, else to the primary, as v4 routed writes.
    * `db-read`, `queue`, and `storage` need their target; a call to a missing target fails the request.
    * From v5, routes decide which requests fetch from storage, and the Internet endpoints' storage flag is ignored.
12. **Ruleset `sandbox/v5`** carries the default configuration, the middleware and framework catalogs, size resources, and the runtime constants: 150 MB per worker, 2 CPU-ms per TLS handshake, 10 requests per kept-alive connection, 5 s of keep-alive idle, and 30 s to start. Rulesets v1 to v4 keep the flat model, and their saves replay bit for bit.

## Consequences

### Positive

* Every question in the brief has an observable answer: the bottleneck, queue and wait, timeouts and rejections, CPU and memory, per-route latency, and health.
* Sync versus async, worker counts, keep-alive, middleware, and slow dependencies change behavior for reasons a learner can check.
* Default capacity stays close to v4: about 56 requests per second on a small instance at the default mix.

### Negative / Trade-offs

* **Overload is now realistic.** A full backlog whose wait exceeds the timeout lets almost nothing succeed. Events therefore cost a design with little headroom more than in v4. The v5 balance test keeps v2's shape without events: 1.5× headroom earns the most, and 5× gives up at least 25%. With events, 1.5× still beats 5×.
* An out-of-memory crash restarts itself; the `restart` response only applies to crash events.
* Below capacity a full backlog only caps the wait; it does not turn requests away (Erlang loss is not modelled).
* Network latency, connection pools to the database, startup and restart knobs, and per-request sampling are left for later slices.

## Alternatives Considered

* **Free vCPU and memory knobs** — rejected; they break the priced economy.
* **Sampled request objects** — heavier, noisier, and harder to check by hand (Principle 3).
* **Framework emulation (ASGI vs WSGI, Gunicorn vs Uvicorn rules)** — little behavior beyond sync versus async and worker counts, which the presets already set.
* **A per-endpoint vector through the solver now** — not needed until a component distinguishes endpoints within a class.

## References

* [ADR-0016](0016-configurable-internet-traffic.md), [`docs/architecture.md`](../architecture.md#application-instance-model)
* `backend/internal/sandbox/app.go`, `app_test.go`
