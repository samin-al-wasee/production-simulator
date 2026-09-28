# Pipeline Runner

Simulated build → test → deploy pipelines on a virtual clock.

**Status:** implemented (Phase 7)

## Purpose

Simulated build → test → deploy pipelines on a virtual clock.

## Provided

- `forgelab pipeline run [-seed N] [-warm-cache] [-fail-step NAME] [-bad-release] [-json] <pipeline>` (`core/internal/pipeline`).
- Stages with sequential or parallel steps, cache-aware durations, seeded flaky steps with retries, forced failures, and deploy stages for rolling, canary, and blue/green with rollback on a bad release.
- Example pipelines in `manifests/pipelines/`; browsable and runnable in the dashboard.

## Dependencies

- None (pure Go; nothing is executed)

## Configuration

Pipeline schema: `apiVersion: forgelab/v1`, `kind: Pipeline`; see `manifests/pipelines/web-release-canary.yaml`. Durations are virtual: a run of many minutes returns instantly.
