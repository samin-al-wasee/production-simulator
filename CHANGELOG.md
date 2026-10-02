# Changelog

All notable changes to ForgeLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed

- Live mode, per ADR-0014: the sample application, every environment (Docker Compose lab and overlays, Kubernetes, AWS and GCP Terraform presets, cost guard), the scenario drills, helper scripts, application and cluster manifests and templates, the real-infrastructure component READMEs, the core packages for host calibration, resource budgeting, virtual clusters, scale factors, capacity, dual metrics, chaos, load testing, benchmark reports, retry semantics, cost guard, compliance, and manifest validation (with their CLI commands, API endpoints, and the JSON Schema dependency), and the dashboard's Overview and Experiments pages. Everything removed is on the `archive/live-lab` branch.

### Changed

- Code folders renamed (ADR-0015): `core/` → `backend/` (Go module `github.com/samin-al-wasee/production-simulator/backend`) and `dashboard/` → `frontend/`; Make targets `dashboard-dev` → `frontend-dev` and `build-core` → `build-backend`. No behavior change; current docs, tooling, and the secret-scan allowlist follow the new paths.
- Economy rebalanced in `sandbox/v2`. Revenue per successful request falls from $0.0005 to $0.00015, and starting cash rises from $1,000 to $1,500.
  - Under v1 over-provisioning always paid; under v2 a design with about 1.5× headroom earns the most.
  - `core/internal/sandbox/balance_test.go` pins the balance: sensible headroom stays solvent and earns the most, no headroom and 5× over-provisioning both cost money, and unanswered events cost money.
- Rollback is not a Sandbox response, because the Sandbox has no releases. It stays in the pipeline simulator.
- ForgeLab is the Production Sandbox (ADR-0014): the dashboard opens on the Sandbox; the learning path is rewritten as six stages of Sandbox and pipeline missions with manual completion; `forgelab serve` drops `-cluster`, `-reserve`, and `-enable-runs`; the documentation is re-scoped around the game (vision, architecture, principles, catalog, scenarios, roadmap, README, AGENTS.md).

### Added

