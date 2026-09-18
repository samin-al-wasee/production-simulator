# AGENTS.md

Rules for **any AI agent (or collaborator)** working in the ForgeLab repository.

## 1. Project mission

ForgeLab is **not an application**. It is a reusable **Production Systems Laboratory**: a platform where any application can be plugged in and executed inside a production-like environment.

The repository exists for learning, experimentation, benchmarking, incident simulation, distributed-systems practice, SRE, platform engineering, DevOps, cloud infrastructure, and system design.

The prime directive is **modular composition, not a fixed architecture**. Every infrastructure layer is optional. Never assume a component is required.

**Status:** bootstrap phase. Documentation and conventions only — **do not implement infrastructure (Docker, Kubernetes, Terraform, application code) until a roadmap phase authorizes it.**

## 2. Decided stack

| Layer | Choice | Rules |
|---|---|---|
| Backend / simulation core | **Go** | Follow `https://github.com/golang-standards/project-layout`-style module layout; keep simulation core logic in pure Go, separable from CLI/API/UI layers |
| Dashboard | **Next.js** (TypeScript, App Router) | Thin consumer of the backend API; never duplicate simulation logic |
| Analytics / AI | **Python** (optional, later) | Separate service/module; never woven into the Go core |

Do not propose changes to this stack without an ADR.

## 3. Architectural principles

1. **Platform over application** — the platform is the product; applications are inputs.
2. **Composition over configuration** — users enable only the components they need.
3. **Production parity** — the lab must behave like real production.
4. **Observable by default** — every component ships metrics, logs, and traces.
5. **Failure is a feature** — incidents are study material, not defects to hide.
6. **Infrastructure as Code** — everything declared, versioned, reproducible.
7. **Cloud agnostic** — local-first; cloud presets only as opt-in layers.
8. **Documentation first** — architecture and decisions precede implementation.

## 4. Coding philosophy

- **Simulation correctness over presentation** — the core is deterministic and testable headlessly, independent of any UI.
- **Kept modules** — simulation logic, runtime orchestration, and dashboard stay separable.
- **Match existing patterns** — check neighboring files before writing new ones.
- **No comments unless they explain non-obvious intent.**
- **No secrets** — never log, store, or commit passwords, tokens, or keys.
- **Minimal and focused diffs** — unrelated cleanups go to separate changes.
- **Explicit before magical** — prefer plain, reviewable code over framework cleverness.

## 5. Documentation-first workflow

Documentation changes and code changes ship together. "Docs updated" is part of the definition of done:

| Change type | Document to touch |
|---|---|
| Architecture / boundary decision | `docs/architecture.md` + an ADR in `docs/decisions/` |
| New or changed component | `docs/component-catalog.md` (+ component README) |
| New or changed scenario | `docs/scenarios.md` (+ scenario folder) |
| Principle change | `docs/principles.md` |
| Structure / ownership change | `docs/repository-structure.md` |
| Phase movement | `ROADMAP.md` |
| Roadmap / experience | `README.md`, `docs/learning-path.md` |
| Agent rules | `AGENTS.md` |

**Source-of-truth priority when deciding what to build:**

```text
1. Explicit user instruction in the current session
2. docs/architecture.md         (how it is structured)
3. docs/principles.md           (guidance it must obey)
4. docs/component-catalog.md    (what components exist)
5. docs/decisions/              (agreed decisions)
6. ROADMAP.md                   (what is authorized now)
7. Existing repository layout   (how it has been done so far)
```

Conflicts between documents are never resolved silently — state both sides and ask.

## 6. ADR requirement

- Record any decision that materially affects architecture, component boundaries, data flow, observability, reliability, or long-term maintainability as an Architecture Decision Record in `docs/decisions/`.
- File naming: `NNNN-short-title.md` (zero-padded, e.g. `0001-guide-stack-selection.md`).
- Follow the template in [`docs/decisions/README.md`](docs/decisions/README.md) (Context / Decision / Consequences / Alternatives Considered).
- Minor implementation decisions do not require an ADR.
- A decision is not finalized until it is implemented and documented.

## 7. Naming conventions

