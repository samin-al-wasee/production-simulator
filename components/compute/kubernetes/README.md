# Kubernetes

Orchestration, scheduling, scaling, rollouts, and self-healing.

**Status:** implemented (Phase 4, kind-based `cloud`-style environment)

## Purpose

Orchestration, scheduling, scaling, rollouts, and self-healing.

## Provided

- Three-node kind cluster (`make k8s-up` / `make k8s-down`) with the sample application, PostgreSQL StatefulSet, PodDisruptionBudget, and Ingress declared in `environments/cloud/kubernetes/base/`.
- Zero-downtime rollouts: `RollingUpdate` with `maxSurge: 1`, `maxUnavailable: 0`, gated by readiness probes.

## Dependencies

- Docker (kind runs nodes as containers)
- kubectl, kind

## Configuration

`scripts/k8s-up.sh [overlay-dir]`; database credentials via `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` environment variables (defaults are local-only).
