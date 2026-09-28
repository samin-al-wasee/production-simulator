# ADR-0012: Benchmark reports, learning-path tracking, and the end-to-end run (Phase 9)

**Status:** accepted
**Date:** 2026-09-28

## Context

Phase 9 delivers the complete ForgeLab experience end to end: a learner should be able to run the whole lab, measure it, work through the learning path with visible progress, and rehearse a multi-failure incident that ends in a postmortem. Progress and reports are personal artifacts and must not pollute the repository, and progress must not depend on self-reporting alone where the platform can observe the outcome.

## Decision

1. **Benchmark reports:** `forgelab benchmark` (`core/internal/report`, built on `internal/loadtest`) runs three profiles (baseline, ramp, spike) against a target, evaluates SLOs (p95 latency, error rate, dropped-request rate; defaults 250 ms, 1%, 5%), and writes a Markdown and a JSON report to `reports/` (git-ignored). Reports describe the host and, when a cluster spec is available, the simulated production and scale factor; virtual RPS is always labelled as a simulated capacity figure, never as a measurement. Rendering is deterministic for the same input; the timestamp is supplied by the caller.
2. **Learning path as data:** `learning/path.yaml` is the machine-readable form of `docs/learning-path.md`: 10 stages and 21 exercises, each with an evidence rule. `core/internal/learning` validates the path and tracks progress in `.forgelab/progress.json` (git-ignored, override with `FORGELAB_PROGRESS`).
3. **Evidence over self-report where possible:** exercises tied to a chaos experiment complete automatically when that experiment passes (CLI or dashboard), and the benchmark exercise completes when a benchmark meets its SLOs; the remaining exercises are marked manually. Completing twice keeps the first record. Path order is guidance (`forgelab learn next`), not a gate.
4. **Surfaces:** `forgelab learn status|next|complete`, API endpoints `GET /api/v1/learning` and `POST /api/v1/learning/{id}/complete`, and a Learning path page in the dashboard, all backed by the same package.
5. **End-to-end run:** `scripts/e2e.sh` (`make e2e`) starts the Compose lab with the observability and messaging overlays, waits for health, then runs the messaging smoke test, the database and Redis outage drills, a short benchmark, and the secret scan, prints learning progress, and tears the stack down (volumes kept; `E2E_KEEP=1` leaves it running). Kubernetes, security-overlay, and cloud checks remain separate commands because they need different tools and ports.
6. **Incident practice:** the `multi-failure-incident` scenario combines two faults under load, and `templates/postmortem/postmortem.md` is the postmortem template for its write-up.

## Consequences

### Positive

- The whole experience is one command (`make e2e`), and each learner's progress and reports stay local.
- Progress is partly verified by the platform, not only self-reported.
- Benchmarks are comparable before and after a change or fault, with the scale factor stated.

### Negative / Trade-offs

- Manual exercises rely on the learner's honesty; the path is a study aid, not an assessment.
- `make e2e` needs Docker, several minutes, and free host ports (8080, 3000, 9090, 5672, 15672, 6379, 9092).
- Running the drills through the e2e script records progress in the local checkout, which is intended but can surprise someone who only wanted a smoke test; set `FORGELAB_PROGRESS` to a scratch file to avoid it.
- The end-to-end run deliberately excludes Kubernetes and cloud presets.

## Alternatives Considered

- **Store progress in a database or the dashboard's browser storage** — rejected: the CLI, API, and dashboard must agree, and a small JSON file is enough.
- **Certificates or scores for completion** — out of scope; the goal is practice, not credentials.
- **Include Kubernetes in the e2e run** — rejected for run time and tooling requirements; it has its own smoke tests.
- **Generate reports as HTML** — Markdown and JSON are reviewable in a pull request and machine-readable; HTML can be layered on later.

## References

- `ROADMAP.md` Phase 9 — Production Simulator
- `docs/learning-path.md`
- `docs/decisions/0008-chaos-load-and-recovery-tooling.md`
- `docs/decisions/0010-pipeline-simulation-and-dashboard.md`