- **Directories:** `kebab-case` (e.g. `component-catalog`, `application.schema.yaml` in `manifests/`).
- **Files:** `kebab-case` for `.md` and `.yaml`; `snake_case` for Go; `camelCase` for TypeScript. Match the language's community convention.
- **Components:** one directory per component under `components/<domain>/<component>/`, with a `README.md` describing purpose, provided services, dependencies, and status.
- **Scenarios:** one directory per scenario under `scenarios/<category>/<name>/`, with a README using the standard scenario template from `docs/scenarios.md`.
- **Applications:** one directory per plugged-in application under `applications/<name>/`, self-contained and clearly documented.
- Keep names consistent and professional — this repo will eventually be open source.

## 8. Folder ownership

| Folder | Owner responsibility |
|---|---|
| `docs/` | All architecture, principles, catalog, scenarios, decisions |
| `applications/` | Plugged-in sample/user applications (never core) |
| `manifests/` | Application manifest schema, validation, and examples |
| `components/` | One subdirectory per domain; each component keeps its own README |
| `environments/` | `local/`, `staging/`, `cloud/` presets |
| `scenarios/` | Reproducible drills grouped by category |
| `scripts/` | Development and validation helpers |
| `templates/` | Reusable starting points for apps, services, manifests |
| `.github/` | Issue/PR templates and CI workflows |

Cross-cutting changes (e.g. a new scenario that needs a new component) must touch the owning folders **and** their catalog/scenario docs together.

## 9. Adding new components

Before adding a component:

1. Check `docs/component-catalog.md` — the component may already be documented as planned.
2. A component may only be added to a roadmap phase that authorizes it (`ROADMAP.md`).
3. Create `components/<domain>/<component>/README.md` covering: purpose, what it provides, what it depends on, planned config, and status (`planned` / `implemented`).
4. Update `docs/component-catalog.md` (table row + status).
5. Any architectural impact requires an ADR (`docs/decisions/`).
6. No component is assumed; a user must be able to opt out of anything.

## 10. Rules for scenarios

1. A scenario belongs to one category folder under `scenarios/`.
2. Every scenario README follows the standard template — **Goal, Components involved, Expected symptoms, Investigation, Success criteria** (see `docs/scenarios.md`).
3. Scenarios are declared, reproducible, and safe: never run an irreversible/destructive drill on a shared environment without explicit approval.
4. Updating a scenario requires updating `docs/scenarios.md` in the same change.
5. Scenario names: `kebab-case`, describing the observed condition (`db-slowdown`, `kafka-consumer-lag`).

## 11. Workflow rules

- **Always read the docs before acting** — at minimum `README.md`, `ROADMAP.md`, and the folder READMEs that the change touches.
- **Plan before code** — for any non-trivial task, state the files to change, the docs to update, and the verification plan.
- **Verify after meaningful changes** — run the appropriate test and lint commands. See `.opencode/command/` (`test.md`, `lint.md`). Current expectations:
  - Go: `go test ./...`, `gofmt -l`, `go vet ./...` (from the Go module root, once it exists)
  - Next.js: the app's `lint` and `typecheck` scripts (once the app exists)
  - Report results honestly; a failing suite is never "probably fine."
- **Never run destructive commands without asking.**
- **Never commit, push, or open PRs unless the user explicitly asks.**
- **Never merge or branch on your own** — propose the branch + commit plan and get confirmation.

## 12. Commit message convention

- **One branch per feature**, created off an up-to-date `main`. Branch names: `<type>/<short-desc>` (e.g. `docs/bootstrap`, `feat/component-catalog`).
- **Small, meaningful commits** — each commit is one unit of work. Keep changes focused; split large changes.
- **Format:** `[<type>] Title` first line, optional body (what/why):

  ```
  [feat] Add application manifest schema

  - define application.schema.yaml
  - add validation notes to manifests/README.md
  - document in component-catalog
  ```

- Suggested `<type>` values: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `infra`.
- Suggested workflow: work on a feature branch, keep it short-lived, merge back fast-forward.

## 13. Guardrails (hard stops)

Stop and consult the user before:

1. Implementing infrastructure, Docker, Kubernetes, Terraform, or application code before a roadmap phase authorizes it.
2. Introducing a dependency or external tool without asking.
3. Changing the decided stack (Go / Next.js / optional Python).
4. Resolving a documentation conflict unilaterally.
5. Writing anything not reproducible via IaC into an environment (drift).
6. Running anything destructive, cloud-costing, or irreversible.