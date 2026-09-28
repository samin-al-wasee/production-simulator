# Load Generator

Repeatable traffic profiles and benchmarks.

**Status:** implemented (Phase 5, `local` preset)

## Purpose

Repeatable traffic profiles and benchmarks.

## Provided

- `forgelab loadtest` (`core/internal/loadtest`): constant, ramp, and spike profiles; open-loop dispatch with dropped-request accounting; latency min/mean/p50/p90/p95/p99/max; status counts; error rate; virtual RPS via `-scale`.
- `make loadtest URL=... RPS=... DURATION=... [PROFILE=ramp RAMP_TO=...] [SCALE=...]`.

## Dependencies

- Networking (any HTTP target)

## Configuration

Flags: `-url`, `-rps`, `-profile`, `-ramp-to`, `-duration`, `-max-in-flight`, `-timeout`, `-scale`, `-json`.
