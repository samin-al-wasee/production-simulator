# rolling-deployment

Category: failures · Status: implemented

## Goal

Understand how a rolling update protects availability, and what happens when a release is bad.

## Components involved

Kubernetes (Deployment `sample-web`, `maxUnavailable: 0`), Load Balancer (ingress-nginx), sample-web. The deliberately broken part is the new release (`APP_UNHEALTHY=1`).

Setup: `make k8s-up`.

## Expected symptoms

- A healthy release replaces pods one at a time; `/version` shifts from `v1` to `v2` with no failed requests.
- A bad release creates a new pod that never becomes Ready; the rollout stalls, old pods keep serving, and `kubectl rollout status` times out.

## Investigation

```sh
kubectl -n forgelab set env deploy/sample-web APP_VERSION=v2-bad APP_UNHEALTHY=1
kubectl -n forgelab rollout status deploy/sample-web --timeout=30s   # times out
kubectl -n forgelab get pods -l app=sample-web                       # new pod 0/1, old pods 1/1
kubectl -n forgelab describe pod <new-pod>                           # readiness probe failing on /healthz
for i in 1 2 3 4 5 6; do curl -s localhost:8081/version; echo; done  # still v1
kubectl -n forgelab rollout undo deploy/sample-web
```

## Success criteria

- You can explain why no traffic reached the bad pod (readiness gating plus `maxUnavailable: 0`).
- After `rollout undo` the deployment is fully available on the previous version.
- A good release (`APP_VERSION=v2 APP_UNHEALTHY-`) completes and `/version` reports `v2`.
