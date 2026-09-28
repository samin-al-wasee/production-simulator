# Capacity Engine

Models virtual capacity and what happens as demand approaches it (ADR-0003).

**Status:** implemented (Phase 0.5)

## Purpose

Makes exhaustion a first-class mechanic: utilization, pressure, saturation, and time to exhaustion are computed the same way whatever the scale factor.

## Provided

- `core/internal/capacity` — `Capacity` (compute plus database connections, QPS, storage), `Demand`, `Utilize`, `Saturated` (>= 100%), `Pressured` (>= 80%), and `TimeToExhaustion` for a growing resource such as storage.

## Dependencies

Resource Virtualization Engine (`virtualcluster.Cluster.Capacity()` supplies the capacity).

## Configuration

Database capacity is declared in the cluster spec's `database` block.
