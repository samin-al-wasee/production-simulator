# ADR-0018: Traffic components and the traffic-to-application contract

**Status:** accepted
**Date:** 2026-10-01
**Supersedes:** parts of [ADR-0016](0016-configurable-internet-traffic.md) (one Internet node holding traffic groups) and item 10 of [ADR-0017](0017-application-instance-model.md) (one global endpoint mix)

## Context

Up to `sandbox/v5` every game starts with one fixed **Internet** node. It holds every client population as traffic groups, and the player cannot remove it. Its requests reach an application whatever the two sides declare: the application's protocol and port are labels, and any endpoint without a route falls to a required catch-all.

The project owner asked for three changes:
* traffic becomes an ordinary component, **Traffic**, that the player places, any number of times, on a canvas that starts **empty**
* each traffic component is **one** client population, with a single choice for each option (client type, region, and the rest)
* a traffic component connects to **exactly one** component, for now only an application instance, under a **contract**: each side declares what it sends or accepts, and a mismatch makes requests fail

The owner also set one rule for everything shown: values on the canvas and overall meters are **aggregated across every traffic component**.

## Decision

1. **A new kind, `traffic`,** replaces the Internet in ruleset `sandbox/v6`. A new v6 game has no nodes. A traffic component costs nothing to place or run, has no size or replicas, and is never the target of a component event.
2. **One population per component.** Its configuration (`ClientConfig`, set with `configure`) holds single choices:
   * a label, the **client type** (web, mobile, API client, bot), and the **region**
   * the **protocol** (HTTP/1.1, HTTP/2, gRPC), the **scheme** (http, https), and the target **port**
   * client **keep-alive**, client **timeout**, and **retries**
   * the **source**: `market`, or a load test with a pattern as in ADR-0016
   * a weighted **endpoint mix** (`"GET /products"` and its share), the one list in the configuration
3. **Connections.** A traffic component has at most one outgoing connection, and in v6 it may only go to an application instance. Many traffic components may feed one application. Nothing connects to a traffic component.
4. **A new component asks for nothing; connecting adopts the application.** A new traffic component has no endpoints and the ruleset's defaults for the rest. Connecting it to an application, as part of the logged `connect` command, sets its protocol, port, scheme (https when the app has TLS), and keep-alive to the application's, and its endpoints to one per route, the catch-all aside, in equal shares kept to hundredths of a percent. The client type, region, timeout, retries, and source stay the client's. Reconfiguring the application adopts again for every traffic component connected to it, replacing their connection settings and endpoints. Losing the connection (disconnecting, or removing the application) returns a component to the ruleset's connection settings and no endpoints, keeping who its clients are. The player can change any of it while connected, so a mismatch is something the player makes on the traffic side, and it shows.
5. **Market volume per segment.** The market's volume, `active users × engagement × diurnal(t)`, is split into segments by client-type share (web 70%, mobile 20%, API 8%, bot 2%) and region share (asia 35%, europe 25%, north America 20%, south America 10%, Africa 5%, Oceania 5%). A market component takes `volume × type share × region share × traffic events on it`. Components of the same segment split it evenly. A segment with no component is demand the player has not captured: it earns nothing and is not served.
6. **The contract.** Checked every tick on each traffic → application connection:

   | Traffic | Application | Mismatch |
   |---|---|---|
   | protocol | `protocol` | every request fails: protocol error |
   | port | `port` | every request fails: connection refused |
   | scheme | `tls` | every request fails: TLS handshake error |
   | each endpoint | its route, or the `*` route | that endpoint fails with 404 |
   | keep-alive | `keepAlive` | connections are reused only when both sides keep them alive |
   | timeout | `timeoutMs` | the shorter one decides success |

   Requests refused at the door (protocol, port, scheme) never reach the application's capacity. A 404 still costs the middleware's time and CPU. A request whose client gave up first still loads the server and its dependencies. In v6 the `*` route is optional, and the application's protocol and port become model inputs.
7. **Each application builds its endpoint mix from its inputs,** weighted by their attempts this tick. The global mix of ADR-0017 item 10 stays for v5 and earlier. An application with no input is measured at its own routes in equal shares, so its capacity is shown before traffic arrives.
8. **Retries per component.** A component with `N` retries sends `load × (1 + f + … + f^N)` attempts, where `f` is that component's attempt failure rate on the previous tick.
9. **Events target components.** Traffic cards (viral surge, marketing spike, seasonal dip) and the DDoS pick one or more traffic components, drawn by the seed, and act only on them. Component cards keep their targets and never hit a traffic component.
10. **Aggregated meters.** RPS, success, errors, p95, retry RPS, concurrency, revenue, and the userbase model are totals or request-weighted means over every traffic component. The flow's `traffic` adds RPS by client type, region, endpoint, and component. Each traffic node also reports its own `traffic` stats: RPS, retries, successes, failures by reason, latency, concurrency, and the contract problem. An application reports RPS by input.
11. **Sentiment needs traffic.** While no traffic component sends real requests, satisfaction and popularity hold still, as during a load test, so an empty world does not grow on perfect quality it never served. The game is in a load test while any component runs one.
12. **The catalog of v6.** CDN, load balancer, and API gateway have no traffic contract yet, so v6 leaves them out of its catalog until their own slices. The *Scale out* goal becomes two or more application replicas serving; the gateway rate limit is not a response in v6, and the rate-limit middleware answers a DDoS.
13. **Replay.** Rulesets v1 to v5 keep the Internet and replay bit for bit.

## Consequences

### Positive

* Principle 4 holds literally: a new game is empty, and the player places even the traffic.
* Client populations are visible on the canvas, so the player can see which population an incident or a slowdown hits.
* Mismatched protocols, ports, TLS, and missing routes become lessons with a reason shown, instead of being impossible.
* Each application now sees its own mix, which closes the gap ADR-0017 left open.

### Negative / Trade-offs

* v6 has fewer components than v5 until the CDN, load balancer, and gateway get their contracts.
* Retries are tracked per component, not per endpoint: when only some endpoints of a component fail, retries still appear to recover part of them.
* Regions are still labels; regional latency and outages are for later slices.
* The dashboard keeps the Internet panels so v1 to v5 games still open.

## Alternatives Considered

* **Keep the Internet and add sources inside it** — rejected by the owner; one node hides the populations.
* **Block a mismatched connection** — a refused connection is the lesson (Principle 5).
* **Keep the CDN, load balancer, and gateway with their flat model between traffic and applications** — it would break the one-connection contract before they have one.

## References

* [ADR-0016](0016-configurable-internet-traffic.md), [ADR-0017](0017-application-instance-model.md), [`docs/architecture.md`](../architecture.md#traffic-components)
* `backend/internal/sandbox/client.go`, `client_test.go`
