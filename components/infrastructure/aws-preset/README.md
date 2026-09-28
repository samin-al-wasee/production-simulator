# AWS Preset

Managed-service variants of the ForgeLab components on AWS.

**Status:** implemented (Phase 6; validated with `terraform validate`, not applied by the project)

## Purpose

Managed-service variants of the ForgeLab components on AWS.

## Provided

- EKS cluster and Spot managed node group in public subnets (no NAT gateway).
- Optional RDS for PostgreSQL 16 (password generated into Secrets Manager), ElastiCache Redis (encrypted), SQS queue with dead-letter queue (`maxReceiveCount` 3, mirroring the local RabbitMQ delivery limit).
- AWS Budgets for the experiment tag; provider default tags on every resource.

## Dependencies

- Terraform
- AWS account and credentials

## Configuration

`environments/cloud/terraform/presets/aws/variables.tf`; instance classes, node counts, TTL, and budget are validated. Required input: `owner`.
