# ForgeLab

**ForgeLab** is a reusable **Production Systems Laboratory** — a platform where any application can be plugged in and executed inside a production-like environment.

ForgeLab is not an application. It is a playground for learning, experimentation, benchmarking, incident simulation, distributed-systems practice, SRE, platform engineering, DevOps, cloud infrastructure, and system design.

> **Bring your app → enable the components you need → break it on purpose → learn how production actually behaves.**

---

## Status

**Early bootstrap.** This repository currently contains the project foundation: documentation, architecture, conventions, and directory structure. No infrastructure has been implemented yet — the component catalog is planned, not built.

The currently implemented decisions are recorded in [`docs/decisions/`](docs/decisions/).

---

## Project Overview

ForgeLab lets a user provide their own application (Rails, Phoenix, Django, FastAPI, Spring Boot, Next.js, React, and more) and enable only the production infrastructure components they want around it.

Supported application shapes include:

* Monolith
* Server-rendered application
* SPA + API
* Microservices
* Event-driven architecture
* Background workers
* Distributed systems

Supported production components (all **optional** and **composable**):

* Docker, Kubernetes
* Load Balancer, Reverse Proxy, API Gateway
* PostgreSQL, Redis, RabbitMQ, Kafka, Object Storage
* Prometheus, Grafana, Loki, Tempo, OpenTelemetry
* Terraform, AWS, GCP
* CI/CD, Security, Chaos Engineering, Load Testing

---

## Vision

```text
Your application                        Infrastructure (just what you need)
+----------------------+                +--------------------------------------+
| Rails / Phoenix /    |                | Networking   Load Balancer, Proxy      |
| Django / FastAPI /   |    plugs       | Compute      Docker, Kubernetes, HPA   |
| Spring / Next.js /   |  ───────────►  | Data         Postgres, Redis, Kafka    |
| React / ...          |    into        | Messaging    RabbitMQ, Queues, DLQs    |
|                      |                | Observability  Prometheus, Grafana    |
+----------------------+                | Reliability  Chaos, Retries, DR       |
                                        +--------------------------------------+
                       ForgeLab Platform Runtime orchestrates it all
```

Every layer is optional. A simple monolith can run with nothing but Docker and a database; a distributed demo can use every component at once.

---

## Features (planned)

* **Application manifests** — declare an application once, run it in any environment
* **Component catalog** — a library of composable production components per domain
* **Environment presets** — `local`, `staging`, `cloud` out of the box
* **Scenario library** — reproducible incidents, traffic, security, and failure drills
* **Observability by default** — metrics, logs, and traces for every component
* **Chaos & reliability tooling** — injection, retries, circuit breakers, DR drills
* **Dashboard** — a thin Next.js UI to launch, monitor, and inspect experiments
* **Benchmarks & load testing** — repeatable load profiles against any deployment

---

## Architecture Overview

ForgeLab is layered conceptually as follows. Every layer is optional and composable.

```mermaid
flowchart TD
    APP["Application (user-provided)"]
    RT["Platform Runtime (orchestration)"]
    NW["Networking (LB / Proxy / Gateway)"]
    CP["Compute (Docker / Kubernetes)"]
    DM["Data & Messaging (DB / Cache / Queues)"]
    OB["Observability (Metrics / Logs / Traces)"]
    RE["Reliability (Chaos / Retries / DR)"]

    APP --> RT
    RT --> NW
    NW --> CP
    CP --> DM
    DM --> OB
    OB --> RE
```

Each layer can be included or skipped per environment. See [`docs/architecture.md`](docs/architecture.md) for details.

**Decided stack:**

| Layer | Choice | Rationale |
|---|---|---|
| Backend / simulation core | **Go** | Strong infrastructure-tooling ecosystem, single static binaries, excellent concurrency |
| Dashboard | **Next.js** (TypeScript) | Thin API consumer; App Router; familiar web stack |
| Analytics / AI | **Python** *(later, optional)* | Separate service, never woven into the Go core |

