# Cloud presets (Terraform)

Opt-in AWS and GCP presets with managed-service variants of ForgeLab components and a plan-time cost guard (ADR-0009).

**Status:** implemented (Phase 6). Validated with `terraform validate` and unit tests; **not applied to a real account by this project**.

## Safety model

- **Nothing here is applied automatically.** The repository has no `apply` target. `make cloud-plan` plans and runs the cost guard, then stops.
- Resources are tagged/labelled `forgelab-experiment`, `forgelab-owner`, and `forgelab-ttl-hours` (default 8, at most 72). Destroy the experiment when the TTL expires; the tag exists so you can find it.
- Variable validation limits instance classes, node counts, TTL, and budget. Workers default to Spot; no NAT gateway; databases are private.
- No credentials in variables or git: AWS RDS generates its password into Secrets Manager; Cloud SQL uses IAM authentication.
- Budgets: AWS Budgets always; a GCP budget when `billing_account` is set.

## Layout

```text
environments/cloud/
├── cost-guard.yaml          rules and price table for `forgelab costguard`
└── terraform/presets/
    ├── aws/                 EKS + optional RDS, ElastiCache, SQS(+DLQ), Budgets
    └── gcp/                 GKE + optional Cloud SQL, Memorystore, Pub/Sub(+DLQ), budget
```

Managed services map to local components: RDS / Cloud SQL ↔ PostgreSQL, ElastiCache / Memorystore ↔ Redis, SQS / Pub/Sub with a dead-letter target ↔ the RabbitMQ delivery-limit topology.

## Workflow

```sh
make cloud-validate                      # offline: fmt, init -backend=false, validate
cp environments/cloud/terraform/presets/aws/terraform.tfvars.example \
   environments/cloud/terraform/presets/aws/terraform.tfvars   # edit: owner, region, ...
make cloud-plan PRESET=aws               # plan + cost guard; never applies
```

If the guard accepts the plan, review it and apply manually with `terraform -chdir=<preset> apply tfplan`. Destroy with `terraform -chdir=<preset> destroy` when finished.

## Cost guard

`forgelab costguard check [-rules file] plan.json` reads `terraform show -json` output and enforces: estimated monthly ceiling, denied resource types, required tags, maximum TTL, and priced/known resources only. Prices are approximate list prices maintained in `cost-guard.yaml` (they exclude storage, data transfer, and taxes); cloud budget alerts remain the source of truth for real spend. Default plans cost about $118/month (AWS) and $138/month (GCP) by this estimate, dominated by the Kubernetes control plane.

## Composition

`enable_database`, `enable_cache`, `enable_queue` switch each managed service; only the cluster is required. Details per preset are in `variables.tf`.
