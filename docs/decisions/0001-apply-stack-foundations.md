# ADR-0001: Stack Foundations — Go Core, Next.js Dashboard, Optional Python, Composable Monorepo

**Status:** Accepted
**Date:** 2026-09-18

## Context

ForgeLab is a Production Systems Laboratory: a reusable platform where a user plugs in their own application and enables only the production components they need. Before any implementation, the foundational technology choices and repository shape must be fixed, because every later component, scenario, and ADR builds on them.

Decisions needed:

1. What powers the simulation/runtime core (the part that is ForgeLab's own product logic)?
2. What powers the dashboard?
3. Is analytics/AI part of the core?
4. How is the repository organized to honor "modular composition, not a fixed architecture"?

Candidate cores considered included Elixir (strong concurrency, but a thinner infrastructure-tooling ecosystem) and Go (mature tooling culture). The requirement that the simulation core is deterministic, headless-testable, and separable from any UI also influences the shape.

## Decision

1. **Simulation/runtime core: Go.** Follow the `golang-standards/project-layout`-style module layout. Simulation logic stays pure Go, separable from CLI/API/UI layers. Rationale: the infrastructure-tooling ecosystem (instrumentation, Kubernetes, observability clients, single static binaries, explicit concurrency) is significantly stronger in Go than in Erlang/Elixir, and Go aligns with the platform's target tooling domain.

2. **Dashboard: Next.js** (TypeScript, App Router). A thin consumer of the backend API. It must never duplicate simulation logic.

3. **Analytics/AI: Python**, later and optional, as a separate service/module — never woven into the Go core.

4. **Repository shape: composable monorepo** with a domain-driven folder layout (`docs/`, `applications/`, `manifests/`, `components/<domain>/`, `environments/`, `scenarios/`, `templates/`). Every platform component is optional and may be opted out by any user. Documentation precedes implementation; architecture decisions live in this directory.

## Consequences

### Positive

- The core is deterministic, headless-testable, and independent of any UI or vendor.
- Strong infrastructure ecosystem for the platform's actual problem domain (Go).
- Dashboard and analytics stay separable; no business logic leaks between layers.
- "Composition over configuration" is structurally enforced by the folder layout and manifest-driven design.
- A documentation-first, ADR-driven process keeps long-term decisions auditable.

### Negative / Trade-offs

- Go has a richer concurrency ergonomics story via goroutines but requires deliberate discipline for domain modeling compared to a typed functional language.
- Python analytics is a second runtime to run and operate; it is deferred until the value is proven.
- Monorepo scale requires folder-ownership discipline (`docs/repository-structure.md`, `AGENTS.md` §8) to avoid drift.

## Alternatives Considered

- **Elixir/Phoenix core** — excellent concurrency and fault-tolerance for the simulator, but a weaker infrastructure-tooling ecosystem and a less natural fit for the platform's Kubernetes/observability-adjacent tooling work. Not chosen.
- **Fully Rust core** — viable, but Go was preferred for the maturity and breadth of platform tooling libraries and faster iteration for this project's goals.
- **All-in-one application repo** — rejected: ForgeLab must be a platform, not an application; blurring that boundary is the anti-goal.
- **Python core** — rejected for the core (perf, deployment, and tooling reasons); reserved for analytics.

## References

- `docs/architecture.md` (platform/application boundary, layering)
- `docs/vision.md` (application vs platform)
- `AGENTS.md` §2 (decided stack) and §6 (ADR requirement)
- `ROADMAP.md` Phase 0 (current, documentation-only)