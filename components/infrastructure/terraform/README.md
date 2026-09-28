# Terraform

Infrastructure as Code for the cloud presets, with offline validation and a plan-time cost guard.

**Status:** implemented (Phase 6; validated with `terraform validate`, not applied by the project)

## Purpose

Infrastructure as Code for the cloud presets, with offline validation and a plan-time cost guard.

## Provided

- Provider-pinned root modules for AWS and GCP under `environments/cloud/terraform/presets/`.
- `make cloud-validate` (fmt, init without backend, validate; no credentials, creates nothing).
- `make cloud-plan PRESET=<aws|gcp>`: plan, then `forgelab costguard check`. The repository has no apply target.
- Cost guard: `environments/cloud/cost-guard.yaml` and `core/internal/costguard`.

## Dependencies

- terraform >= 1.6
- Cloud credentials (plan only)

## Configuration

Copy `terraform.tfvars.example` to `terraform.tfvars` in the preset (git-ignored). Rules: `environments/cloud/cost-guard.yaml`.
