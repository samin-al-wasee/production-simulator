# Hardware Calibration

Detects the physical resources of the host and computes the budget ForgeLab may spend on real containers and processes. It is the Physical Layer input to the Resource Virtualization Engine (ADR-0003).

**Status:** implemented (Phase 0.5, detection and budgeting; calibration benchmarks are not yet included)

## Purpose

Answers "what can this machine actually run?" so that later stages can derive a scale factor and virtual capacity without faking behavior.

## Provided

- `core/internal/calibration` — CPU cores, memory, and disk capacity. Container limits (cgroup v2 `cpu.max`, `memory.max`) take precedence over host totals. Linux only.
- `core/internal/budget` — splits the host into a reserve kept for the host and an allocatable remainder; `Remaining` checks workload demands against it.
- `forgelab host` — prints the budget as a table or JSON (`-json`).

## Dependencies

None. Pure Go, deterministic parsing; only `calibration.Detect` reads the operating system.

## Configuration

| Flag | Default | Meaning |
|---|---|---|
| `-reserve` | `0.25` | Fraction of each resource reserved for the host, in `[0, 1)` |
| `-disk-path` | `.` | Path whose filesystem capacity is reported |
| `-json` | off | Print the budget as JSON |

## Planned

- Throughput calibration (measured, not just detected, capacity) feeding the Scale Factor Engine.
- Per-resource reserve policies and named profiles.