- Phase 11.4: connections and inter-service communication (ADR-0019), in ruleset `sandbox/v7`. New games use v7; v1 to v6 replay bit for bit, now pinned by `testdata/replay.golden`.
  - **Listeners and connections:** every component that takes connections listens on a protocol, port, and TLS (SQL, RESP, S3, AMQP, or the app's own); every edge gets a client side with a pool, timeout, and retries, adopted on connect and following its target. A mismatch refuses its calls with the reason on the edge.
  - **Service calls:** routes call other services' endpoints by name, synchronously or asynchronously; applications connect to applications. Load is carried per endpoint, so each service sees exactly what its callers call, and success and latency flow back.
  - **Outcomes:** a network hop, the connection's timeout, and retries that recover failures and amplify load; a pool smaller than the calls in flight is a bottleneck (`pool:<target>`).
  - **Reported:** per-connection attempts, retries, failures, latency, and problem (`flow.edges`).
  - **Templates:** Storefront, Catalog, Orders, Payments, and Notifications application types.
  - **Dashboard:** a connection inspector with **Configure connection**, listeners on data components, apps titled by service name, failing edges in red, connections in the app's inside view, and calls in the app form.
- App templates (Phase 11): placing an application instance offers an application type (e-commerce, flight booking, ride sharing, social feed, video streaming: routes with costs, dependencies, and typical shares) on a stack (Django, FastAPI, Express, Rails, Go), or a manual configuration. `place` can carry the instance's `AppConfig`, validated before anything is paid. Routes gain a typical `share`, which a connecting traffic component adopts. The templates are v6 ruleset data the engine never reads.
- Phase 11: traffic components (ADR-0018), in ruleset `sandbox/v6`. New games use v6, and v1 to v5 replay bit for bit.
  - **Empty start:** a v6 game has no node. The Internet becomes **Traffic**, a free component the player places any number of times.
  - **One population each:** `configure` on a traffic component sets a single client type, region, protocol, scheme, port, keep-alive, client timeout, retries, and source (market or load test), plus a weighted endpoint mix. The market's volume is split by client-type and region share; components of one segment split it.
  - **Adopting on connect:** a new traffic component has no endpoints. Connecting it to an application instance sets its protocol, port, scheme, and keep-alive to the app's and its endpoints to one per route (catch-all aside) in equal shares; reconfiguring the app updates its connected traffic, and disconnecting or removing the app clears them again. The player can change them while connected.
  - **Contract:** a traffic component connects to exactly one application instance. A protocol, port, or TLS mismatch refuses every request before it reaches the app; an endpoint with no route fails with 404 (the `*` route is optional in v6); keep-alive needs both sides; the shorter timeout decides success. The app's protocol and port become model inputs.
  - **Per-application mix:** each app's endpoint mix comes from its inputs, closing ADR-0017's global-mix gap. Retries are tracked per component.
  - **Events:** traffic cards and the DDoS hit one or more traffic components, drawn by the seed; component cards never hit them.
  - **Aggregated:** meters and revenue add up every component. The flow's `traffic` adds RPS by client type and component, and each traffic node reports RPS, retries, successes, failures by reason, latency, concurrency, and the contract problem. Sentiment holds still while no real traffic flows.
  - **Catalog:** CDN, load balancer, and API gateway leave v6 until they have a traffic contract; *Scale out* becomes two or more app replicas serving.
  - **API:** `GET /api/v1/sandbox/ruleset?version=` serves an older ruleset, so the dashboard opens an older game with its own catalog and defaults.
  - **Dashboard:** a Traffic palette entry, nodes titled by their population and coloured by how their requests fare, the contract problem on the node and its edge, a **Configure traffic** form, an *Inputs* breakdown and an inputs column in the app's inside view, and **Inside a traffic component** (clients → requests → connection → app).
  - **Tests:** Go tests for the empty start, segment volume, each contract mismatch, 404s, client timeouts and keep-alive, per-app mixes, retries, targeted events, sentiment, validation, replay, and v6 balance; an API test; a browser test.
- Phase 11: the application instance as a modelled backend service (ADR-0017), in ruleset `sandbox/v5`. New games use v5, and v1 to v4 replay bit for bit.
  - **Configuration:** `configure` on an application instance sets its configuration:
    - labels and a framework preset
    - sync or async processing, workers, max concurrency, backlog, max connections, timeout, TLS, and keep-alive
    - middleware from a catalog (each adds ms and CPU-ms; rate limiting rejects above its limit)
    - routes per endpoint with a `*` catch-all: base and CPU time, memory, request and response size, error rate, and dependencies (`cache`, `db-read`, `db-write`, `queue`, `storage`)

    Validation corrects nothing. CPU, memory, and network come from the priced size.
  - **Capacity emerges** as the smallest of the CPU, slot, connection, and network limits under the routes' costs, and it is reported as the bottleneck. Dependency waits use last tick's latency, so a slow database fills sync workers.
  - **Overload:** the backlog fills, waits grow, requests time out (exponential waits) and are rejected. Running out of memory crashes the instance, which restarts. Health is derived: starting, healthy, degraded, unhealthy, stopped.
  - **Reported:** the node's flow carries `app`, with runtime state and per-route RPS, outcomes, and latency.
  - **Routing:** `db-write` goes through a connected queue, and `cache` falls back to the database, so v4 designs keep working. From v5, routes, not endpoint flags, decide storage fetches.
  - **Balance:** the default instance serves about 56 RPS on small, against v4's 50. Overload now collapses as it does in production, so events cost designs with little headroom more. The v5 balance test pins this.
  - **Dashboard:** an app runtime panel, a **Configure app** form with a worker-memory note, and nodes coloured by health. Clicking an app instance opens an animated view inside it (connections → backlog → workers → middleware → routes → dependencies → response) with the engine's values and the bottleneck marked; **← System** or Esc returns.
  - **Tests:** Go tests for each part of the model, its validation, replay, v4 designs, and balance; an API test; and a browser test.
- Phase 11 started: a configurable Internet (ADR-0016), in ruleset `sandbox/v4`. New games use v4, and v1 to v3 replay as before.
  - **Traffic configuration:** the new `configure` command sets the Internet's configuration:
    - the source: `market` (users, as before) or a `configured` load test with a pattern (constant, ramp, spike, burst, periodic, or daily schedule)
    - traffic groups, each with a share, an endpoint mix, a region mix, and retries
    - endpoints, each with a method, a path, and cacheable and storage flags

    Every problem is reported at once, and nothing is corrected.
  - **Load tests:** they earn nothing; users, satisfaction, popularity, and goals hold still; and goal streaks restart. Events judged after running into a load test do not count towards goals.
  - **Flow solver:** load is routed per request class (cacheable read, read, write). A CDN answers only cacheable reads (45% of them in v4), and the endpoint mix sets an application's read, write, and storage shares.
  - **Retries:** they add `load × (f + … + f^N)` attempts from last tick's failure rate `f`. A request fails only if every attempt fails.
  - **Breakdown:** the flow carries `traffic`: source, RPS, retry RPS, requests in flight (Little's law), and RPS by group, region, and endpoint. Meters carry `loadTest`, and events carry `loadTest`.
  - **Dashboard:** clicking the Internet opens an animated view inside it (regions → groups → endpoints → downstream); **← System** or Esc returns. The Internet inspector shows live traffic and its breakdown. A **Configure traffic** form lists the engine's validation problems. A *load test* badge appears in the meters and on the Internet node, and the goals strip notes that goals are paused.
  - **Tests:**
    - Go tests for every pattern, group and endpoint splits, CDN and application class routing, zero traffic, Little's law, retries (storms and recovery), load-test economics and goals, a validation table and its boundaries, JSON replay, and v4 balance
    - an API test for `configure`
    - unit tests for the form's conversions
    - a browser test that configures a load test
- Phase 10 completed: goals and unlocks, in ruleset `sandbox/v3`. New games use v3, and the API accepts `ruleset` to start an older version.
  - **Goals:** fifteen goals checked after every tick. A goal's conditions can bound a meter, a meter's change over a window, a statistic over a component kind, or a count of recovered events. A goal can also hold for several ticks or require an earlier goal. Reached goals are permanent and replay deterministically.
  - **Unlocks:** the load balancer unlocks after the first request; the cache, read replica, queue, worker, and API gateway at 10k users; the CDN at 100k users with health of 80 or more.
  - **Learning path:** `goal` evidence type. Fifteen exercises complete automatically when a game reaches their goal, recorded as `sandbox <game id>`. New Stage 7, Incidents, has three exercises.
  - **API:** the state carries `goals` with each condition's value.
  - **Dashboard:** a goals strip with progress bars, a notice when a goal is reached, locked kinds in the palette, and a note on auto-completed exercises on the Learning path page.
  - **CLI:** `forgelab serve -progress`, used by the browser tests so test games never touch your progress.
- Phase 10 Event Deck and incident responses, in ruleset `sandbox/v2`. New games use v2; `sandbox/v1` is unchanged, has no events, and its saves replay as before.
  - **Cards:** eleven seeded cards (`viral-surge`, `marketing-spike` announced two hours ahead, `seasonal-dip`, `instance-crash`, `zone-outage`, `db-slowdown`, `cache-stampede`, `queue-backlog`, `ddos`, `cost-spike`, `third-party-outage`) drawn from day two. Popularity raises the odds of surges and attacks, and complexity raises the odds of failures.
  - **Effects:** each card changes model inputs: traffic, attack traffic, replicas up, service time, hit ratio, capacity, cost, or a failure share.
  - **Outcomes:** each event is judged recovered or not one hour after it ends.
  - **Responses:** the `respond` command offers `restart`, `failover` (promote a read replica) and `rate-limit` / `lift-rate-limit` (an API gateway blocks 90% of attack traffic and 1% of users).
  - **Flow solver:** it tracks attack traffic separately from real traffic, and a down queue holds its backlog.
  - **API:** the state carries `events`, and nodes carry `downReplicas` and `rateLimited`.
  - **Dashboard:** an event strip, partial-outage and rate-limit badges on nodes, Respond actions in the inspector, and an Attack RPS tile.
  - **Tests:** a browser test that plays a seeded game through events and responses.
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