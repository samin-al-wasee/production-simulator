# ADR-0003: Resource Virtualization Engine (Phase 0.5)

**Status:** proposed
**Date:** 2026-09-19

## Context

ForgeLab runs on a developer's finite local hardware (e.g. 16 GB RAM, 8 CPU cores) yet must simulate production systems spanning hundreds of servers, terabytes of memory, and millions of requests per second. Several existing principles conflict with raw emulation of such systems: "production parity" (the lab must behave like real production), "failure is a feature" (bottlenecks and exhaustion are study material), and "composition over configuration" (every environment is assembled from declared pieces).

The platform must therefore distinguish two notions of resource that were previously implicit:

1. **Physical Resources** — the user's actual hardware: real cores, RAM, disk, and running containers/processes.
2. **Virtual Production Resources** — the simulated production environment ForgeLab presents: virtual nodes, RAM, RPS, storage, and replicas.

The missing architectural component is the subsystem that reconciles the two without faking behavior.

## Decision

Introduce the **Resource Virtualization Engine** as a top-level architectural subsystem, authorized by a new **Phase 0.5 — Simulation Foundation** milestone in `ROADMAP.md`.

The engine defines and maintains a three-layer model:

1. **Physical Layer** — the actual host: hardware detection, physical resource budgeting, and the real containers/processes ForgeLab orchestrates.
2. **Simulation Layer** — the Resource Virtualization Engine: hardware calibration, physical budgeting, deterministic **scale factor** calculation, and the virtual resource model of the declared production topology.
3. **Production View** — the virtual cluster users interact with: nodes, capacity, traffic, and topologies, with metrics presented in virtual terms.

Responsibilities of the engine: hardware detection and calibration, physical resource budgeting, virtual resource modeling, scale factor calculation, horizontal and vertical scaling virtualization, traffic/RPS virtualization, storage and database capacity modeling, capacity exhaustion simulation, and physical ↔ virtual metrics translation.

Planned module structure (documented, not implemented), kept pure Go and deterministic:

```text
simulation/
├── capacity-engine/
├── resource-virtualization/
├── scaling-model/
├── traffic-model/
├── calibration/
└── profiles/
```

The engine **never fakes behavior, only virtualizes capacity**: bottlenecks, latency, backpressure, and exhaustion occur for the same reasons they would in real production, at the scale the view claims. Every dashboard surface must expose both physical and virtual metrics with the current scale factor always visible (**Dual Metrics Mode**).

## Consequences

### Positive

- ForgeLab can honestly represent production environments much larger than the host hardware, keeping the "production parity" and "failure is a feature" principles intact.
- Deterministic scale-factoring keeps the simulation core headless-testable and reproducible for the same host + manifest.
- Physical vs virtual separation makes limits concrete: capacity exhaustion and budgeting are first-class simulation mechanics.
- Dashboards remain thin consumers; the engine's metric translation lives in the Go core, not the UI.
- The `simulation/` modules mirror the catalog's Simulation domain, keeping structure and docs aligned.

### Negative / Trade-offs

- Adds a foundational subsystem and a new milestone (Phase 0.5) before Phase 1's runtime; the engine's model must be designed carefully so later phases (observability, Kubernetes) compose onto it without rework.
- Physical ↔ virtual translation touches every metrics path, so instrumentation conventions must be defined early to avoid drift.
- Risk of presenting virtual values as real measurements; mitigated by mandatory scale labels and Dual Metrics Mode.

## Alternatives Considered

- **Emulate capacity literally (no virtualization)** — a laptop cannot run hundreds of nodes or terabytes of RAM; infeasible and pointless.
- **Fake the numbers (cosmetic scaling)** — rejected outright: it would violate production parity and make incidents meaningless.
- **Linearly clamp virtual capacity to host capacity** — discards the teaching value of scale; not chosen because bottleneck preservation is a goal, not a side effect.
- **Keep virtualization logic in the dashboard only** — rejected: simulation logic must live in the deterministic Go core, never the UI.

## References

- `docs/architecture.md` (three-layer model, `simulation/` layout, Dual Metrics Mode)
- `docs/principles.md` (principles 9–12: Physical ≠ Virtual, virtualize capacity, preserve dynamics, dual metrics)
- `docs/component-catalog.md` (Simulation domain: Resource Virtualization Engine, Capacity Engine, Scaling Model, Traffic Model, Hardware Calibration, Resource Profiles)
- `ROADMAP.md` Phase 0.5 — Simulation Foundation
- `docs/vision.md` (Production Systems Simulator framing)
- `docs/decisions/0001-apply-stack-foundations.md` (Go core, composable monorepo)