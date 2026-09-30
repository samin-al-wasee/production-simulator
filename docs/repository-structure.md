# ForgeLab Repository Structure

**Document status:** v2.1 (code folders renamed by [ADR-0015](decisions/0015-backend-and-frontend-folders.md))

New files must fit into an existing folder; structural changes require updating this document and, if architectural, an ADR.

## Top-level map

```text
ForgeLab/
├── README.md                Project overview and documentation map
├── AGENTS.md                Rules for AI agents and collaborators
├── CONTRIBUTING.md          Contribution guide
├── ROADMAP.md               Phased plan (authorization for work)
├── LICENSE                  Apache-2.0
├── CHANGELOG.md             Release history
├── Makefile                 Development helper targets
│
├── docs/                    Vision, architecture, principles, catalog, decisions
├── backend/                 Go: Sandbox engine, pipeline simulator, learning tracker, secret scan, API, CLI
├── frontend/                Next.js: Sandbox, Pipelines, Learning path (thin consumer of the backend API)
├── components/              One README per component, grouped by domain
├── manifests/               Declared pipelines for the pipeline simulator
├── learning/                Machine-readable learning path (path.yaml)
├── security/                Secret-scan allowlist
├── templates/               Postmortem template
├── .devcontainer/           Development container
└── .github/                 Issue and PR templates
```

## Folder ownership

| Folder | Contains | Owns |
|---|---|---|
| `docs/` | Vision, architecture, principles, catalog, scenarios, glossary, learning path, security, decisions | Every conceptual document |
| `backend/` | `cmd/forgelab` CLI; `internal/sandbox`, `internal/pipeline`, `internal/learning`, `internal/secretscan`, `internal/api` | Simulation logic and its tooling; never UI code |
| `frontend/` | Next.js app: Sandbox (home), Pipelines, Learning path; Playwright tests in `e2e/` | Dashboard UI; never simulation logic |
| `components/` | `sandbox/`, `cicd/`, `experience/`, `security/`, one README per component | Component documentation |
| `manifests/` | `pipelines/*.yaml` | Pipeline declarations |
| `learning/` | `path.yaml` | Learning path definition |
| `security/` | `secretscan.yaml` | Secret-scan allowlist |
| `templates/` | `postmortem/` | Templates |
| `.devcontainer/` | `devcontainer.json`, `Dockerfile`, setup scripts | Development container definition |
| `.github/` | Issue templates, PR template | GitHub automation |

Generated, git-ignored local state lives in `.forgelab/`: learning progress (`progress.json`) and saved Sandbox games (`sandbox/`).

## Docs layout

| Document | Purpose |
|---|---|
| `docs/vision.md` | Why ForgeLab exists and what the game teaches |
| `docs/architecture.md` | The engine, its model and formulas, the API, the dashboard |
| `docs/principles.md` | Design principles |
| `docs/component-catalog.md` | Components by domain, and the in-game component kinds |
| `docs/scenarios.md` | Planned in-game events and incidents |
| `docs/learning-path.md` | The seven-stage learning path |
| `docs/glossary.md` | Terminology reference |
| `docs/security.md` | Repository security practice |
| `docs/development-loop.md` | Requirement → plan → implement → review → test → document loop |
| `docs/repository-structure.md` | This document |
| `docs/decisions/` | Architecture Decision Records |

## Components layout

```text
components/<domain>/
├── README.md        # domain overview and component table
└── <component>/
    └── README.md    # purpose, provided services, dependencies, config, status
```

## Cross-cutting rule

A change that touches several folders (for example a new in-game component kind that needs engine, API, and dashboard work) updates each owning folder **and** the catalog in the same change.
