# ADR-0025: Load balancer, API gateway, and CDN with contracts

**Status:** accepted
**Date:** 2026-10-02

## Context

[ADR-0018](0018-traffic-components.md) took the load balancer, API gateway, and CDN out of the catalog from `sandbox/v6` because they had no traffic contract: their flat models split load by request class and could not route by path, cache by endpoint, or fail in the ways the edge fails. With connections ([ADR-0019](0019-connections-and-service-calls.md)) and per-endpoint load in place, they can return as modelled components.

## Decision

1. **Back in the catalog from v13**, unlocked as before (load balancer after the first request, gateway at the startup tier, CDN at the scale-up tier). Each listens on HTTP/1.1 :443 with TLS. Traffic connects to an application, a load balancer, a gateway, or a CDN; a CDN to balancers, gateways, and applications; a balancer to applications and gateways; a gateway to applications and balancers.
2. **Traffic in front of the edge** adopts the edge component's listener, and the routes of the first application behind it; when an application is later connected behind an edge component, traffic in front of it that asks for nothing adopts that application's routes.
3. **Per-endpoint forwarding.** Each edge component receives load per endpoint and forwards it per endpoint, so the applications behind see exactly what reaches them. Each endpoint's success and latency are computed back through every hop to the traffic.
4. **Load balancer** (`lb`): algorithm and health checks.
   * Round robin gives every replica of every target the same share, whatever its size; least connections follows capacity.
   * With health checks a failed target receives nothing. Without, a round-robin balancer keeps its share, and least connections, seeing an idle target, gives it an average share; those requests fail.
5. **API gateway** (`gateway`): routes from path prefix to service name (the longest matching prefix wins; a request with no match is a 404 at the gateway), authentication (2 ms per request), and a rate limit above which requests are answered with 429. The incident response that rate-limits a gateway still works.
6. **CDN** (`cdn`): cacheable endpoints (every GET by default), TTL, distinct objects per endpoint, object size. A cacheable request is a hit with chance `1 − exp(−TTL × endpoint rate ÷ objects)`, answered in 10 ms; misses go to the origin. A CDN is a managed service, priced by use ($0.02 per GB and $0.0075 per 10,000 requests), with no size or replicas.
7. **Reported** as the node's `edge`: health, each target's rate and share and whether it is up, 404s, 429s, and a CDN's hits, misses, hit ratio, egress, and cost.
8. **Ruleset `sandbox/v13`** (least connections with health checks; a gateway routing `/` to the default app; a CDN with a 300 s TTL over 1,000 objects of 50 KB). v1 to v12 replay bit for bit.

## Consequences

### Positive

* Round robin over unequal replicas, a balancer without health checks, a gateway route that forgets a path, and a CDN that offloads reads but not writes are each visible failures with reasons.
* Gateways split one front door into many services by path, completing the microservice picture of ADR-0019.
* A CDN's cost and offload can be weighed against storage egress ([ADR-0022](0022-object-storage-model.md)).

### Negative / Trade-offs

* Sticky sessions, retries at the balancer, and request transforms are not modelled.
* The CDN has one region; per-region edges are for a later slice.
* The CDN's objects per endpoint is a declared number, not derived from the data.

## Alternatives Considered

* **Keeping the v5 flat edge** — it cannot route by path or cache by endpoint, so it cannot keep the contracts of v6 onward.
* **Health checks with an interval and thresholds** — a tick is five minutes, longer than any interval; the on/off choice keeps the lesson.

## References

* [ADR-0018](0018-traffic-components.md), [ADR-0019](0019-connections-and-service-calls.md), [`docs/architecture.md`](../architecture.md#edge-model)
* `backend/internal/sandbox/edge.go`, `edge_test.go`
