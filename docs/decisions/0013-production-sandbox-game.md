# ADR-0013: Production Sandbox — a model-driven production-system game (Phase 10)

**Status:** accepted; Live mode (decision 1) and the alternative rejected there are superseded by [ADR-0014](0014-sandbox-only-platform.md)
**Date:** 2026-09-29

## Context

After Phase 9, ForgeLab runs real infrastructure (Docker Compose, Kubernetes, cloud presets) and the dashboard is a control panel for it: launch an experiment, watch metrics, run a pipeline. That is valuable, but it has two limits:

1. **The architecture is fixed before the learner starts.** The learner receives a composed stack and breaks it; they rarely *design* one, and never watch their own design outgrow itself.
2. **Real infrastructure caps the scale of the lesson.** Even with resource virtualization, one laptop cannot show a system growing from one instance to hundreds, across a sudden surge, a zone outage, and a budget that runs out.

The next goal is a **city-builder for software production** (in the spirit of SimCity): the player opens the dashboard, starts from an **empty production**, places every component themselves — load balancers, application instances, databases, caches, queues — wires them together, and runs the system as a living business. Traffic grows and falls, events and incidents happen, and the player's choices trade off cost, revenue, health, complexity, popularity, engagement, scale, userbase, and RPS.

This cannot run on real containers: a player placing 200 application instances must not start 200 containers. It therefore needs a **modelled** simulation, which collides with Principle 10 ("never fake behavior, only virtualize capacity") and with ADR-0004, which rejected "fake the metrics without executing workloads". It also introduces a game economy (money, satisfaction, popularity), which sits close to Principles 1, 13, and 14 (no business logic in the platform).

## Decision

1. **A separate Sandbox mode.** ForgeLab gains a second way of running a production system:
   * **Live mode** — everything that exists today (Compose, Kubernetes, cloud presets, chaos, load tests). Unchanged, and still governed by Principle 10 as written.
   * **Sandbox mode** — a deterministic, model-driven simulation of a production system that the player builds from nothing in the dashboard. No containers, processes, or cloud resources are created. Sandbox is the default entry point of the dashboard; Live mode remains one click away.

2. **Modelled, not faked.** Sandbox behavior is derived from explicit capacity and queueing models, never from scripted or random metric values. Each component has a declared capacity and service time; per-tick load flows through the player's topology; utilization produces latency; saturation produces queueing, drops, and errors; errors and latency produce churn. A bottleneck in the Sandbox exists for the same reason it would in production (Principle 11 holds). Principle 10 is amended to state this scope explicitly, and a new principle requires every Sandbox value to be labelled as simulated.

3. **Everything lives in the Go core.** A new pure-Go package, `core/internal/sandbox`, owns the entire game: world state, component catalog, traffic model, flow solver, economy, meters, and event deck. It is deterministic — a game is fully defined by `(ruleset version, seed, ordered command log)` — and testable headlessly. The dashboard never computes simulation values; it renders state and sends commands (architecture rule 3).

4. **Empty start, explicit composition.** A new game contains only the traffic source (the "Internet" node) and starting cash. Every component is placed by the player; nothing is implied (Principle 2). A system with no path from the Internet to an application instance serves no traffic and earns nothing.

5. **Initial component catalog** (sizes `small`/`medium`/`large`, reusing the Resource Profile vocabulary): DNS/CDN, Load Balancer, API Gateway, Application Instance, Background Worker, Database Primary, Database Read Replica, Cache, Message Queue, Object Storage. Each kind declares capacity (RPS or ops/s), base service time, cost per simulated hour, complexity weight, and which kinds it may connect to. The catalog is data, versioned with the ruleset.

6. **Generic economy, no business logic.** The economy is abstract and domain-free:
   * **Revenue** — earned per *successful* user request (a single configurable rate per ruleset).
   * **Cost** — each component's hourly cost, charged every tick; plus an operations overhead that grows with complexity.
   * **Cash** — revenue minus cost, accumulated; the game is lost when cash stays negative past a grace period.
   * No product types, checkout flows, or domain terms enter the model (Principles 13 and 14). Feature labels may be added later purely as presentation.

7. **Meters.** Each tick the engine publishes: **RPS**, **userbase** (total and active), **p95 latency**, **error rate**, **availability**, **system health** (0–100, from error rate, latency against the SLO, and availability), **satisfaction** (0–100, the "happiness" rating, from latency, errors, and outages), **popularity** (drives new-user acquisition), **engagement** (requests per active user, raised by satisfaction), **complexity** (sum of component weights and connections; raises incident odds and operations cost), **scale** (a tier derived from userbase and peak RPS), **revenue**, **cost**, and **cash**. The feedback loop is: popularity → new users → RPS → load → latency/errors → satisfaction → churn and engagement → revenue.

8. **Traffic model.** RPS = active users × engagement × a diurnal curve, plus event modifiers. Simulated time advances in fixed ticks (initially one tick = five simulated minutes, so one simulated day is 288 ticks); tuning values are ruleset data, not code constants.

