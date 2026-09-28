# ForgeLab Roadmap

This document is the source of truth for **what is authorized and what is next**. A feature may only be implemented when the phase it belongs to is active.

Legend: ⬜ planned · 🔨 in progress · ✅ done

## Phase 0 — Foundation *(current)*

> Bootstrap the repository, documentation, architecture, conventions, and directory structure. No infrastructure implementation.

- [x] Repository bootstrap (this structure)
- [x] Vision, architecture, principles, glossary (`docs/`)
- [x] Component catalog — planned component table (`docs/component-catalog.md`)
- [x] Scenario catalog (`docs/scenarios.md`)
- [x] Learning path (`docs/learning-path.md`)
- [x] Application manifest schema (`manifests/application.schema.yaml`)
- [x] ADR process (`docs/decisions/`)
- [x] Contribution and agent conventions (`CONTRIBUTING.md`, `AGENTS.md`)
- [x] Getting-started placeholder in `README.md`

## Phase 0.5 — Simulation Foundation *✅ done*

> Introduce the **Resource Virtualization Engine** so ForgeLab can represent production systems much larger than the host hardware. No production components yet; this phase is the virtual resource model under everything else.

- [x] Hardware detection and calibration (`core/internal/calibration`, `forgelab host`)
- [x] Physical resource budgeting (`core/internal/budget`)
- [x] Virtual cluster model (`core/internal/virtualcluster`)
- [x] Scale factor engine (`core/internal/scale`)
- [x] Capacity modeling (CPU, RAM, storage, database) (`core/internal/capacity`)
- [x] Physical vs virtual metrics (Dual Metrics Mode) (`core/internal/metrics`, `forgelab cluster`; the dashboard surface lands with Phase 7)

## Phase 1 — Local single-node *✅ done*

> Run a user-provided application in a production-like local environment.

- [x] `local` environment preset (Docker Compose)
- [x] Minimal runtime: reverse proxy + one application + one database
- [x] Application manifest validation tooling
- [x] Health checks and basic entrypoint scripts (container healthchecks, `make up`/`down`/`logs`/`ps`)

## Phase 2 — Observability *✅ done*

> Every component observable by default.

- [x] Prometheus + Grafana
- [x] Structured logging (Loki)
- [x] Distributed tracing (Tempo + OpenTelemetry)
- [x] Reference dashboards and alerts

## Phase 3 — Messaging & data *✅ done*

- [x] Redis (cache / rate limiting; pub/sub is available in the server, not yet exercised)
- [x] RabbitMQ (queues, work distribution)
- [x] Kafka (streams, consumer groups)
- [x] Dead-letter queues and retry semantics

## Phase 4 — Kubernetes *✅ done*

- [x] `cloud`-style environment on Kubernetes (`environments/cloud/kubernetes/`, kind)
- [x] Load balancer / ingress setup (ingress-nginx)
- [x] Horizontal Pod Autoscaling
- [x] Rolling, canary, and blue/green deployment scenarios

## Phase 5 — Reliability & chaos *✅ done*

- [x] Chaos injection tooling (`core/internal/chaos`, `forgelab chaos`)
- [x] Outage drills (database, Redis, Kafka)
- [x] Load testing harness (`core/internal/loadtest`, `forgelab loadtest`)
- [x] Disaster-recovery drills and backup/restore

## Phase 6 — Cloud presets *✅ done*

- [x] Terraform modules (AWS, GCP) (`environments/cloud/terraform/presets/`)
- [x] Managed services variants of components (RDS/Cloud SQL, ElastiCache/Memorystore, SQS/Pub/Sub)
- [x] Cost-guard rules for experiments (`environments/cloud/cost-guard.yaml`, `forgelab costguard`)

## Phase 7 — CI/CD & dashboard *✅ done*

- [x] Pipeline simulation (build → test → deploy) (`core/internal/pipeline`, `forgelab pipeline run`)
- [x] Next.js dashboard: launch, monitor, inspect experiments (`dashboard/`, `forgelab serve`)

## Phase 8 — Security

- [ ] Attack simulation scenarios
- [ ] Secrets management, TLS, RBAC
- [ ] Compliance-oriented checks

## Phase 9 — Production Simulator

- [ ] The complete ForgeLab experience, end to end
- [ ] Benchmark reports and learning-path completion tracking

---

## Proposed: Synthetic Applications roadmap (planning sketch)

> **Non-committal.** These are the currently proposed phases for the **Synthetic Applications / Workload Engine** subsystem (`docs/decisions/0004-synthetic-applications-workload-engine.md`). They are **not locked commitments** and do **not** replace the authoritative phases above. When authorized, this work is folded into the numbered phases. Phases listed in the sketch's "current proposed" order, not implementation order.

- **Phase 1 — Synthetic Application Foundation:** Workload DSL, service model, endpoint model, operation model, synthetic service generator, basic HTTP workloads
- **Phase 2 — Production Operations:** PostgreSQL operations, Redis operations, RabbitMQ operations, Kafka operations, background workers, concurrency, transactions, external calls
- **Phase 3 — Production Infrastructure:** load balancer, reverse proxy, API gateway, containers, Kubernetes, service discovery, networking
- **Phase 4 — Resource Virtualization:** physical resource detection, virtual resource model, capacity scaling, logical RPS scaling, virtual replicas, virtual memory/CPU/storage
- **Phase 5 — Scenarios and Reliability:** failure injection, chaos, traffic manipulation, scaling scenarios, incidents, recovery
- **Phase 6 — Dashboard:** application builder, topology visualization, live metrics, scenario controls, resource controls

Note: the sketch's "Phase 4 — Resource Virtualization" covers the same territory as the authoritative **Phase 0.5 — Simulation Foundation** above; only one of the two will be active once this work is authorized.

---

## How phases are advanced

1. Documentation for the phase lands first (architecture + catalog updates).
2. Architectural changes get an ADR in `docs/decisions/`.
3. Implementation lands incrementally, small and reviewable.
4. Each phase ends with a `CHANGELOG.md` entry.