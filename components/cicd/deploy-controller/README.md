# Deploy Controller

Rolling, canary, and blue/green rollout strategies.

**Status:** implemented (Phase 4, kind-based `cloud`-style environment)

## Purpose

Rolling, canary, and blue/green rollout strategies.

## Provided

- Rolling: the base Deployment strategy.
- Canary: `strategies/canary/` (second Deployment plus weighted canary Ingress; default weight 20).
- Blue/green: `strategies/blue-green/` and `scripts/k8s-bluegreen-switch.sh <blue|green>`.
- Drills: `scenarios/failures/rolling-deployment/`, `canary-deployment/`, `blue-green-deployment/`.

## Dependencies

- Kubernetes
- Load Balancer

## Configuration

Apply a strategy with `sh scripts/k8s-up.sh environments/cloud/kubernetes/strategies/<name>`.
