# privileged-workload-admission

Category: security · Status: implemented

**Defensive drill.** Run it only against your own local lab (localhost). Never point these commands at systems you do not own.

## Goal

Confirm the cluster refuses to run a privileged or otherwise insecure workload.

## Components involved

Kubernetes Pod Security admission (`restricted`), Scanner (compliance). Setup: `make k8s-up`.

## Expected symptoms

- Creating a privileged pod in the `forgelab` namespace is rejected with a `violates PodSecurity` message.
- `make compliance` passes for the shipped manifests and fails for a manifest you make insecure (for example `privileged: true`).

## Investigation

```sh
kubectl -n forgelab run bad --image=busybox:1.36 --restart=Never --overrides='{"spec":{"containers":[{"name":"bad","image":"busybox:1.36","securityContext":{"privileged":true}}]}}' -- sleep 1
make compliance
```

## Success criteria

- The admission error names the violated policy.
- You can explain the difference between admission-time (Pod Security) and pre-deploy (compliance check) enforcement.
