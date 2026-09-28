# ADR-0010: Pipeline simulation, core API, and Next.js dashboard (Phase 7)

**Status:** accepted
**Date:** 2026-09-28

## Context

Phase 7 adds CI/CD pipeline simulation and a dashboard to launch, monitor, and inspect experiments. AGENTS.md requires the dashboard to be a thin consumer of the backend API and forbids duplicating simulation logic, and the simulation core to be deterministic and testable headlessly.

## Decision

1. **Pipelines are simulated, not executed.** `core/internal/pipeline` plays a declared `kind: Pipeline` (build, test, deploy) on a virtual clock. Steps have durations, optional warm-cache durations, and a flaky failure chance with retries; parallel stages take the longest step. Flakiness uses a seeded PRNG, so the same pipeline, seed, and options always give the same run. Deploy stages model rolling, canary, and blue/green strategies; a bad release fails the readiness or analysis gate and rolls back. Examples live in `manifests/pipelines/`; the CLI is `forgelab pipeline run`. Simulation never shells out, so it is safe to expose.
2. **A small HTTP adapter in the core** (`core/internal/api`, `forgelab serve`) exposes host budget, dual metrics (`/cluster`), local stack status, experiments and their runs, and pipeline simulation. Every value is computed by the core packages.
3. **Starting experiments is opt-in and constrained:** disabled unless `-enable-runs`, only on loopback listen addresses, only experiments discovered under `scenarios/` (no arbitrary paths), one run at a time, and the chaos engine's own `forgelab-*` target restriction still applies. The API binds `127.0.0.1:8090` by default.
4. **Dashboard:** Next.js (App Router, TypeScript) in `dashboard/`. Server components fetch the API with no caching; interactive parts (run experiment and poll, simulate pipeline) are small client components that call `/api/forgelab/*`, which `next.config.ts` rewrites to the API, so the browser needs no CORS. Pages: Overview (Dual Metrics Mode plus local stack), Experiments, Pipelines. The scale factor badge is in the header of every page, physical and virtual panels are visually distinct and labelled, and virtual values are never presented as hardware measurements.
5. **Dual metrics semantics corrected:** virtual usage applies each resource's own virtual/physical ratio to its physical usage, so utilization is identical in both views and bottlenecks are preserved. Using the single power-of-two effective factor for every resource had shown 272% virtual CPU utilization on a host at 13% load.
6. **Quality gates:** `make lint-fe`, `make test-fe` (ESLint, `tsc --noEmit`, Vitest for formatting and timeline helpers) and `next build` pass; the API has handler tests including the disabled-by-default and single-run guarantees.

## Consequences

### Positive

- Pipelines and experiments can be explored without touching real infrastructure (pipelines) or with strong guardrails (experiments).
- Dashboard logic is limited to rendering, so behavior stays testable headlessly in Go.
- Same-origin calls through the rewrite avoid CORS configuration and keep the API off public addresses.

### Negative / Trade-offs

- Live monitoring is a snapshot on page load; there is no streaming, and Prometheus and Grafana remain the tools for time series.
- Experiment logs are polled once per second while a run is active.
- The dashboard has no authentication; it is designed for loopback use on a developer machine.
- Component rendering is verified by build, type check, lint, and manual requests, not by browser tests.

## Alternatives Considered

- **Executing real pipeline steps** — rejected: the goal is teaching pipeline dynamics, and execution would let the API run arbitrary commands.
- **Putting simulation logic in Next.js route handlers** — rejected by AGENTS.md §2.
- **Server-sent events or WebSockets for monitoring** — deferred until a use case needs them.
- **Adding a component library (Tailwind, shadcn)** — avoided to keep dependencies small (AGENTS.md §13).

## References

- `ROADMAP.md` Phase 7 — CI/CD & dashboard
- `docs/decisions/0003-resource-virtualization-engine.md` (Dual Metrics Mode)
- `docs/decisions/0008-chaos-load-and-recovery-tooling.md`
