# ForgeLab Component Catalog

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

Every part of ForgeLab, grouped by domain, with its status. A component is real when it has a README under `components/<domain>/<component>/`. Rules for adding one: `AGENTS.md` §9.

The Live-mode components (reverse proxy, PostgreSQL, Redis, RabbitMQ, Kafka, observability stack, Kubernetes, Terraform presets, chaos and load tooling, and others) were retired by ADR-0014; they are on the `archive/live-lab` branch.

## Catalog by domain

### Sandbox

The Production Sandbox game (ADR-0013). See [`components/sandbox/`](../components/sandbox/README.md).

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Sandbox Engine | Deterministic world state, command log, tick loop, save/replay (`backend/internal/sandbox`) | — | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Sandbox Ruleset | Versioned data: placeable component kinds, capacities, costs, complexity weights, tuning (`sandbox/v1`; `sandbox/v2` adds the Event Deck and rebalances the economy; `sandbox/v3` adds goals and unlocks; `sandbox/v4` adds the Internet's traffic configuration; `sandbox/v5` adds the application instance model; `sandbox/v6` replaces the Internet with traffic components) | Sandbox Engine | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Traffic Model | The Internet's configuration (ADR-0016): market or load-test volume with a pattern, traffic groups, endpoint mix, regions, and client retries; a per-tick traffic breakdown | Sandbox Engine, Sandbox Ruleset | implemented · Phase 11 (`components/sandbox/sandbox-engine/`) |
| Traffic Components | Traffic as placeable components (ADR-0018): one client population each (client type, region, protocol, scheme, port, keep-alive, timeout, retries, endpoint mix, market or load test); the traffic-to-application contract; aggregated meters | Sandbox Engine, Sandbox Ruleset | implemented · Phase 11 (`components/sandbox/sandbox-engine/`) |
| Database Model | Primary and read replica as data stores (ADR-0020): query profiles, buffer cache over data that grows with users, CPU, IOPS, row locks, max connections, replication with lag, derived health | Flow Solver, Connections | implemented · Phase 11 (`components/sandbox/sandbox-engine/`) |
| Connections & Service Calls | Listeners and client-side connections on every edge (protocol, port, TLS, pool, timeout, retries) with a contract; routes calling other services by name, sync or async; per-endpoint load; per-connection stats (ADR-0019) | Flow Solver, Application Model | implemented · Phase 11 (`components/sandbox/sandbox-engine/`) |
| Application Model | An application instance as a backend service (ADR-0017): routes, middleware, sync/async workers, CPU, memory, connections, network, backlog, timeouts, out-of-memory crashes, derived health | Flow Solver, Traffic Model | implemented · Phase 11 (`components/sandbox/sandbox-engine/`) |
| Flow Solver | Routes per-tick load through the player's topology, per request class (cacheable read, read, write); utilization, latency, saturation, errors | Sandbox Engine, Sandbox Ruleset, Traffic Model | implemented · Phase 10, classes in Phase 11 (`components/sandbox/sandbox-engine/`) |
| Economy & Meters | Revenue, cost, cash; health, satisfaction, popularity, engagement, complexity, scale, userbase | Flow Solver | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Sandbox API | `/api/v1/sandbox/` games, commands, speed, step, save/replay, SSE tick stream (`backend/internal/api`) | Sandbox Engine | implemented · Phase 10 (`components/sandbox/sandbox-api/`) |
| Sandbox Canvas | Dashboard screen: build palette, React Flow topology canvas, meters, inspector, speed controls | Sandbox API | implemented · Phase 10 (`components/sandbox/sandbox-canvas/`) |
| Event Deck | Seeded, state-dependent events and incidents (eleven cards: surges, crashes, zone outage, slowdowns, DDoS, cost spikes, and more) and responses (restart, failover, rate limit) | Sandbox Engine | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Goals & Unlocks | Fifteen goals checked each tick, kinds unlocked by reaching them, and automatic learning-path completion | Sandbox Engine, Learning Tracker | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |

#### In-game component kinds (rulesets `sandbox/v1` to `sandbox/v5`)

These are what a player places in a game, plus the Internet, which every game has and which cannot be placed or removed. In `sandbox/v4` the Internet is configurable: traffic groups, endpoints, regions, retries, and load tests live inside it, not on the canvas. They are ruleset data, not repository components; capacities and costs are for the `small` size and one replica. In `sandbox/v3` some kinds are locked until a goal is reached (see [architecture](architecture.md#goals-and-unlocks)).

| Kind | Role in the model | Capacity (ops/s) | Cost / h |
|---|---|---|---|
| CDN | Serves 30% of requests at the edge (v1 to v3), or 45% of cacheable reads (v4); forwards the rest | 2000 | $3.00 |
| Load balancer | Splits load across targets by capacity; skips failed targets | 5000 | $1.50 |
| API gateway | Forwards load to balancers or application instances; can rate-limit attack traffic (v2) | 2000 | $2.00 |
| Application instance | Serves requests; sends reads, writes, and storage calls downstream, in the shares the request mix sets. In v5 a backend service whose capacity emerges from its size (1 vCPU / 1 GB small), workers, and routes: about 56 ops/s at the defaults | 50 (v1–v4) | $2.00 |
| Background worker | Drains a queue and writes to the database | 40 | $1.50 |
| Database primary | Serves reads and writes | 300 | $4.00 |
| Database read replica | Serves reads | 300 | $3.50 |
| Cache | Serves 80% of reads, sends misses to the database | 5000 | $2.00 |
| Message queue | Accepts writes, holds a backlog, feeds workers | 1000 | $1.50 |
| Object storage | Serves the requests that need stored objects (10% in v1 to v3; the storage endpoints in v4) | 1000 | $1.00 |

#### In-game component kinds (ruleset `sandbox/v6`)

A v6 game starts with no node at all. Its catalog is v5's without the Internet, the CDN, the load balancer, and the API gateway, which return once they have a traffic contract, plus:

| Kind | Role in the model | Capacity (ops/s) | Cost / h |
|---|---|---|---|
| Traffic | One population of clients: its client type and region decide its share of the market (or a load test sets its rate). Asks for nothing until it connects to exactly one application instance, whose protocol, port, scheme, keep-alive, and routes it then adopts, and follows when that app is reconfigured; disconnected, it asks for nothing again; the contract is in [architecture](architecture.md#traffic-components). Free to place and run, with no size or replicas | — | $0.00 |

### CI/CD

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Pipeline Runner | Build → test → deploy on a virtual clock; rolling, canary, blue-green, rollback (`backend/internal/pipeline`) | — | implemented · Phase 7 (`components/cicd/pipeline-runner/`) |

### Experience

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Learning Tracker | Learning path of Sandbox and pipeline missions, with progress; completed by Sandbox goals or by hand (`backend/internal/learning`) | — | implemented · Phase 9, re-scoped (`components/experience/learning-tracker/`) |

### Security

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Scanner | Committed-secret detection (`backend/internal/secretscan`) | — | implemented · Phase 8, re-scoped (`components/security/scanner/`) |

---

## Status legend

| Status | Meaning |
|---|---|
| `planned` | Documented here; not yet implemented |
| `implemented` | Implemented, with a README under `components/<domain>/<component>/` |

## Adding a component

Follow `AGENTS.md` §9: check this catalog, confirm a roadmap phase authorizes it, add its README, update this table, and write an ADR if it changes the architecture.
