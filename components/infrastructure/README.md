# Infrastructure Components

Infrastructure-as-Code and cloud presets: Terraform, managed services, and drift control.

**Status:** cataloged in `docs/component-catalog.md` — nothing implemented yet.

## Planned components

| Component | Provides | Status |
|---|---|---|
| Terraform | Infrastructure-as-Code on cloud presets | planned |
| AWS Preset | Managed-service variants (EKS, RDS, SQS, …) | planned |
| GCP Preset | Managed-service variants (GKE, CloudSQL, Pub/Sub, …) | planned |
| Config Sync | Versioned drift detection and reconciliation | planned |

The platform is **cloud agnostic**: these are opt-in layers, never the baseline.