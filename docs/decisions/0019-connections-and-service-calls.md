# ADR-0019: Connections and inter-service communication

**Status:** accepted
**Date:** 2026-10-02

## Context

Up to `sandbox/v6` only the traffic-to-application connection has a contract ([ADR-0018](0018-traffic-components.md)). Every other edge is a bare arrow: an application reaches its cache, database, queue, and storage by kind, with no protocol, port, pool, timeout, or retries, and applications cannot call each other. A learner cannot build microservices, see a call chain slow down, run out of connections to a database, or break a connection by misconfiguring it. The project owner asked for service-to-service communication over different protocols, at the granularity of the traffic and application slices, as the foundation for the remaining components and for observability.

## Decision

1. **Listeners.** Every kind that takes connections has a listener (protocol, port, TLS) in the ruleset: databases SQL on 5432, caches RESP on 6379, object storage S3 over TLS on 443, queues AMQP on 5672. An application's listener is its own configuration. A node may override its kind's listener with `configure` (`listener`).
2. **Connections.** Every edge to a listening target has a client side (`Connection`: protocol, port, TLS, pool per caller replica, timeout, retries). Connecting adopts the target's listener with the ruleset's defaults (pool 20, timeout 1 s, no retries). Changing the target's listener or an application's configuration makes every connection to it follow, keeping its pool, timeout, and retries. `configure` with `from`, `to`, and `connection` changes it.
3. **The contract on every edge.** A protocol, port, or TLS mismatch refuses every call over that edge with a reason. Refused calls never reach the target; they fail the requests that needed them. Traffic keeps its own contract.
4. **Calls to services.** A route may call other services' endpoints (`calls`: service name, endpoint, async). A call reaches the connected application instances whose name is the service's (service discovery by name), split by capacity. Applications may connect to applications; the topology stays acyclic.
   * A **synchronous** call's success and latency join the route's, as dependencies do.
   * An **asynchronous** call is sent and not waited for: it loads the service, but costs the caller only the network hop and never fails it.
   * A call to a service that is not connected fails the route; an async one is dropped.
5. **Per-endpoint load.** The solver carries each application's load per endpoint, from traffic and from callers, so a service receives exactly the endpoints its callers call. Each application's mix comes from that load; with none it is measured at its own routes' typical shares. Per-endpoint success and latency flow back to callers. This replaces the class split of ADR-0017 item 10 and the input mix of ADR-0018 item 7.
6. **A call's outcome.** Over a connection, a call to a target that succeeds with chance `p` at mean latency `t` costs `t + hop` (0.5 ms) and succeeds with `p × (1 − exp(−timeout ÷ (t + hop)))`, latency taken as exponential as elsewhere. With `r` retries it succeeds with `1 − (1 − p_attempt)^(r+1)` and sends `1 + f + … + f^r` attempts, `f` being the connection's attempt failure rate on the previous tick. Waits use the previous tick's latency, as dependencies did.
7. **Pools.** A call holds a connection for its latency. Each connection adds a limit to its caller: `pool × replicas ÷ (connection seconds per request)`, reported as the bottleneck `pool:<target>`. A small pool to a slow target limits the caller although its CPU is idle.
8. **Keep-alive.** Calls from other services arrive over pooled connections and count as kept alive.
9. **Reported.** The flow carries `edges`: per connection, attempts, retry attempts, failed attempts, mean latency, and the contract problem. Edges from components without the application model report the load they carried and how their target fared.
10. **Templates.** v7 adds microservice application types: a Storefront calling Catalog, Orders, and Payments, and Orders notifying a Notifications service asynchronously.
11. **Ruleset `sandbox/v7`.** Rulesets v1 to v6 replay bit for bit, pinned by `testdata/replay.golden`.

## Consequences

### Positive

* Microservices and call chains are buildable: latency adds up across services, a slow or failing service degrades its callers, and async calls decouple them.
* Connection pools, timeouts, and retries become design decisions with visible trade-offs: retries recover transient failures and amplify load.
* Every edge reports what it carried, which later observability slices build on.

### Negative / Trade-offs

* A target's capacity for splitting load among replicas of a service is measured at the mix it already has this tick, or at its routes' typical shares before any arrives.
* Retries and waits react with a tick of delay, by design.
* Request latency is taken as exponential when a timeout cuts it, the same simplification as the application's queue.

## Alternatives Considered

* **Calls addressed by node ID** — rejected; templates and replicas need names, as service discovery does.
* **Per-call timeouts and retries on the route** — the connection is where clients set them, and it keeps routes short.
* **Sampled requests to trace calls** — heavier and noisier than per-endpoint rates (Principle 3).

## References

* [ADR-0017](0017-application-instance-model.md), [ADR-0018](0018-traffic-components.md), [`docs/architecture.md`](../architecture.md#connections-and-service-calls)
* `backend/internal/sandbox/conn.go`, `conn_test.go`, `templates.go`
