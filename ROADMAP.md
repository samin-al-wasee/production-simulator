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

## Phase 1 — Local single-node *🔨 in progress*

> Run a user-provided application in a production-like local environment.

- [ ] `local` environment preset (Docker Compose)
- [ ] Minimal runtime: reverse proxy + one application + one database
- [ ] Application manifest validation tooling
- [ ] Health checks and basic entrypoint scripts

## Phase 2 — Observability

> Every component observable by default.

- [ ] Prometheus + Grafana
- [ ] Structured logging (Loki)
- [ ] Distributed tracing (Tempo + OpenTelemetry)
- [ ] Reference dashboards and alerts

## Phase 3 — Messaging & data

- [ ] Redis (cache / pub-sub / rate limiting)
- [ ] RabbitMQ (queues, work distribution)
- [ ] Kafka (streams, consumer groups)
- [ ] Dead-letter queues and retry semantics

## Phase 4 — Kubernetes

- [ ] `cloud`-style environment on Kubernetes
- [ ] Load balancer / ingress setup
- [ ] Horizontal Pod Autoscaling
- [ ] Rolling, canary, and blue/green deployment scenarios

## Phase 5 — Reliability & chaos

- [ ] Chaos injection tooling
- [ ] Outage drills (database, Redis, Kafka)
- [ ] Load testing harness
- [ ] Disaster-recovery drills and backup/restore

## Phase 6 — Cloud presets

- [ ] Terraform modules (AWS, GCP)
- [ ] Managed services variants of components
- [ ] Cost-guard rules for experiments

## Phase 7 — CI/CD & dashboard

- [ ] Pipeline simulation (build → test → deploy)
- [ ] Next.js dashboard: launch, monitor, inspect experiments

## Phase 8 — Security

- [ ] Attack simulation scenarios
- [ ] Secrets management, TLS, RBAC
- [ ] Compliance-oriented checks

## Phase 9 — Production Simulator

- [ ] The complete ForgeLab experience, end to end
- [ ] Benchmark reports and learning-path completion tracking

---

## How phases are advanced

1. Documentation for the phase lands first (architecture + catalog updates).
2. Architectural changes get an ADR in `docs/decisions/`.
3. Implementation lands incrementally, small and reviewable.
4. Each phase ends with a `CHANGELOG.md` entry.