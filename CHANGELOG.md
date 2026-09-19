# Changelog

All notable changes to ForgeLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Repository bootstrap: directory structure, conventions, and documentation.
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

- `ROADMAP.md` — Phase 1 (Local single-node) marked in progress.
- `docs/architecture.md`, `docs/repository-structure.md`, `AGENTS.md` — document the local single-node envelope and `core/` placement.