# Changelog

All notable changes to ForgeLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed

- Live mode, per ADR-0014: the sample application, every environment (Docker Compose lab and overlays, Kubernetes, AWS and GCP Terraform presets, cost guard), the scenario drills, helper scripts, application and cluster manifests and templates, the real-infrastructure component READMEs, the core packages for host calibration, resource budgeting, virtual clusters, scale factors, capacity, dual metrics, chaos, load testing, benchmark reports, retry semantics, cost guard, compliance, and manifest validation (with their CLI commands, API endpoints, and the JSON Schema dependency), and the dashboard's Overview and Experiments pages. Everything removed is on the `archive/live-lab` branch.

### Changed

- ForgeLab is the Production Sandbox (ADR-0014): the dashboard opens on the Sandbox; the learning path is rewritten as six stages of Sandbox and pipeline missions with manual completion; `forgelab serve` drops `-cluster`, `-reserve`, and `-enable-runs`; the documentation is re-scoped around the game (vision, architecture, principles, catalog, scenarios, roadmap, README, AGENTS.md).

### Added

- Dashboard browser tests: `@playwright/test` dev dependency, `dashboard/e2e/sandbox.spec.ts` (builds, wires, runs, scales, deletes, and resumes a Sandbox game, failing on any browser error), and `make test-e2e`.
- Phase 10 Sandbox API and dashboard: `/api/v1/sandbox/` (ruleset, games, commands, speed 0/1/2/4/8 ticks per second, step, save to `.forgelab/sandbox/`, replay from a save, Server-Sent Events stream) and a dashboard Sandbox page (build palette with drag-and-drop, `@xyflow/react` topology canvas, meters with SVG sparklines, component inspector, speed controls, save, resume on reload).
- Phase 10 Sandbox engine: `core/internal/sandbox` — ruleset `sandbox/v1` (ten placeable component kinds, three sizes, economy and growth tuning), validated commands (place, remove, connect, disconnect, resize, scale, move) with loop and budget checks, a two-pass flow solver (capacity-proportional routing, cache/CDN hit ratios, queue backlog and workers, utilization-driven latency, saturation drops), per-tick economy and meters (revenue, cost, cash, health, satisfaction, popularity, engagement, complexity, tier, userbase growth and churn), bankruptcy, and deterministic save/replay.
- Phase 10 started (Production Sandbox, ADR-0013 proposed): Sandbox mode documented in `docs/architecture.md`, `docs/vision.md`, and `docs/glossary.md`; Principle 10 scoped to Live mode and Principle 15 added; Sandbox domain in `docs/component-catalog.md` and `components/sandbox/`; Phase 10 milestones in `ROADMAP.md`.
- Phase 9 completed: `core/internal/report` and `forgelab benchmark` (baseline/ramp/spike with SLO verdicts, Markdown and JSON reports), `learning/path.yaml`, `core/internal/learning` and `forgelab learn` with automatic completion from passing experiments and benchmarks, API endpoints and a dashboard Learning path page, `scripts/e2e.sh` (`make e2e`), the `multi-failure-incident` scenario, and `templates/postmortem/postmortem.md` (ADR-0012).
- Phase 8 completed: security overlay (`compose.security.yaml`: TLS, headers, rate limiting, secret files), Kubernetes hardening (Pod Security `restricted`, non-root workloads, default-deny NetworkPolicies, RBAC roles, ingress TLS), `core/internal/compliance` and `core/internal/secretscan` with `forgelab security compliance|scan-secrets`, `scripts/security-smoke.sh`, `scripts/k8s-security-smoke.sh`, `scripts/compliance.sh`, six defensive attack-simulation scenarios, and `docs/security.md` (ADR-0011).
- Dual metrics now apply each resource's own virtual/physical ratio to its usage, so utilization matches in both views (previously the single binding factor could show virtual CPU above 100% on a lightly loaded host).
- Phase 7 completed: `core/internal/pipeline` and `forgelab pipeline run` (virtual-clock build/test/deploy simulation with rolling, canary, blue/green and rollback), `core/internal/api` and `forgelab serve`, the `dashboard/` Next.js app (Overview with Dual Metrics Mode, Experiments, Pipelines), and `make serve`/`dashboard-dev` (ADR-0010).
- Phase 6 completed: Terraform presets for AWS (EKS, RDS, ElastiCache, SQS+DLQ, Budgets) and GCP (GKE, Cloud SQL, Memorystore, Pub/Sub+DLQ, budget) with validated safe defaults, `environments/cloud/cost-guard.yaml`, `core/internal/costguard` and `forgelab costguard check`, `scripts/cloud-validate.sh`, `scripts/cloud-plan.sh` (never applies), and `make cloud-validate`/`cloud-plan` (ADR-0009).
- Phase 5 completed: `core/internal/chaos` and `forgelab chaos` (declared experiments with hypothesis probes), `core/internal/loadtest` and `forgelab loadtest` (constant/ramp/spike, open-loop, virtual RPS), outage drills for database, Redis, Kafka and network latency, `scripts/db-backup.sh`, `db-restore.sh`, `dr-drill.sh`, and `make chaos`/`loadtest`/`backup` (ADR-0008).
- Phase 4 completed: Kubernetes environment on `kind` (`environments/cloud/kubernetes/` — base, canary and blue-green strategies, HPA, ingress-nginx, metrics-server), `make k8s-up`/`k8s-down`, `scripts/k8s-*.sh`, four scenarios (rolling, canary, blue/green, HPA scale-out), and `/version`, `cpu_ms`, `APP_UNHEALTHY` in the sample app (ADR-0007).
- Phase 3 completed: `environments/local/compose.messaging.yaml` (Redis, RabbitMQ with declared retry/dead-letter topology, Kafka with retry and DLQ topics, exporters), Redis-backed `/api/cache` and `/api/limited` in the sample app, `core/internal/retry` and `forgelab retry`, `scripts/messaging-smoke.sh`, `scripts/dlq-requeue.sh`, `make up-msg`/`up-all`/`smoke-msg`, and new alerts (ADR-0006).
- Phase 2 completed: `environments/local/compose.observability.yaml` (Prometheus, Grafana, Loki + Promtail, Tempo, OpenTelemetry Collector, nginx and PostgreSQL exporters), reference dashboard and alerts, `make up-obs`, and a dependency-free telemetry layer plus `/api/work` in `applications/sample-web` (ADR-0005).
- Repository bootstrap: directory structure, conventions, and documentation.
- `environments/local/` — implemented `local` preset: Docker Compose stack (nginx reverse proxy + sample application + PostgreSQL) with container healthchecks and dependency ordering (Phase 1, ADR-0002).
- `environments/local/proxy/nginx.conf` — reverse-proxy configuration for the local preset.
- `environments/local/.env.example` — credential/port template for the local preset.
- Phase 0.5 completed: `core/internal/virtualcluster`, `scale`, `capacity`, `metrics`, `forgelab cluster`, and `manifests/cluster.example.yaml` (virtual cluster model, scale factor engine, capacity modeling, Dual Metrics translation).
- `core/internal/calibration`, `core/internal/budget`, and `forgelab host` — hardware detection (cgroup-aware) and physical resource budgeting (Phase 0.5, ADR-0003 accepted).
- `docs/development-loop.md`, `/dev-loop` command for OpenCode and Claude Code — agent-agnostic requirement → plan → implement → review → test → document loop.
- `applications/sample-web/` — minimal sample application (Go, stdlib only) behind the local proxy; `/healthz` reports database reachability.
- `components/networking/reverse-proxy/` and `components/databases/postgresql/` — component READMEs, marked implemented in `docs/component-catalog.md`.
- `Makefile` — `up`, `down`, `ps`, `logs` targets for the `local` stack.
- `docs/vision.md` — why ForgeLab exists and what problems it solves.
- `docs/architecture.md` — conceptual, composable, multi-layer architecture.
- `docs/principles.md` — design principles (platform over application, composition over configuration, production parity, etc.).
- `docs/repository-structure.md` — directory map and folder ownership.
- `docs/component-catalog.md` — catalog of planned production components (not implemented).
- `docs/scenarios.md` — planned incident, traffic, and reliability scenarios.
- `docs/learning-path.md` — progressive learning roadmap.
- `docs/glossary.md` — terminology reference.
- `docs/decisions/` — Architecture Decision Record process and first ADR (stack selection).
- `manifests/application.schema.yaml` — initial application manifest schema.
- `applications/`, `components/`, `environments/`, `scenarios/`, `scripts/`, `templates/` — placeholder directories with READMEs.
- `.github/` — issue templates, PR template, workflows placeholder.
- `AGENTS.md`, `CONTRIBUTING.md`, `.editorconfig`, `.gitignore`, `.env.example`, `Makefile`.
- Apache-2.0 `LICENSE`.
- `.devcontainer/` — development container (Go, Docker-in-Docker, Node LTS) for isolated local lab work.
- `core/` — Go simulation core, with `forgelab validate` for application manifests (`docs/decisions/0002-local-single-node-runtime.md`).
- `manifests/hello.example.yaml` and `manifests/shop-backend.example.yaml` — curated example manifests validated by the core CLI.
- `docs/decisions/0003-resource-virtualization-engine.md` — Resource Virtualization Engine ADR and Phase 0.5 simulation roadmap.
- `docs/architecture.md` — three-layer model (Physical / Simulation / Production View) and `simulation/` module layout.
- `docs/principles.md` — principles 9–12 (Physical ≠ Virtual, virtualize capacity, preserve dynamics, dual metrics).
- `docs/component-catalog.md` — Simulation domain: Resource Virtualization Engine, Capacity Engine, Scaling Model, Traffic Model, Hardware Calibration, Resource Profiles.
- `ROADMAP.md` — Phase 0.5 Simulation Foundation (planned).
- `docs/vision.md` — Production Systems Simulation and Engineering Platform framing.
- `docs/decisions/0004-synthetic-applications-workload-engine.md` — Synthetic Applications / Workload Engine ADR (behavior over features, Workload DSL, templates).
- `docs/architecture.md` — Synthetic Applications / Workload Engine subsystem: Mode A/B, Workload DSL, generator structure, dashboard integration.
- `docs/vision.md` — three inputs → one production model; "Production Systems Simulation and Engineering Platform" framing.
- `docs/principles.md` — principles 13–14 (system behavior over business functionality; business labels are visualization).
- `docs/component-catalog.md` — Synthetic Applications domain: Workload Engine, generators, Operation Engine, data/concurrency models, Telemetry Generator, Workload Templates.
- `docs/repository-structure.md` — `synthetic-apps/` planned layout.
- `docs/scenarios.md` — synthetic applications as scenario substrate; added cache/lock/exhaustion/eviction/external-API/retry/timeout/regression scenario rows.
- `docs/glossary.md` — Synthetic Application, Workload DSL, Production Operation, Feature Label, Workload Template.
- `ROADMAP.md` — Proposed Synthetic Applications roadmap (planning sketch, non-committal).

### Changed

- `ROADMAP.md` — Phase 1 (Local single-node) marked done.
- `README.md` — Status and Getting Started now document the runnable `local` preset.
- `docs/component-catalog.md`, `environments/` READMEs, `applications/` README — component and sample status moved from planned to implemented.
- `ROADMAP.md` — Phase 1 (Local single-node) marked in progress.
- `docs/architecture.md`, `docs/repository-structure.md`, `AGENTS.md` — document the local single-node envelope and `core/` placement.