9. **Event deck.** A seeded deck of events, whose odds depend on the world state: viral surge, marketing spike, seasonal dip, sudden traffic drop, DDoS, instance crash, zone outage, database slowdown, cache stampede, queue backlog, cost spike, and third-party outage. Higher complexity raises failure odds; higher popularity raises surge and attack odds. Incident events reuse the vocabulary of the existing scenario catalog (`docs/scenarios.md`) so the Sandbox and Live mode teach the same failure modes.

10. **Commands, not mutations.** The player changes the world only through commands — `place`, `remove`, `connect`, `disconnect`, `resize`, `scale` (replicas), `set-speed` (pause, 1×, 2×, 4×), and later `respond` actions (failover, rate-limit, rollback). Commands are validated by the engine, applied at a tick boundary, and appended to the command log. A saved game is the seed plus the command log (a declared, replayable artifact, in keeping with Principle 6), stored under `.forgelab/sandbox/` (git-ignored).

11. **API.** New endpoints under `/api/v1/sandbox/`: create a game, get its state, post commands, and stream ticks with Server-Sent Events. Games are held in memory by `forgelab serve` and saved to disk on request.

12. **Dashboard.** A new Sandbox screen: a build palette, a topology canvas where components are placed and wired, a HUD (cash, revenue/cost, RPS, userbase, health, satisfaction), a component inspector (utilization, latency, errors, cost), an event feed, and speed controls. The canvas uses **React Flow (`@xyflow/react`)**, approved by the user as a new dashboard dependency. Time-series sparklines use plain SVG; no chart library is added.

13. **Delivered in milestones** (ROADMAP Phase 10): docs and ADR → core engine → API → dashboard canvas and HUD → event deck and incident response → goals and unlocks (missions). Each milestone ships with tests and doc updates.

## Consequences

### Positive

- The learner designs the architecture, not just breaks it, and sees it evolve under growth — the missing half of production intuition.
- Production-scale dynamics (hundreds of instances, millions of RPS, multi-day growth) run instantly on any laptop, with no Docker, cloud cost, or ports.
- Cost, revenue, and satisfaction make trade-offs concrete: over-provisioning is safe but bankrupting; under-provisioning is cheap until the surge.
- Determinism (seed + command log) makes games reproducible, shareable, and testable in CI.
- The engine is a headless Go package, so the CLI, tests, or a future analytics service can drive it without the UI.

### Negative / Trade-offs

- **A model is not production.** Sandbox dynamics are only as honest as the capacity and queueing formulas; they are approximations (M/M/c-style latency, fixed hit ratios). Mitigated by labelling every Sandbox value as simulated, documenting the formulas, and keeping Live mode as the ground truth.
- **Principle 10 is narrowed.** It now applies to Live mode; Sandbox is governed by the new "modelled, not faked" rule. Future contributors must not use the Sandbox as precedent for faking Live-mode metrics.
- **Game-balance work.** Tuning costs, capacities, and event odds is ongoing and subjective; tuning lives in ruleset data so it can change without code changes, and each ruleset is versioned so saves stay replayable.
- **New dependency.** `@xyflow/react` adds to the dashboard's supply chain.
- **Scope growth.** The dashboard stops being purely a thin monitoring surface and becomes the primary product surface; the core must keep up with a richer API.
- **Two modes to explain.** Documentation, README, and the learning path must make clear which mode a page or exercise uses.

## Alternatives Considered

- **Replace the lab with the game** — rejected: throws away Phases 1–9 and the ground truth that keeps the model honest.
- **Game drives real infrastructure (hybrid)** — rejected for now: placing a component would start containers, capping scale at the host and adding latency, ports, and failure modes unrelated to the lesson. It can be revisited later as an explicit "materialize this design in Live mode" feature.
- **Simulation logic in the dashboard (TypeScript)** — rejected: violates the thin-consumer rule and the deterministic, headless core.
- **Business archetypes (e-commerce, social, streaming)** — rejected for the first ruleset: it brings domain logic into the model. Archetypes may return later as workload templates in the ADR-0004 sense (traffic shape and operation mix only).
- **Random or scripted metric curves** — rejected: bottlenecks would not emerge from the player's design, and the game would teach nothing.
- **Hand-rolled SVG canvas instead of React Flow** — viable and dependency-free, but slower to build for dragging, wiring, zoom, and selection; the user chose React Flow.

## References

- `ROADMAP.md` Phase 10 — Production Sandbox
- `docs/architecture.md` (Sandbox mode)
- `docs/principles.md` (Principle 10 scope, Principle 15)
- `docs/component-catalog.md` (Sandbox domain)
- `docs/decisions/0003-resource-virtualization-engine.md`
- `docs/decisions/0004-synthetic-applications-workload-engine.md`
- `docs/decisions/0010-pipeline-simulation-and-dashboard.md`
