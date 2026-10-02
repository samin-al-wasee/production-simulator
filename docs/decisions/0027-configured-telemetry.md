# ADR-0027: Telemetry is configured, not given

**Status:** accepted
**Date:** 2026-10-02

## Context

Every value the game computes has always been on screen: each component's load, latency, errors, and internals, and the system's RPS, p95, and error rate. Production is not like that. An engineer sees what the system reports, at the resolution and sampling configured, through telemetry backends that cost money and can fall over. The project owner asked for observability that is more granular and more insightful, and explicitly not shown by default: it must be configured carefully to monitor and observe the components and the whole system.

## Decision

1. **Telemetry backends** are components from `sandbox/v14`: a **metrics store**, a **log store**, and a **trace backend**. Each has an ingest capacity per replica that scales with size (20,000 samples, 3,000 lines, or 5,000 spans per second on small), a price per replica-hour, and a price per GB ingested ($0.30, $0.50, $0.30). Nothing connects to them; every instrumented component ships to the backends of each signal, split by capacity.
2. **Instrumentation per component** (`Telemetry`, `configure` with `telemetry`), **off by default**:
   * metrics on or off, and their resolution (10–300 s): `series × replicas ÷ resolution` samples per second, series by kind (an application 40, a database 50, …);
   * log level (off, error, warn, info, debug) and sampling: error logs failed requests, warn adds 5% of the rest, info one line per request plus failures, debug six;
   * trace sampling: `sampled requests × (1 + outgoing connections)` spans per second.
3. **Reporting has a cost:** on applications and workers each log line costs 0.02 CPU-ms and each span 0.05 CPU-ms per request, which the application model charges; every signal costs ingest at its backend.
4. **Saturation drops data:** a backend that receives more than it can take stores its capacity and drops the rest; each component's coverage of a signal is the share kept, and is 0 when there is no backend for it.
5. **What the player sees** (`NodeStats.obs`, `Meters.monitored`):
   * business numbers (cash, revenue, cost, users, satisfaction, popularity, complexity) always;
   * a component's live numbers, panels, and inside view only while its metrics reach a store;
   * the system's RPS, p95, errors, and health only while a component that traffic reaches first is monitored;
   * every component's configuration, whether it is down, and its bill, always.

   The engine still computes everything; observation decides what the dashboard shows. Games of v13 and earlier show everything, as before.
6. **Ruleset `sandbox/v14`.** v1 to v13 replay bit for bit.

## Consequences

### Positive

* Monitoring becomes a design decision with a price: what to instrument, at what resolution, with what sampling, and how big the backends must be.
* A saturated log store during an incident is visible as dropped telemetry, as it is in production.
* Later slices (failure attribution, metrics history, traces, logs, alerts) build on what is reported, not on omniscience.

### Negative / Trade-offs

* A new game starts dark: the player must learn to place a metrics store and turn on metrics before the meters say anything technical.
* The API still returns the full model; the dashboard hides what is not observed. A player reading the API directly sees everything.
* Retention and query cost are folded into the ingest price.

## Alternatives Considered

* **Hiding unobserved values in the API** — the engine's tests and replays need the full model; the observed view is a presentation of it, decided by the engine's `obs` and `monitored`.
* **Telemetry edges on the canvas** — every component connected to three backends would bury the topology.

## References

* [ADR-0019](0019-connections-and-service-calls.md), [`docs/architecture.md`](../architecture.md#telemetry-model)
* `backend/internal/sandbox/telemetry.go`, `telemetry_test.go`
