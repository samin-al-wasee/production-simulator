# Kubernetes environment

A `cloud`-style environment on Kubernetes that runs on a laptop with `kind` (ADR-0007).

**Status:** implemented (Phase 4).

## Requirements

`docker`, `kind`, and `kubectl` on `PATH`. Add-on manifests (ingress-nginx, metrics-server) are downloaded on first run.

## Quick start

```sh
make k8s-up      # cluster + add-ons + base manifests
curl localhost:8081/version
make k8s-down    # delete the cluster
```

Ingress is published on host port **8081** (the `local` preset uses 8080).

## Layout

```text
environments/cloud/kubernetes/
├── kind-config.yaml         1 control plane + 2 workers, ingress port mapping
├── base/                    namespace, PostgreSQL, sample-web, HPA, PDB, Ingress
└── strategies/
    ├── canary/              second Deployment + weighted canary Ingress
    └── blue-green/          blue and green Deployments behind a switchable Service
```

## Strategies

```sh
sh scripts/k8s-up.sh environments/cloud/kubernetes/strategies/canary
sh scripts/k8s-up.sh environments/cloud/kubernetes/strategies/blue-green
sh scripts/k8s-bluegreen-switch.sh green
```

Overlays include `base/`. Deleting an overlay with `kubectl delete -k` therefore also deletes the base; remove individual objects instead. Moving from `base` to `blue-green` leaves the base Deployment running, so delete it first: `kubectl -n forgelab delete deploy/sample-web hpa/sample-web pdb/sample-web`.

## Secrets

The `forgelab-db` Secret is created by `scripts/k8s-up.sh` from `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` (local-only defaults). It is never stored in the manifests.

## Drills

`scenarios/failures/rolling-deployment/`, `canary-deployment/`, `blue-green-deployment/`, and `scenarios/scaling/hpa-scale-out/`.
