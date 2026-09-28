# Cloud Environment

Managed-cluster presets: Kubernetes plus opt-in cloud services (AWS/GCP via Terraform).

**Status:** implemented — Kubernetes on `kind` (Phase 4, `kubernetes/`) and opt-in AWS/GCP Terraform presets with a cost guard (Phase 6, `terraform/`).

## Goals (placeholder)

- Runs the same manifests as `local/` and `staging/` without logic changes.
- Uses Terraform presets from `components/infrastructure/`.
- Branded with cost-guard rules: nothing runs destructively or at unapproved cost.

Contents will be added when cloud presets are authorized on the roadmap.