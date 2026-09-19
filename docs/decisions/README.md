# Architecture Decision Records

This directory records ForgeLab's architecture decisions. Rules live in `AGENTS.md` §6; use this file as the index and follow the template below for new records.

## Index

| ADR | Status | Title |
|---|---|---|
| [0001](0001-apply-stack-foundations.md) | Accepted | Stack foundations: Go core, Next.js dashboard, optional Python, composable monorepo |
| [0002](0002-local-single-node-runtime.md) | Proposed | Local single-node runtime: Docker Compose local preset and Go core in `core/` |
| [0003](0003-resource-virtualization-engine.md) | Proposed | Resource Virtualization Engine: physical vs virtual resource model and `simulation/` modules (Phase 0.5) |

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