# ADR-0004: Synthetic Applications and Workload Engine

**Status:** superseded by [ADR-0014](0014-sandbox-only-platform.md) (was proposed; the code it describes is on the `archive/live-lab` branch)
**Date:** 2026-09-19

## Context

ForgeLab's value is in reproducing **production behavior** — traffic, latency, failures, scaling, incidents — not in shipping a specific application. Today that requires a user to bring a real application (Rails, Phoenix, Django, FastAPI, Spring Boot, Node.js, Next.js, or another containerized app) before they can use the platform. That prerequisite is a barrier: building, or even importing, a realistic business application is expensive and distracts from the learning goal.

More fundamentally, a real application is not what makes a production system interesting. The interesting properties come from the **operations** the application performs against infrastructure: read/write patterns, cache behavior, messaging, worker concurrency, latency, retries, connection pools. Those properties are generic and can be modeled independently of any business logic.

This decision establishes Synthetic Applications as a first-class input so ForgeLab can generate production-like applications from configuration alone.

## Decision

Introduce a **Synthetic Applications / Workload Engine** subsystem, documented in `docs/architecture.md`, with these commitments:

1. **Two application modes.** Mode A — real applications (user-provided, deployed and observed as-is). Mode B — synthetic applications (generated from configuration, no business logic).
2. **Behavior over features.** The simulator executes **production operations** (HTTP, db reads/writes, transactions, cache reads/writes, CPU/memory workloads, external calls, messaging, background jobs, file operations, WebSockets, retries, timeouts, concurrency, connection pools, rate limiting, batch processing). Business-domain names are **feature labels** — a visualization layer that can be attached to any workload without changing system behavior.
3. **Workload DSL.** Synthetic applications are declared with a future declarative **Workload DSL**. The schema is **not finalized** and is documented as conceptual; it will evolve through implementation.
4. **Workload templates.** A planned template library under `synthetic-apps/templates/` (generic, ecommerce, travel, banking, social) describing **engineering characteristics**, not complete business applications.
5. **Production-realistic composition.** Generated systems mirror real topology — load balancer, reverse proxy, API gateway, multiple services, PostgreSQL, Redis, RabbitMQ/Kafka, workers, external services — with components opt-in via configuration.
6. **Resource virtualization integration.** Synthetic applications consume **simulated resources** through the Resource Virtualization Engine (CPU, memory, network, disk, database capacity, connection pools, cache/queue capacity, worker concurrency). This is a **capacity/workload model** — it scales how much capacity is displayed, never an attempt to execute a million real requests on local hardware.
7. **Scenario integration.** Synthetic applications are the designed substrate for the scenario engine: traffic → saturation → latency → backlog → scaling → bottleneck → incident.
8. **Planned structure.** Generator modules under `synthetic-apps/` (workload-engine, application/service/endpoint generators, operation-engine, db/cache/messaging models, concurrency-model, telemetry-generator, templates). **Implementation language and architecture remain open.**
9. **Dashboard integration.** Future visual workflow: Application Type → Select Templates → Configure Services → Configure Operations → Configure Traffic → Configure Infrastructure → Start Simulation, with controls for endpoints, templates, DB/cache/broker operations, concurrency, latency, failure rates, traffic distribution, and scaling.

## Consequences

### Positive

- Removes the "bring your own app" barrier; anyone can exercise production behavior immediately.
- One workload model generates thousands of different production-system configurations without thousands of custom applications.
- Synthetic apps are measurable, deterministic, and scenario-ready by construction — ideal subjects for the incident engine.
- Feature-label separation keeps business terminology out of the simulation core (platform stays free of business logic).
- Composes cleanly with Resource Virtualization; a production-scale app can be modeled without matching physical infrastructure.

### Negative / Trade-offs

- **Not locked in the roadmap** — this subsystem is documented as a proposed planning sketch; its future phasing must be folded into the authoritative `ROADMAP.md` phases when authorized (possible overlap: Resource Virtualization sketch phase vs authoritative Phase 0.5).
- Risk of modeling operations too coarsely versus a real application's behavior — mitigated by templates that encode common engineering characteristics.
- Workload DSL is a new schema to design, version, and validate alongside the existing application manifest.
- Synthetic apps could be mistaken for real ones in dashboards; Dual Metrics Mode and explicit labeling must apply here too.

## Alternatives Considered

- **Require a real application for everything** — rejected: preserves the barrier and scales poorly to "thousands of configurations."
- **Ship many small sample business apps** — rejected: violates platform-over-application, becomes a maintenance burden, and covers only the shipped features.
- **Fake the metrics without executing workloads** — rejected: violates production parity and makes scenarios meaningless (mirror of the "never fake behavior" principle).
- **Model only infrastructure, not applications** — rejected: misses the most important dynamics (latency chains, workers, saturation) that originate in application behavior.

## References

- `docs/architecture.md` (Synthetic Applications / Workload Engine, application modes, Workload DSL, generator layout)
- `docs/vision.md` (three inputs → one production model; Production Systems Simulation and Engineering Platform)
- `docs/principles.md` (principles 13–14: behavior over business functionality, business labels are visualization)
- `docs/component-catalog.md` (Synthetic Applications domain components)
- `docs/repository-structure.md` (`synthetic-apps/` planned layout)
- `docs/scenarios.md` (synthetic apps as scenario substrate)
- `docs/glossary.md` (Synthetic Application, Workload DSL, Production Operation, Feature Label, Workload Template)
- `docs/decisions/0003-resource-virtualization-engine.md` (Resource Virtualization Engine, Phase 0.5)
- `docs/decisions/0001-apply-stack-foundations.md` (Go core, composable monorepo)