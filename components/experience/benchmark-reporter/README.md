# Benchmark Reporter

Repeatable benchmark reports with SLO verdicts.

**Status:** implemented (Phase 9)

## Purpose

Repeatable benchmark reports with SLO verdicts.

## Provided

- `forgelab benchmark -url <url>` runs baseline, ramp, and spike profiles (`core/internal/report`, `core/internal/loadtest`) and writes `reports/benchmark-<timestamp>.md` and `.json` (git-ignored).
- Reports include host resources, the physical budget, the simulated cluster and scale factor, per-run status counts and latency percentiles, physical and virtual RPS, and pass/fail per SLO.
- `make benchmark URL=... [RPS=... STEP=...]`.

## Dependencies

- Load Generator
- Resource Virtualization Engine (optional, for the scale factor)

## Configuration

Flags: `-rps`, `-peak`, `-step-duration`, `-slo-p95` (default 250ms), `-slo-error-rate` (0.01), `-slo-drop-rate` (0.05), `-cluster`, `-out`, `-no-record`. A run that meets its SLOs completes the `s6-benchmark` learning exercise.
