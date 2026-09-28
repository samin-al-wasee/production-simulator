# Infrastructure Components

Infrastructure-as-Code and cloud presets: Terraform, managed services, and drift control.

**Status:** partially implemented — Terraform, AWS Preset, and GCP Preset ship in Phase 6 (ADR-0009); Config Sync is planned.

## Components

| Component | Provides | Status |
|---|---|---|
| Terraform | Infrastructure-as-Code on cloud presets | implemented — see [terraform/](terraform/) |
| AWS Preset | Managed-service variants (EKS, RDS, SQS, …) | implemented — see [aws-preset/](aws-preset/) |
| GCP Preset | Managed-service variants (GKE, CloudSQL, Pub/Sub, …) | implemented — see [gcp-preset/](gcp-preset/) |
| Config Sync | Versioned drift detection and reconciliation | planned |

The platform is **cloud agnostic**: these are opt-in layers, never the baseline.