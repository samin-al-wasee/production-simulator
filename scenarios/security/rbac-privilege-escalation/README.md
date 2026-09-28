# rbac-privilege-escalation

Category: security · Status: implemented

**Defensive drill.** Run it only against your own local lab (localhost). Never point these commands at systems you do not own.

## Goal

Verify that read-only and operator identities cannot escalate to secrets, exec, or RBAC changes.

## Components involved

Kubernetes RBAC (`forgelab-viewer`, `forgelab-operator`). Setup: `make k8s-up`.

## Expected symptoms

- Viewers can read pods but cannot delete pods, patch deployments, or read secrets.
- Operators can delete pods and patch deployments but cannot read secrets, exec into pods, or create role bindings.
- The application service account has no API access.

## Investigation

```sh
kubectl auth can-i get secrets -n forgelab --as=alice --as-group=forgelab:viewers
kubectl auth can-i patch deployments -n forgelab --as=bob --as-group=forgelab:operators
kubectl auth can-i create rolebindings -n forgelab --as=bob --as-group=forgelab:operators
sh scripts/k8s-security-smoke.sh
```

## Success criteria

- Every escalation attempt is answered `no`.
- You can explain why granting `secrets` read or `pods/exec` would defeat the other controls.
