# network-segmentation-breach

Category: security · Status: implemented

**Defensive drill.** Run it only against your own local lab (localhost). Never point these commands at systems you do not own.

## Goal

Show that a compromised or misplaced pod cannot reach the database, and that the block is caused by policy.

## Components involved

Kubernetes NetworkPolicies (default deny plus allows), postgres, sample-web. Setup: `make k8s-up`.

## Expected symptoms

- A pod labelled like the application reaches postgres:5432 (positive control).
- A pod with any other label cannot reach postgres or the application directly.

## Investigation

```sh
sh scripts/k8s-security-smoke.sh   # runs the control and the intruder probes
kubectl -n forgelab get networkpolicy
kubectl -n forgelab describe networkpolicy postgres-from-web
```

## Success criteria

- The control succeeds and the intruder fails, so the failure is attributable to the policy.
- You can trace which policy allows each hop of ingress → web → postgres and which denies everything else.
