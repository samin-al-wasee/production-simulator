# AGENTS.md

Rules for **any AI agent (or collaborator)** working in the ForgeLab repository.

## 1. Project mission

ForgeLab is the **Production Sandbox**: a deterministic, model-driven game in which a player builds a production system from an empty world and runs it under growth, events, incidents, and a budget ([ADR-0014](docs/decisions/0014-sandbox-only-platform.md)).

The repository exists for learning production systems: scaling, bottlenecks, caching, queues, reliability, the economics of capacity, and system design.

The prime directive is **modelled, never scripted**: every value the game shows is computed from declared capacities and the player's topology, and labelled as simulated. Nothing in ForgeLab starts a real container, process, or cloud resource.

**Status:** Phase 12 (Observability) is in progress, after Phase 11 (Deep component simulation) and Phase 10 (Production Sandbox); the Live-mode lab of phases 1 to 9 is retired and kept on the `archive/live-lab` branch (see `ROADMAP.md`). **Do not reintroduce real infrastructure (Docker, Kubernetes, Terraform, sample applications) without a new ADR, and build only what a roadmap phase authorizes.** The one container is the API's own deployment image (`backend/Dockerfile`, ADR-0026), not lab infrastructure.

## 2. Decided stack

| Layer | Choice | Rules |
|---|---|---|
| Backend / simulation core | **Go** | Follow `https://github.com/golang-standards/project-layout`-style module layout; keep simulation core logic in pure Go, separable from CLI/API/UI layers |
| Dashboard | **Next.js** (TypeScript, App Router) | Thin consumer of the backend API; never duplicate simulation logic |
| Analytics / AI | **Python** (optional, later) | Separate service/module; never woven into the Go core |

Do not propose changes to this stack without an ADR.

## 3. Architectural principles

The full text is in [`docs/principles.md`](docs/principles.md).

1. **Modelled, never scripted, always labelled** — values come from the model, never from curves or random numbers.
2. **Bottlenecks come from the design** — the same design under the same load has the same bottleneck.
3. **Formulas are explainable** — simple, documented, checkable by hand, pinned by tests.
4. **The player composes everything** — a new game is empty; nothing is implied.
5. **Failure is a feature** — saturation, incidents, and bankruptcy are study material.
6. **Reproducible by construction** — ruleset version + seed + command log replays a game exactly.
7. **Behavior over business functionality** — a generic economy; no domain logic.
8. **Headless core, thin dashboard** — all simulation logic in Go; the UI renders and sends commands.
9. **Documentation first** — architecture and decisions precede implementation.

## 4. Coding philosophy

- **Simulation correctness over presentation** — the core is deterministic and testable headlessly, independent of any UI.
- **Kept modules** — simulation engine, API, and dashboard stay separable.
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
| New or changed in-game scenario (event) | `docs/scenarios.md` |
| Change to the model's formulas or tuning | `docs/architecture.md` (+ a new ruleset version if saves would replay differently) |
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
- **Scenarios:** in-game events, documented in `docs/scenarios.md` with the standard template, named in `kebab-case` after the condition (`viral-surge`, `db-slowdown`).
- Keep names consistent and professional — this repo will eventually be open source.

## 8. Folder ownership

| Folder | Owner responsibility |
|---|---|
| `docs/` | All architecture, principles, catalog, scenarios, decisions |
| `backend/` | Go: Sandbox engine, pipeline simulator, learning tracker, secret scan, API, CLI (never UI/dashboard code) |
| `frontend/` | Next.js dashboard; a thin consumer of the backend API (never simulation logic) |
| `components/` | One subdirectory per domain; each component keeps its own README |
| `manifests/` | Declared pipelines for the pipeline simulator |
| `learning/` | Machine-readable learning path |
| `security/` | Secret-scan allowlist |
| `templates/` | Postmortem template |
| `.github/` | Issue and PR templates |

Cross-cutting changes (e.g. a new in-game component kind that needs engine, API, and dashboard work) must touch the owning folders **and** the catalog together.

## 9. Adding new components

Before adding a component:

1. Check `docs/component-catalog.md` — the component may already be documented as planned.
2. A component may only be added to a roadmap phase that authorizes it (`ROADMAP.md`).
3. Create `components/<domain>/<component>/README.md` covering: purpose, what it provides, what it depends on, planned config, and status (`planned` / `implemented`).
4. Update `docs/component-catalog.md` (table row + status).
5. Any architectural impact requires an ADR (`docs/decisions/`).
6. No component is assumed; in the game, the player places every one.

## 10. Rules for scenarios

1. A scenario is an in-game event drawn from the Event Deck and documented in `docs/scenarios.md` with the standard template — **Goal, Trigger, Effect on the model, Expected symptoms, Responses, Success criteria**.
2. A scenario changes model inputs (traffic, capacity, service time, hit ratio, cost), never the output meters directly.
3. Scenarios are seeded and recorded, so replaying a save replays them.
4. Adding or changing a scenario updates `docs/scenarios.md` in the same change.

## 11. Workflow rules

- **Follow the development loop** — requirement → plan → implement → review → test → document → repeat, as defined in [`docs/development-loop.md`](docs/development-loop.md). Invoke with `/dev-loop <requirement>` in OpenCode or Claude Code.
- **Always read the docs before acting** — at minimum `README.md`, `ROADMAP.md`, and the folder READMEs that the change touches.
- **Plan before code** — for any non-trivial task, state the files to change, the docs to update, and the verification plan.
- **Verify after meaningful changes** — run the appropriate test and lint commands. See `.opencode/command/` (`test.md`, `lint.md`). Current expectations:
  - Go: `go test ./...`, `gofmt -l`, `go vet ./...` (from `backend/`)
  - Next.js: the app's `lint`, `typecheck`, and `test` scripts; for UI changes also `make test-e2e` (Playwright), since unit tests and API checks do not exercise the browser
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

1. Building a feature before a roadmap phase authorizes it, or reintroducing real infrastructure (Docker, Kubernetes, Terraform, sample applications) without an ADR.
2. Introducing a dependency or external tool without asking.
3. Changing the decided stack (Go / Next.js / optional Python).
4. Resolving a documentation conflict unilaterally.
5. Changing a ruleset value in place instead of adding a ruleset version (saved games would replay differently).
6. Running anything destructive or irreversible.