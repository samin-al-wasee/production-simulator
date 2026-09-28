# Resource Virtualization Engine

Reconciles the host's physical budget with the simulated production cluster (ADR-0003). It virtualizes capacity; it never fakes behavior.

**Status:** implemented (Phase 0.5)

## Purpose

Lets a laptop present a production-scale cluster: nodes, CPU, memory, disk, and RPS are shown in virtual terms, always labelled with the active scale factor.

## Provided

- `core/internal/virtualcluster` — the declared cluster (`kind: Cluster`, see `manifests/cluster.example.yaml`): node groups built from built-in profiles (`small`, `medium`, `large`, `xlarge`) or inline resources, plus an optional virtual database tier.
- `core/internal/scale` — per-resource virtual/physical ratios; the effective factor is the binding ratio rounded up to a power of two (x1, x2, … x128). Converts quantities such as RPS in both directions.
- `core/internal/metrics` — Dual Metrics Mode: physical measurements and derived virtual values side by side, each tagged with its kind, plus the scale factor.
- `forgelab cluster <file>` — prints or emits (`-json`) the dual view.

## Dependencies

Hardware Calibration (physical budget). Pure Go and deterministic.

## Configuration

`forgelab cluster [-reserve 0.25] [-disk-path .] [-rps N] [-json] <cluster-file>`

## Notes

Virtual usage applies each resource's own virtual/physical capacity ratio to its physical usage, so utilization is the same in both views and bottlenecks are preserved; the power-of-two scale factor is the label and is used for RPS. Host-level disk usage includes data that is not ForgeLab's, so it can exceed the allocatable budget.