---

## Repository Structure

```text
ForgeLab/
├── docs/            # Vision, architecture, principles, catalog, scenarios, ADRs
├── applications/    # User-provided sample applications (pluggable)
├── manifests/       # Application manifest schema + examples
├── components/      # Optional production components, grouped by domain
├── environments/    # local / staging / cloud presets
├── scenarios/       # Incident, traffic, and reliability drill definitions
├── scripts/         # Development and validation helpers
├── templates/       # Starting points for apps, services, manifests
└── .github/         # Issue/PR templates and CI workflows
```

See [`docs/repository-structure.md`](docs/repository-structure.md) for the full map and folder-ownership rules.

---

## Design Philosophy

* **Platform over application** — ForgeLab is the stage, not the playwright
* **Composition over configuration** — enable what you need, ignore the rest
* **Production parity** — the lab behaves like a real production environment
* **Observable by default** — every component ships with metrics, logs, and traces
* **Failure is a feature** — incidents are here to be studied, not avoided
* **Infrastructure as Code** — everything is declared, versioned, and reproducible
* **Cloud agnostic** — no vendor lock-in; local-first with cloud presets
* **Documentation first** — architecture and decisions precede implementation

See [`docs/principles.md`](docs/principles.md).

---

## Future Roadmap (summary)

1. **Foundation** — bootstrap docs, conventions, manifest schema *(current)*
2. **Local single-node** — Docker Compose runtimes, first components
3. **Observability** — metrics/logs/traces wired into every component
4. **Messaging & queues** — RabbitMQ/Kafka, DLQs, async patterns
5. **Kubernetes** — clusters, scaling, rollout strategies
6. **Reliability & chaos** — fault injection, outage drills, DR
7. **Cloud presets** — Terraform + AWS/GCP
8. **CI/CD & dashboard** — pipeline simulation, lab UI
9. **Security drills** — attack simulation, secrets, compliance
10. **Production Simulator** — the full experience, end to end

Full detail in [`ROADMAP.md`](ROADMAP.md) and [`docs/learning-path.md`](docs/learning-path.md).

---

## Documentation Map

| Document | Purpose |
|---|---|
| [AGENTS.md](AGENTS.md) | Rules governing AI agents and collaborators |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute to ForgeLab |
| [ROADMAP.md](ROADMAP.md) | Phased implementation plan |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [docs/vision.md](docs/vision.md) | Why ForgeLab exists |
| [docs/architecture.md](docs/architecture.md) | Conceptual architecture |
| [docs/principles.md](docs/principles.md) | Design principles |
| [docs/repository-structure.md](docs/repository-structure.md) | Directory map and ownership |
| [docs/component-catalog.md](docs/component-catalog.md) | Catalog of planned components |
| [docs/scenarios.md](docs/scenarios.md) | Planned lab scenarios |
| [docs/learning-path.md](docs/learning-path.md) | Progressive learning roadmap |
| [docs/glossary.md](docs/glossary.md) | Terminology reference |
| [docs/decisions/](docs/decisions/) | Architecture Decision Records |

When documents conflict, stop and identify the conflict instead of silently choosing one.

---

## Getting Started

> This section is a placeholder. No infrastructure exists yet.

1. Clone the repository.
2. **Recommended:** open the repo in VS Code / Cursor with the Dev Containers extension and reopen in the container (`.devcontainer/`), which provides Go, Node.js, and Docker-in-Docker. Developing outside the container works too, but the toolchain versions you need (Go, Node.js, Docker, Make) must be installed manually.
3. Read [`docs/vision.md`](docs/vision.md) and [`docs/architecture.md`](docs/architecture.md).
4. Read [`CONTRIBUTING.md`](CONTRIBUTING.md) before contributing.

Tooling is expected to include: Go, Node.js (Next.js), Docker, and optionally Python. Exact prerequisites and commands will be documented here once the first runnable increment lands.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).