# ADR-0007: Kubernetes environment: kind cluster with Kustomize manifests (Phase 4)

**Status:** superseded by [ADR-0014](0014-sandbox-only-platform.md) (was accepted; the code it describes is on the `archive/live-lab` branch)
**Date:** 2026-09-28

## Context

Phase 4 needs a `cloud`-style environment on Kubernetes with ingress, autoscaling, and deployment-strategy drills. It must run on a laptop (local-first, cloud agnostic), be declared as code, and reuse the same sample application and database as the `local` preset. Managed-cluster variants come later (Phase 6).

## Decision

1. **Cluster:** a three-node `kind` cluster (`environments/cloud/kubernetes/kind-config.yaml`; one control plane published on host port 8081, two workers) run inside the existing Docker-in-Docker devcontainer. Required tools: `docker`, `kind`, `kubectl`; they are prerequisites, not vendored.
2. **Manifests:** plain Kubernetes YAML composed with **Kustomize** (`kubectl apply -k`), the tool built into kubectl, so no additional tooling or templating language is introduced. `base/` holds the namespace, PostgreSQL StatefulSet, the sample-web Deployment, Service, PodDisruptionBudget, HorizontalPodAutoscaler, and an nginx Ingress.
3. **Cluster add-ons** are installed by `scripts/k8s-up.sh` from pinned upstream manifests: ingress-nginx (controller v1.11.3, kind provider) and metrics-server (v0.7.2, with `--kubelet-insecure-tls` because kind kubelets use self-signed certificates).
4. **Secrets stay out of git:** the `forgelab-db` Secret is created by the script from environment variables (defaults are local-only).
5. **Deployment strategies as overlays:** the base Deployment uses `RollingUpdate` with `maxUnavailable: 0`; `strategies/canary/` adds a canary Deployment and a weighted canary Ingress; `strategies/blue-green/` runs two colored Deployments behind a Service whose selector is switched by `scripts/k8s-bluegreen-switch.sh`.
6. **Release identity without rebuilds:** the sample application reports `APP_VERSION` on `/version` and can be made unhealthy with `APP_UNHEALTHY=1`, so releases are simulated by changing environment variables of one image.
7. **Scenarios** implemented on this environment: rolling deployment, canary deployment, blue/green deployment, and HPA scale-out.

## Consequences

### Positive

- Everything is reproducible with `make k8s-up` and removable with `make k8s-down`.
- Real Kubernetes behavior (readiness gating, HPA reconciliation, ingress weighting) is exercised, not mocked.
- The same manifests are the starting point for cloud clusters in Phase 6.

### Negative / Trade-offs

- Requires `kind` and `kubectl`, and add-on manifests are fetched from the internet on first run.
- metrics-server takes a few minutes to report the first CPU samples, so HPA shows `<unknown>` initially.
- Applying an overlay after `base` leaves the base Deployment behind; drills must delete it first (documented).
- Ingress endpoint updates lag a few seconds behind a Service selector switch.

## Alternatives Considered

- **Helm charts** — more flexible but adds a templating language and a tool dependency; Kustomize is enough for overlays.
- **k3d / minikube** — comparable; kind runs cleanly in Docker-in-Docker and is the Kubernetes project's own tool.
- **Argo Rollouts / Flagger for canary** — appropriate for progressive delivery later; plain Deployments and ingress annotations expose the mechanics first.
- **Service mesh traffic splitting** — heavier than needed for the drills.

## References

- `ROADMAP.md` Phase 4 — Kubernetes
- `docs/decisions/0002-local-single-node-runtime.md`
- `docs/decisions/0005-observability-overlay.md`
