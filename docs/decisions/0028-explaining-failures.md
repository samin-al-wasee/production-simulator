# ADR-0028: Explaining failures: causes, metrics history, traces, and logs

**Status:** accepted
**Date:** 2026-10-02

## Context

With telemetry configured ([ADR-0027](0027-configured-telemetry.md)) the player sees a component's numbers when it is monitored. Numbers say *that* something is wrong, not *why*: an error rate does not say whether requests were refused by a database, rejected by a full backlog, failed by a handler, or lost to a broken connection, nor where a slow request spends its time. The owner asked to be able to understand what happened and why, with the detail available only where it was configured.

## Decision

1. **A report per tick** (`Report`), derived from the model after the solve. It changes nothing in the simulation and is not part of the replayed state, so rulesets keep replaying exactly; the API returns it as `report`.
2. **Causes:** every failure per second is attributed to a reason and a place.
   * Application: rejected (backlog or rate limit), timed out, handler error, not found (no route), missing dependency (nothing connected for a call), dependency failed (naming the connections whose calls failed), down or crashed.
   * Database or cache: too many connections, overloaded. Storage and streams: throttled. Gateway: not found, rate limited. Queue: full. Any other component: overloaded.
   * Connections: a broken contract, on the edge. Traffic: not connected. Outside: a third-party outage.
   * Asynchronous losses (dead letters, events lost past the retention) are listed apart: no request fails for them.

   A cause is **seen** when its place logs errors to a log store that keeps them; the rest is counted as unattributed.
3. **Metrics history:** each component whose metrics a store keeps gets one sample per tick (requests, errors, latency, utilization), up to two simulated days.
4. **Traces:** for every endpoint every traffic component asks for, a span tree through each hop: an application's queue wait, its handler, and each call in order (a service call nests the callee's route; a dependency is a leaf with its latency; async calls are marked), an edge component forwarding to its main target or answering a hit. Durations are the model's mean latencies, and each span carries its chance of success. A trace is **seen** when the component traffic reaches first samples traces into a trace backend; its rate is the sampled requests per second.
5. **Logs:** aggregated, labelled lines each component writes at its level, kept by a log store: errors from its causes, warnings for routes slower than half the SLO, an info line per route, a debug line per connection, each with a rate.
6. **The dashboard** adds an **Observe** panel (v14): Failures, Metrics (charts from zero), Traces (a waterfall), and Logs (filtered by level), each showing only what was seen.

## Consequences

### Positive

* A failure can be followed from its symptom to its cause, and a slow request through its spans, with the telemetry the player chose to pay for.
* Missing error logs at the failing place show as unattributed failures, which is itself a lesson.

### Negative / Trade-offs

* Traces show mean latencies and the main path of each split, not individual sampled requests.
* Logs are aggregated by message, not listed line by line.

## Alternatives Considered

* **Sampling individual requests to build traces and logs** — noisy and heavy, and not checkable by hand (Principle 3).
* **A new ruleset** — the report changes no outcome, so v14 keeps it.

## References

* [ADR-0027](0027-configured-telemetry.md), [`docs/architecture.md`](../architecture.md#explaining-failures)
* `backend/internal/sandbox/report.go`, `report_test.go`
