# Autoscaler (HPA)

Horizontal scaling on CPU utilization.

**Status:** implemented (Phase 4, kind-based `cloud`-style environment)

## Purpose

Horizontal scaling on CPU utilization.

## Provided

- `HorizontalPodAutoscaler` for sample-web: 2–8 replicas, 50% average CPU target, fast scale-up (up to 4 pods / 15 s), 60 s scale-down stabilization.
- metrics-server installed by `scripts/k8s-up.sh`.
- Drill: `scenarios/scaling/hpa-scale-out/`.

## Dependencies

- Kubernetes
- metrics-server

## Configuration

`environments/cloud/kubernetes/base/hpa.yaml`. Pods must declare CPU requests, which the base Deployment does.
