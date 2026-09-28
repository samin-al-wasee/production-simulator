# ADR-0008: Chaos, load, and recovery tooling in the Go core (Phase 5)

**Status:** accepted
**Date:** 2026-09-28

## Context

Phase 5 adds chaos injection, outage drills, a load-testing harness, and disaster-recovery drills. These need to be repeatable, safe by default, declared as code, and testable without the infrastructure they act on. Principle 5 (failure is a feature) requires drills to state a hypothesis and check it, not just break things.

## Decision

1. **Chaos experiments are declared** as `kind: Experiment` YAML (`scenarios/failures/<name>/experiment.yaml`): a hypothesis, one target container, one fault, and HTTP probes in three phases (`before` steady state, `during` the fault, `after` recovery), each with an expected status and optional body substring.
2. **`core/internal/chaos` owns the engine.** Planning (`Plan`) is pure; execution goes through injected `Executor` and `Prober` interfaces, so the engine is unit-tested with fakes. The run order is fixed: verify steady state (no injection if it fails), inject, verify degradation, hold the fault, **always revert**, verify recovery. Reverting uses a fresh context so an interrupted run still restores the target, and runs exactly once.
3. **Safety by construction:** targets must match `^forgelab-[a-z0-9-]+$`, so the tool cannot act on other containers; `forgelab chaos plan` and `make chaos DRY=1` print the commands without running them.
4. **Faults:** `stop`, `pause`, `cpu-throttle` (via `docker update`), and `latency` / `packet-loss` (via `tc netem` run in the target's network namespace from a throwaway container, so target images need no tooling; the fault needs `NET_ADMIN`, granted only to that helper container).
5. **`core/internal/loadtest` is an open-loop generator** with `constant`, `ramp`, and `spike` profiles. The schedule is deterministic and requests are dispatched on time regardless of response time; when the in-flight limit is reached they are counted as dropped instead of delayed, avoiding coordinated omission. Results include latency percentiles (nearest rank), status counts, error rate, and a virtual RPS column derived from the scale factor (Dual Metrics Mode).
6. **Backup and recovery** use plain `pg_dump`/`psql` scripts (`scripts/db-backup.sh`, `db-restore.sh`). `scripts/dr-drill.sh` runs a full destroy-and-restore drill and refuses to run without `FORGELAB_CONFIRM_DESTRUCTIVE=1` because it deletes the database volume.
7. **Outage drills** ship for the database, Redis, and Kafka, plus a network-latency drill; Kafka is observed through Prometheus (`up{job="kafka"}`) because the sample application does not use it.

## Consequences

### Positive

- Every drill states a hypothesis and reports pass or fail; a wrong hypothesis is a finding, not an error.
- The engine's ordering and revert guarantees are covered by tests without Docker.
- Load results are comparable across runs thanks to the deterministic schedule.

### Negative / Trade-offs

- Faults act on Docker Compose containers only; Kubernetes pod-level chaos is not included yet.
- Probes only check status and body substrings, so latency assertions are made with the load test, not the experiment.
- The latency fault runs `apk add` inside a helper container at injection time, which needs network access and adds a few seconds.
- The destructive part of the DR drill is opt-in and was not exercised in automated verification; backup and restore themselves were.

## Alternatives Considered

- **Chaos Mesh / Litmus / Toxiproxy** — heavier, Kubernetes- or proxy-centric; the Compose-first lab needs something that runs on a laptop with no extra control plane.
- **k6 / vegeta / wrk for load** — good tools, but a small in-core harness gives virtual-RPS translation and keeps the drills self-contained; they can still be used against the same targets.
- **Closed-loop load generation** — rejected because slow responses would reduce offered load and hide latency.
- **Shell scripts for chaos** — rejected: no hypothesis checking, ordering guarantees, or unit tests.

## References

- `ROADMAP.md` Phase 5 — Reliability & chaos
- `docs/principles.md` principle 5
- `docs/decisions/0006-messaging-overlay-and-retry-semantics.md`
