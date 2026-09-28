# hpa-scale-out

Category: scaling · Status: implemented

## Goal

Watch the Horizontal Pod Autoscaler react to CPU load and settle back.

## Components involved

Kubernetes (HPA `sample-web`, 2–8 replicas, 50% CPU), metrics-server, sample-web (`/api/work?cpu_ms=`).

Setup: `make k8s-up`; wait a few minutes until `kubectl -n forgelab get hpa` shows a CPU percentage instead of `<unknown>`.

## Expected symptoms

- CPU utilization jumps far above 50% and replicas grow from 2 toward 8 within about a minute.
- After the load stops, utilization falls and replicas remain high until the 60 s scale-down window passes.

## Investigation

```sh
seq 1 6000 | xargs -P 24 -I{} curl -s -o /dev/null 'localhost:8081/api/work?cpu_ms=100' &
watch -n 5 kubectl -n forgelab get hpa sample-web
kubectl -n forgelab top pods
kubectl -n forgelab describe hpa sample-web
```

## Success criteria

- You saw replicas reach the maximum and can explain why (target 50%, requests 50m).
- You saw scale-down after the stabilization window.
- You can say what limits scale-out (maxReplicas, the 200m CPU limit, the database).
