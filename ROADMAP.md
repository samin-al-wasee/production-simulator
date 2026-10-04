# ForgeLab Roadmap

This document is the source of truth for **what is authorized and what is next**. A feature may only be implemented when the phase it belongs to is active.

Legend: ⬜ planned · 🔨 in progress · ✅ done · 🗄️ retired

ForgeLab is the **Production Sandbox** ([ADR-0014](docs/decisions/0014-sandbox-only-platform.md)). Phases 0 to 9 built a real-infrastructure lab (Live mode); most of it was retired and is kept on the `archive/live-lab` branch. What survived is noted below.

## Phase 13 — Accounts & product foundation *🔨 in progress*

> ForgeLab becomes a product: real users, persistent work, and a place to return to.
> Identity and an application datastore wrap the deterministic core without touching it.
> The core stays headless and DB-free. ADR-0030, ADR-0031.

- [ ] **13.1 Accounts & foundation**: OAuth (GitHub, Google) sign-in, DB-backed sessions,
  a Postgres application store, user-owned sandboxes with save/resume, per-user learning
  progress, and a My ForgeLab shell. The global `.forgelab` files are retired.
  (ADR-0030, ADR-0031)
  - [x] 13.1a Database foundation
  - [ ] 13.1b Identity (OAuth, sessions, login)
  - [ ] 13.1c Ownership, persistence, and per-user progress
  - [ ] 13.1d My ForgeLab shell
- [ ] **13.2 Free sandbox polish** — planned, not started
- [ ] **13.3 Goals & progression** — planned, not started

## Phase 11 — Deep component simulation *✅ done*

> Each component becomes progressively realistic, one at a time, so players learn production engineering by experimenting: configuration inside the component, behavior in the Go core, metrics out. The canvas stays simple: one node per infrastructure concept. ADR-0016.
>
> Every slice ships the same way: an ADR, a new ruleset version (older saves replay bit for bit, pinned by `testdata/replay.golden`), the model in the Go core with tests, API fields, a configuration form, inspector panel, and inside view in the dashboard, browser tests, and the docs.

- [x] Internet: traffic groups, endpoint mix and request classes, regions, client retries, and load tests with patterns (constant, ramp, spike, burst, periodic, schedule); the flow solver routes per request class; traffic breakdown; Internet configuration form (ruleset `sandbox/v4`, ADR-0016)
- [x] Application instance: a backend service whose capacity emerges from CPU, workers or concurrency slots, connections, and network under its routes' costs; middleware; backlog, timeouts, rejection, out-of-memory crashes, derived health; per-route metrics; configuration form (ruleset `sandbox/v5`, ADR-0017)
- [x] Traffic components: the Internet becomes placeable Traffic, one client population each; an empty start; the traffic-to-application contract (protocol, port, TLS, routes, keep-alive, timeout); per-application endpoint mixes; targeted traffic events; aggregated meters (ruleset `sandbox/v6`, ADR-0018)
- [x] App templates: an application type (routes with typical shares) on a stack; `place` with a configuration
- [x] **11.4 Connections and inter-service communication** (ruleset `sandbox/v7`, ADR-0019)
  - Every component has a **listener** (protocol, port, TLS) and every connection a **client** side (pool, timeout, retries) that adopts the listener on connect and follows it, generalizing the traffic contract to every edge; mismatches fail with a reason.
  - Wire protocols per kind: HTTP/1.1, HTTP/2, gRPC between services; SQL (Postgres wire) to databases; RESP to caches; S3 over HTTPS to storage; AMQP to queues; Kafka to event streams.
  - Routes declare **calls** instead of dependency kinds: a cache, database read or write, storage, queue publish, or **another service's endpoint** (microservices), each synchronous or asynchronous, with its own timeout and retries.
  - The solver carries **per-endpoint load** to every application, so a service receives exactly the endpoints its callers call; per-endpoint success and latency flow back to callers.
  - Connection pools per call target: a pool smaller than the concurrent calls becomes a bottleneck; a network hop adds latency.
  - Per-connection stats: rate, errors by reason, latency. Inside views show calls by target.
- [x] **11.5 Database** (ruleset `sandbox/v8`, ADR-0020): primary and read replica as modelled data stores. Resources from the size (vCPU, buffer-pool memory, disk IOPS); max connections against the callers' pools; read and write query profiles (CPU, pages, indexed or not); buffer-cache hit ratio from working set against memory, with data growing with users; lock contention on hot rows; replication applies every write on each replica, with lag; health, bottleneck, and a configuration form and inside view.
- [x] **11.6 Cache** (ruleset `sandbox/v9`, ADR-0021): memory size, key count and value size, TTL, eviction (LRU, LFU, none); hit ratio from the working set that fits and the TTL; misses read through to the database; a cold cache after a restart warms over ticks; stampede on mass expiry; max connections and network; form and inside view.
- [x] **11.7 Object storage** (ruleset `sandbox/v10`, ADR-0022): request-rate limits per prefix, bandwidth, latency from object size (first byte plus transfer), storage classes, stored GB and egress priced; form and inside view.
- [x] **11.8 Message queue and background worker** (ruleset `sandbox/v11`, ADR-0023): queues with message types, max backlog, visibility timeout, acknowledgements, redelivery, and a dead-letter queue; workers as consumers with concurrency, per-message cost, and their own calls (like routes); end-to-end delay; forms and inside views.
- [x] **11.9 Event stream and event-driven architecture** (ruleset `sandbox/v12`, ADR-0024): a new event stream component with topics, partitions, retention, and consumer groups; routes and workers publish events, services and workers subscribe; partition throughput and per-group lag; ordering per partition; fan-out to many consumers; form and inside view.
- [x] **11.10 Edge: load balancer, API gateway, CDN** (ruleset `sandbox/v13`, ADR-0025): back in the catalog with their contracts. Load balancer: algorithm (round robin, least connections, weighted), health checks that pull failing targets, TLS termination, connection limits, sticky sessions; it can also front internal services. API gateway: routes by path to different services, authentication, rate limits per client type, request size limits. CDN: edge cache for cacheable endpoints, TTL, hit ratio from the endpoint mix, origin offload, purge. Traffic connects to any of them; forms and inside views.

