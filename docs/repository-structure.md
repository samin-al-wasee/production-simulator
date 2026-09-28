# ForgeLab Repository Structure

**Document status:** Baseline · v1.0

The layout is stable. New files must fit into an existing folder; structural changes require updating this document and, if architectural, an ADR.

## Top-level map

```text
ForgeLab/
├── README.md                Project overview and documentation map
├── AGENTS.md                Rules for AI agents and collaborators
├── CONTRIBUTING.md          Contribution guide
├── ROADMAP.md               Phased plan (authorization for work)
├── LICENSE                  Apache-2.0
├── CHANGELOG.md             Release history
├── .gitignore               Ignore rules
├── .editorconfig            Editor/formatting defaults
├── Makefile                 Development helper targets
├── .env.example             Environment template
│
├── docs/                    All architecture, principles, catalog, scenarios, decisions
├── core/                    Go simulation core, CLI, and manifest validation
├── dashboard/               Next.js dashboard (thin consumer of the core API)
├── synthetic-apps/          Planning sketch: workload engine and workload templates (not implemented)
├── applications/            Plugged-in sample/user applications (never core)
├── manifests/               Application manifest schema, validation, examples
├── components/              Optional production components, grouped by domain
├── environments/            local / staging / cloud presets
├── scenarios/               Reproducible drills grouped by category
├── scripts/                 Development and validation helpers
├── templates/               Reusable starting points (apps, services, manifests)
├── .devcontainer/           Development container (Go, Node, Docker-in-Docker)
└── .github/                 Issue/PR templates and CI workflows
```

## Folder ownership

| Folder | Contains | Owns |
|---|---|---|
| `docs/` | Vision, architecture, principles, catalog, scenarios, glossary, learning path, decisions | Every conceptual document |
| `core/` | `cmd/forgelab` CLI, `internal/manifest` validation, `internal/calibration` and `internal/budget` (Phase 0.5), later simulation packages | Simulation core and its tooling |
| `synthetic-apps/` | Planned workload-engine, application/service/endpoint generators, operation-engine, models, telemetry-generator, templates | Synthetic Applications / Workload Engine planning |
| `applications/` | One self-contained sample per plugged-in application: `applications/<name>/` | Application samples; never core code |
| `manifests/` | `application.schema.yaml`, validation tooling, curated examples | Manifest schema and validation rules |
| `components/` | One directory per domain: networking, compute, databases, messaging, storage, observability, security, infrastructure, cicd, reliability | Component implementations and their READMEs |
| `environments/` | `local/`, `staging/`, `cloud/` presets composing components | Environment definitions |
| `scenarios/` | Categories + one folder per scenario: traffic, failures, security, scaling, performance | Scenario definitions |
| `scripts/` | Helper scripts (validation, generators) | Scripts |
| `templates/` | `application/`, `service/`, `manifests/` starting points | Templates |
| `.devcontainer/` | `devcontainer.json`, base `Dockerfile`, container setup scripts | Development container definition |
| `.github/` | Issue templates, PR template, workflows | GitHub automation |

## Docs layout

| Document | Purpose |
|---|---|
| `docs/vision.md` | Why ForgeLab exists |
| `docs/architecture.md` | Conceptual, composable architecture |
| `docs/principles.md` | Design principles |
| `docs/development-loop.md` | Agent-agnostic requirement → plan → implement → review → test → document loop |
| `docs/repository-structure.md` | This document |
| `docs/component-catalog.md` | Catalog of planned components by domain |
| `docs/scenarios.md` | Scenario template + planned scenarios |
| `docs/learning-path.md` | Progressive learning roadmap |
| `docs/glossary.md` | Terminology reference |
| `docs/decisions/` | Architecture Decision Records (ADR) |

## Components layout

```text
components/<domain>/
└── <component>/
    └── README.md    # purpose, provided services, dependencies, config, status
```

## Scenarios layout

```text
scenarios/<category>/
└── <scenario-name>/
    └── README.md    # Goal, Components involved, Expected symptoms,
                     # Investigation, Success criteria (see docs/scenarios.md)
```

## Environments layout

```text
environments/
├── local/        # laptop runtimes (Docker Compose, processes)
├── staging/      # closer-to-prod with persistence and observability
└── cloud/        # managed clusters (Kubernetes + cloud presets)
```

## Simulation layout

Phase 0.5 — Simulation Foundation, in progress. The module map below is conceptual; the code lives in `core/internal/` (ADR-0003 amendment). `calibration` and `budget` are implemented:

```text
simulation/
├── capacity-engine/
├── resource-virtualization/
├── scaling-model/
├── traffic-model/
├── calibration/
└── profiles/
```

## Synthetic apps layout

Planned (Synthetic Applications / Workload Engine, not yet implemented):

```text
synthetic-apps/
├── workload-engine/
├── application-generator/
├── service-generator/
├── endpoint-generator/
├── operation-engine/
├── db-model/
├── cache-model/
├── messaging-model/
├── concurrency-model/
├── telemetry-generator/
└── templates/
    ├── generic/       # crud-api, read-heavy-api, write-heavy-api, ...
    ├── ecommerce/     # catalog, cart, checkout, inventory
    ├── travel/        # search, booking, notification
    ├── banking/       # account, transfer, ledger
    └── social/        # feed, chat, notification, media
```

## Cross-cutting rule

A change that spans ownership (e.g. a new scenario needing a new component) must touch the owning folders **and** their catalog/scenario docs together (see `AGENTS.md` §5, §8).