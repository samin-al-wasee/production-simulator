# canary-deployment

Category: failures · Status: implemented

## Goal

Expose a new release to a small share of traffic, then promote or abort.

## Components involved

Kubernetes, Load Balancer (ingress-nginx canary annotations), Deploy Controller (`strategies/canary`), sample-web.

Setup: `make k8s-up`, then `sh scripts/k8s-up.sh environments/cloud/kubernetes/strategies/canary`.

## Expected symptoms

- About 20% of requests report `v2-canary`; the rest report `v1`.
- Raising the weight to 100 sends every request to the canary; removing the canary Ingress returns all traffic to stable.

## Investigation

```sh
for i in $(seq 1 100); do curl -s localhost:8081/version; echo; done | sort | uniq -c
kubectl -n forgelab annotate ingress sample-web-canary nginx.ingress.kubernetes.io/canary-weight=50 --overwrite
# abort: remove only the canary objects (never `kubectl delete -k`, which also deletes the base)
kubectl -n forgelab delete ingress/sample-web-canary svc/sample-web-canary deploy/sample-web-canary
```
Compare error ratio and latency for the canary in Grafana (`make up-obs` covers the Compose stack; in Kubernetes use `kubectl logs`).

## Success criteria

- You measured the split (roughly 80/20) and changed it.
- You aborted the canary and confirmed 100% of traffic is back on `v1`.