## Phase 12 — Observability *✅ done*

> Seeing inside the system is something the player builds and pays for, as in production. Uninstrumented components show only whether they are up; detail appears where the player configured it, at the resolution and sampling they chose, through telemetry backends that have capacity and cost. ADR-0027.

- [x] **12.1 Telemetry is configured, not given** (ruleset `sandbox/v14`, ADR-0027): a metrics store, a log store, and a tracing backend as components, with ingest capacity, retention, and cost; per-component instrumentation (metrics on or off and resolution, log level and sampling, trace sampling) that costs CPU on the component and ingest at the backend; dropped telemetry when a backend saturates. The dashboard shows business numbers always and technical metrics only where instrumented; older rulesets keep showing everything.
- [x] **12.2 Explaining failures**: every failed and slow request attributed to a reason and a place (refused, not found, rejected, timed out, dependency failed, handler error, out of memory, rate limited) and aggregated from the model; a metrics explorer per component and connection over history; a trace view (span waterfall) of each route across services from the model's latencies; log records as aggregated, labelled events (`504 upstream timeout from db-primary-1 × 37/s`). Each needs the matching telemetry from 12.1.
- [x] **12.3 Alerts, SLOs, and incidents**: alert rules on any collected metric with thresholds and durations, firing and resolved history; SLOs per route or service with error budgets and burn rate; an incident timeline that lines up events, alerts, and the player's commands.

## Phase 10 — Production Sandbox *✅ done*

> A city-builder for software production: the player starts from an empty world, places and wires every component in the dashboard, and runs the system as a business under growth, events, and incidents. Fully model-driven; deterministic Go core; the dashboard renders and sends commands. ADR-0013, ADR-0014.

- [x] Documentation and ADR-0013 (architecture, principles, catalog, vision)
- [x] Sandbox engine: world, component catalog, traffic model, flow solver, economy, meters, deterministic ticks (`backend/internal/sandbox`)
- [x] Sandbox API: games, commands, SSE tick stream, save/load (`/api/v1/sandbox/`)
- [x] Dashboard Sandbox screen: build palette, React Flow canvas, HUD, inspector, speed controls, browser tests
- [x] Retire Live mode; the Sandbox becomes the whole product (ADR-0014)
- [x] Event deck and incident response actions (surges, outages, DDoS, failover, rate limiting), with economy rebalancing (ruleset `sandbox/v2`)
- [x] Goals and unlocks (missions tied to the learning path, with automatic completion; ruleset `sandbox/v3`)

## History: phases 0 to 9

| Phase | What it built | Now |
|---|---|---|
| 0 — Foundation | Repository, docs, conventions, ADR process | ✅ kept (docs re-scoped) |
| 0.5 — Simulation Foundation | Host calibration, resource virtualization, dual metrics | 🗄️ retired |
| 1 — Local single-node | Docker Compose lab, sample application, manifest validation | 🗄️ retired |
| 2 — Observability | Prometheus, Grafana, Loki, Tempo, OpenTelemetry | 🗄️ retired |
| 3 — Messaging & data | Redis, RabbitMQ, Kafka, retries, dead-letter queues | 🗄️ retired |
| 4 — Kubernetes | kind cluster, ingress, HPA, rollout scenarios | 🗄️ retired |
| 5 — Reliability & chaos | Chaos engine, outage drills, load generator, backup/restore | 🗄️ retired |
| 6 — Cloud presets | AWS and GCP Terraform presets, cost guard | 🗄️ retired |
| 7 — CI/CD & dashboard | Pipeline simulator; Next.js dashboard | ✅ pipeline simulator and dashboard kept; experiment pages retired |
| 8 — Security | Security overlay, hardening, compliance checks, secret scan | ✅ secret scan kept; the rest retired |
| 9 — Production Simulator | End-to-end run, benchmarks, learning path | ✅ learning path kept (rewritten as Sandbox missions); the rest retired |

---

## How phases are advanced

1. Documentation for the phase lands first (architecture + catalog updates).
2. Architectural changes get an ADR in `docs/decisions/`.
3. Implementation lands incrementally, small and reviewable.
4. Each phase ends with a `CHANGELOG.md` entry.
