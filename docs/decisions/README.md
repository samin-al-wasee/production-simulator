# Architecture Decision Records

This directory records ForgeLab's architecture decisions. Rules live in `AGENTS.md` §6; use this file as the index and follow the template below for new records.

## Index

| ADR | Status | Title |
|---|---|---|
| [0001](0001-apply-stack-foundations.md) | Accepted | Stack foundations: Go core, Next.js dashboard, optional Python, composable monorepo |
| [0002](0002-local-single-node-runtime.md) | Superseded by 0014 | Local single-node runtime: Docker Compose local preset and Go core in `core/` |
| [0003](0003-resource-virtualization-engine.md) | Superseded by 0014 | Resource Virtualization Engine: physical vs virtual resource model and `simulation/` modules (Phase 0.5) |
| [0004](0004-synthetic-applications-workload-engine.md) | Superseded by 0014 | Synthetic Applications / Workload Engine: behavior-over-features model, Workload DSL, workload templates |
| [0005](0005-observability-overlay.md) | Superseded by 0014 | Observability as a composable Compose overlay (Phase 2) |
| [0006](0006-messaging-overlay-and-retry-semantics.md) | Superseded by 0014 | Messaging & data overlay with declared retry and dead-letter semantics (Phase 3) |
| [0007](0007-kubernetes-environment.md) | Superseded by 0014 | Kubernetes environment: kind cluster with Kustomize manifests (Phase 4) |
| [0008](0008-chaos-load-and-recovery-tooling.md) | Superseded by 0014 | Chaos, load, and recovery tooling in the Go core (Phase 5) |
| [0009](0009-cloud-presets-and-cost-guard.md) | Superseded by 0014 | Terraform cloud presets with a plan-time cost guard (Phase 6) |
| [0010](0010-pipeline-simulation-and-dashboard.md) | Accepted (partly superseded by 0014) | Pipeline simulation, core API, and Next.js dashboard (Phase 7) |
| [0011](0011-security-controls-and-verification.md) | Superseded by 0014 | Security controls, verification, and defensive attack drills (Phase 8) |
| [0012](0012-production-simulator-experience.md) | Superseded by 0014 | Benchmark reports, learning-path tracking, and the end-to-end run (Phase 9) |
| [0013](0013-production-sandbox-game.md) | Accepted (Live mode superseded by 0014) | Production Sandbox: a model-driven production-system game built from an empty world (Phase 10) |
| [0014](0014-sandbox-only-platform.md) | Accepted | ForgeLab is the Production Sandbox; the Live-mode lab is retired |
| [0015](0015-backend-and-frontend-folders.md) | Accepted | Name the top-level code folders `backend/` and `frontend/` |
| [0016](0016-configurable-internet-traffic.md) | Accepted | A configurable Internet: traffic groups, request mix, and load tests (Phase 11) |
| [0017](0017-application-instance-model.md) | Accepted | The application instance as a modelled backend service (Phase 11) |
| [0018](0018-traffic-components.md) | Accepted | Traffic components and the traffic-to-application contract (Phase 11) |
| [0019](0019-connections-and-service-calls.md) | Accepted | Connections and inter-service communication (Phase 11) |
| [0020](0020-database-model.md) | Accepted | The database as a modelled data store (Phase 11) |
| [0021](0021-cache-model.md) | Accepted | The cache as a modelled store (Phase 11) |
| [0022](0022-object-storage-model.md) | Accepted | Object storage as a priced, rate-limited service (Phase 11) |
| [0023](0023-queue-and-worker-model.md) | Accepted | Message queues with redelivery, and workers as consumers (Phase 11) |
| [0024](0024-event-streams.md) | Accepted | Event streams and event-driven architecture (Phase 11) |
| [0025](0025-edge-components.md) | Accepted | Load balancer, API gateway, and CDN with contracts (Phase 11) |
| [0026](0026-container-deployment.md) | Accepted | Deploying the API as a container (Render) |
| [0027](0027-configured-telemetry.md) | Accepted | Telemetry is configured, not given (Phase 12) |
| [0028](0028-explaining-failures.md) | Accepted | Explaining failures: causes, metrics history, traces, and logs (Phase 12) |
| [0029](0029-alerts-slos-incidents.md) | Accepted | Alerts, SLOs, and an incident timeline (Phase 12) |
| [0030](0030-application-store-and-postgres.md) | Accepted | An application layer and a Postgres application store (Phase 13) |
| [0031](0031-oauth-identity.md) | Accepted | OAuth-only sign-in with database-backed sessions (Phase 13) |
| [0032](0032-user-owned-resources.md) | Accepted | User-owned sandboxes and progress, with an anonymous fallback (Phase 13) |
| [0033](0033-sign-in-front-door-and-my-forgelab.md) | Accepted | Sign-in as the front door, and the My ForgeLab shell (Phase 13) |

## Conventions

- File naming: `NNNN-short-title.md`, zero-padded (e.g. `0001-apply-stack-foundations.md`).
- Each ADR has a stable numeric identifier. Never renumber or reuse.
- Create an ADR when a decision materially affects architecture, component boundaries, data flow, observability, reliability, or long-term maintainability.
- Minor implementation decisions do **not** require an ADR.
- A decision is not finalized until it is implemented **and** reflected in the relevant docs (`docs/architecture.md`, `docs/component-catalog.md`, etc.).
- Status values: `proposed` → `accepted` / `rejected` / `deprecated` / `superseded by NNNN`.

## Template

```markdown
# ADR-NNNN: Short Decision Title

**Status:** proposed
**Date:** YYYY-MM-DD

## Context

Why is this decision needed? What constraint or problem does it address?

## Decision

What exactly was decided? State it in full sentences, not bullet-point pride.

## Consequences

### Positive

* ...

### Negative / Trade-offs

* ...

## Alternatives Considered

* **Alternative** — why it was not chosen

## References

* Related docs / ADRs
```

To create a new ADR: copy the template, write the decision, add a row to the index table above, and update any architecture/catalog docs the decision affects.
