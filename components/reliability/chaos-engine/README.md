# Chaos Engine

Fault injection with hypothesis checking.

**Status:** implemented (Phase 5, `local` preset)

## Purpose

Fault injection with hypothesis checking.

## Provided

- `forgelab chaos plan|run <experiment>` (`core/internal/chaos`): stop, pause, cpu-throttle, latency, packet-loss against `forgelab-*` containers.
- Experiment order: steady state → inject → verify degradation → hold → revert (always, once) → verify recovery.
- `make chaos FILE=<experiment> [DRY=1]`.

## Dependencies

- Docker
- Running local stack (`make up`, plus overlays for Redis/Kafka drills)

## Configuration

Experiment schema: `apiVersion: forgelab/v1`, `kind: Experiment`; see `scenarios/failures/db-outage/experiment.yaml`. Targets must match `forgelab-[a-z0-9-]+`.
