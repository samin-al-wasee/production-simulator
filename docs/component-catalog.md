# ForgeLab Component Catalog

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

Every part of ForgeLab, grouped by domain, with its status. A component is real when it has a README under `components/<domain>/<component>/`. Rules for adding one: `AGENTS.md` §9.

The Live-mode components (reverse proxy, PostgreSQL, Redis, RabbitMQ, Kafka, observability stack, Kubernetes, Terraform presets, chaos and load tooling, and others) were retired by ADR-0014; they are on the `archive/live-lab` branch.

## Catalog by domain

### Sandbox

The Production Sandbox game (ADR-0013). See [`components/sandbox/`](../components/sandbox/README.md).

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Sandbox Engine | Deterministic world state, command log, tick loop, save/replay (`core/internal/sandbox`) | — | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Sandbox Ruleset | Versioned data: placeable component kinds, capacities, costs, complexity weights, tuning (`sandbox/v1`; `sandbox/v2` adds the Event Deck and rebalances the economy) | Sandbox Engine | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Flow Solver | Routes per-tick load through the player's topology; utilization, latency, saturation, errors | Sandbox Engine, Sandbox Ruleset | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Economy & Meters | Revenue, cost, cash; health, satisfaction, popularity, engagement, complexity, scale, userbase | Flow Solver | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Sandbox API | `/api/v1/sandbox/` games, commands, speed, step, save/replay, SSE tick stream (`core/internal/api`) | Sandbox Engine | implemented · Phase 10 (`components/sandbox/sandbox-api/`) |
| Sandbox Canvas | Dashboard screen: build palette, React Flow topology canvas, meters, inspector, speed controls | Sandbox API | implemented · Phase 10 (`components/sandbox/sandbox-canvas/`) |
| Event Deck | Seeded, state-dependent events and incidents (eleven cards: surges, crashes, zone outage, slowdowns, DDoS, cost spikes, and more) and responses (restart, failover, rate limit) | Sandbox Engine | implemented · Phase 10 (`components/sandbox/sandbox-engine/`) |
| Goals & Unlocks | Missions with targets, unlocks, and automatic learning-path completion | Sandbox Engine, Learning Tracker | planned · Phase 10 |

#### In-game component kinds (rulesets `sandbox/v1` and `sandbox/v2`)

These are what a player places in a game. They are ruleset data, not repository components; capacities and costs are for the `small` size and one replica.

| Kind | Role in the model | Capacity (ops/s) | Cost / h |
|---|---|---|---|
| CDN | Serves 30% of requests at the edge, forwards the rest | 2000 | $3.00 |
| Load balancer | Splits load across targets by capacity; skips failed targets | 5000 | $1.50 |
| API gateway | Forwards load to balancers or application instances; can rate-limit attack traffic (v2) | 2000 | $2.00 |
| Application instance | Serves requests; sends reads, writes, and storage calls downstream | 50 | $2.00 |
| Background worker | Drains a queue and writes to the database | 40 | $1.50 |
| Database primary | Serves reads and writes | 300 | $4.00 |
| Database read replica | Serves reads | 300 | $3.50 |
| Cache | Serves 80% of reads, sends misses to the database | 5000 | $2.00 |
| Message queue | Accepts writes, holds a backlog, feeds workers | 1000 | $1.50 |
| Object storage | Serves the 10% of requests that need stored objects | 1000 | $1.00 |

### CI/CD

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Pipeline Runner | Build → test → deploy on a virtual clock; rolling, canary, blue-green, rollback (`core/internal/pipeline`) | — | implemented · Phase 7 (`components/cicd/pipeline-runner/`) |

### Experience

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Learning Tracker | Learning path of Sandbox and pipeline missions, with progress (`core/internal/learning`) | — | implemented · Phase 9, re-scoped (`components/experience/learning-tracker/`) |

### Security

| Component | Provides | Depends on | Status |
|---|---|---|---|
| Scanner | Committed-secret detection (`core/internal/secretscan`) | — | implemented · Phase 8, re-scoped (`components/security/scanner/`) |

---

## Status legend

| Status | Meaning |
|---|---|
| `planned` | Documented here; not yet implemented |
| `implemented` | Implemented, with a README under `components/<domain>/<component>/` |

## Adding a component

Follow `AGENTS.md` §9: check this catalog, confirm a roadmap phase authorizes it, add its README, update this table, and write an ADR if it changes the architecture.
