# Load Balancer

Traffic distribution across replicas with health-based routing.

**Status:** implemented (Phase 4, kind-based `cloud`-style environment)

## Purpose

Traffic distribution across replicas with health-based routing.

## Provided

- ingress-nginx controller (pinned v1.11.3) as the cluster entry point, published on host port 8081.
- Kubernetes Services balance across ready pods only; readiness probes remove unhealthy pods from rotation.
- Weighted canary routing through ingress annotations.

## Dependencies

- Kubernetes

## Configuration

`environments/cloud/kubernetes/base/ingress.yaml`; controller installed by `scripts/k8s-up.sh`.
