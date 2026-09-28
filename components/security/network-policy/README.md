# Network Policy

East-west segmentation and RBAC on Kubernetes.

**Status:** implemented (Phase 8)

## Purpose

East-west segmentation and RBAC on Kubernetes.

## Provided

- Default-deny ingress and egress plus explicit allows: ingress-nginx → sample-web:8080, sample-web → postgres:5432, DNS (`base/network-policies.yaml`). Enforcement was verified on kind, with a positive control.
- RBAC (`base/rbac.yaml`): `forgelab-viewer` (read-only) and `forgelab-operator` for groups `forgelab:viewers` and `forgelab:operators`; no access to Secrets, `pods/exec`, or RBAC; the application service account has no API permissions.
- Pod Security admission `restricted` on the namespace.

## Dependencies

- Kubernetes

## Configuration

Bind the roles to your identity provider's groups. Verify with `sh scripts/k8s-security-smoke.sh`.
