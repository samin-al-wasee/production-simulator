# GCP Preset

Managed-service variants of the ForgeLab components on Google Cloud.

**Status:** implemented (Phase 6; validated with `terraform validate`, not applied by the project)

## Purpose

Managed-service variants of the ForgeLab components on Google Cloud.

## Provided

- Regional GKE cluster and Spot node pool with Workload Identity.
- Optional Cloud SQL for PostgreSQL 16 (private IP, IAM authentication, no password), Memorystore Redis, Pub/Sub topic and subscription with a dead-letter topic.
- Optional project budget when `billing_account` is set; default labels on every resource.

## Dependencies

- Terraform
- A dedicated Google Cloud project and credentials

## Configuration

`environments/cloud/terraform/presets/gcp/variables.tf`. Required inputs: `project_id`, `owner`. Pub/Sub dead-letter needs at least 5 delivery attempts.
