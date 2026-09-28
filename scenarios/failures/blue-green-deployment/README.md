# blue-green-deployment

Category: failures · Status: implemented

## Goal

Cut traffic between two complete environments instantly, and roll back just as fast.

## Components involved

Kubernetes, Deploy Controller (`strategies/blue-green`), sample-web (`v1-blue` and `v2-green`).

Setup: `make k8s-up`, then `kubectl -n forgelab delete deploy/sample-web hpa/sample-web pdb/sample-web` and `sh scripts/k8s-up.sh environments/cloud/kubernetes/strategies/blue-green`.

## Expected symptoms

- All requests report `v1-blue`.
- After `sh scripts/k8s-bluegreen-switch.sh green`, all requests report `v2-green` within a few seconds (ingress endpoints refresh with a short lag).
- Switching back to `blue` is the rollback.

## Investigation

```sh
for i in $(seq 1 6); do curl -s localhost:8081/version; echo; done | sort | uniq -c
sh scripts/k8s-bluegreen-switch.sh green
sleep 5; for i in $(seq 1 6); do curl -s localhost:8081/version; echo; done | sort | uniq -c
kubectl -n forgelab get svc sample-web -o jsonpath='{.spec.selector}'
sh scripts/k8s-bluegreen-switch.sh blue
```

## Success criteria

- You can state what a cutover changes (the Service selector only) and why rollback is instant.
- You observed the propagation lag and can explain it.
- Both colors ran at full capacity during the switch (the cost of the strategy).
