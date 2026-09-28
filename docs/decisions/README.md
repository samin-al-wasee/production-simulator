# Architecture Decision Records

This directory records ForgeLab's architecture decisions. Rules live in `AGENTS.md` §6; use this file as the index and follow the template below for new records.

## Index

| ADR | Status | Title |
|---|---|---|
| [0001](0001-apply-stack-foundations.md) | Accepted | Stack foundations: Go core, Next.js dashboard, optional Python, composable monorepo |
| [0002](0002-local-single-node-runtime.md) | Proposed | Local single-node runtime: Docker Compose local preset and Go core in `core/` |
| [0003](0003-resource-virtualization-engine.md) | Accepted | Resource Virtualization Engine: physical vs virtual resource model and `simulation/` modules (Phase 0.5) |
| [0004](0004-synthetic-applications-workload-engine.md) | Proposed | Synthetic Applications / Workload Engine: behavior-over-features model, Workload DSL, workload templates |
| [0005](0005-observability-overlay.md) | Accepted | Observability as a composable Compose overlay (Phase 2) |
| [0006](0006-messaging-overlay-and-retry-semantics.md) | Accepted | Messaging & data overlay with declared retry and dead-letter semantics (Phase 3) |
| [0007](0007-kubernetes-environment.md) | Accepted | Kubernetes environment: kind cluster with Kustomize manifests (Phase 4) |
| [0008](0008-chaos-load-and-recovery-tooling.md) | Accepted | Chaos, load, and recovery tooling in the Go core (Phase 5) |
| [0009](0009-cloud-presets-and-cost-guard.md) | Accepted | Terraform cloud presets with a plan-time cost guard (Phase 6) |
| [0010](0010-pipeline-simulation-and-dashboard.md) | Accepted | Pipeline simulation, core API, and Next.js dashboard (Phase 7) |
| [0011](0011-security-controls-and-verification.md) | Accepted | Security controls, verification, and defensive attack drills (Phase 8) |

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
