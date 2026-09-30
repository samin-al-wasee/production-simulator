# ADR-0014: ForgeLab is the Production Sandbox; the Live-mode lab is retired

**Status:** accepted
**Date:** 2026-09-30
**Supersedes:** ADR-0002, ADR-0003, ADR-0004, ADR-0005, ADR-0006, ADR-0007, ADR-0008, ADR-0009, ADR-0011, ADR-0012; the Live-mode parts of ADR-0010 and ADR-0013

## Context

ADR-0013 introduced the Production Sandbox, a model-driven game in which a player builds a production system from an empty world, as a second mode beside the existing **Live mode**. Live mode was everything Phases 1 to 9 built: a Docker Compose lab with a sample application, observability and messaging overlays, a Kubernetes environment, AWS and GCP Terraform presets, chaos and load tooling, benchmarks, hardening checks, and the host-calibrated Resource Virtualization Engine.

With the Sandbox playable, the project owner decided that ForgeLab is the game. Keeping Live mode means maintaining two products: the Docker, Kubernetes, and Terraform stacks; a large set of scenario drills; and documentation that explains both. Live mode also needs Docker, free ports, and minutes of start-up, none of which the Sandbox needs.

ADR-0013 had rejected "replace the lab with the game". This ADR reverses that alternative on the owner's explicit instruction.

## Decision

1. **ForgeLab is the Production Sandbox.** The product is a deterministic, model-driven production-system game: the Go engine (`core/internal/sandbox`), its API, and the dashboard canvas. There is no Live mode.
2. **Removed:** the sample application (`applications/`); every environment (`environments/`: Compose, Kubernetes, Terraform, cost guard); the scenario drills (`scenarios/`); the helper scripts (`scripts/`); the application and cluster manifests and their templates; the real-infrastructure component READMEs; the core packages for host calibration, budgeting, virtual clusters, scale factors, capacity, dual metrics, chaos, load testing, benchmark reports, retry semantics, cost guard, compliance checks, and manifest validation, with their CLI commands and API endpoints; and the dashboard's Overview and Experiments pages.
3. **Kept:**
   * the **Sandbox** (engine, API, canvas, browser tests);
   * the **pipeline simulator** (`core/internal/pipeline`, `manifests/pipelines/`, the Pipelines page), which is already virtual-clock only;
   * the **learning path**, rewritten as Sandbox and pipeline missions, with manual completion only (its automatic evidence came from the removed chaos runs and benchmarks);
   * the **secret scan** and repository hygiene (`core/internal/secretscan`, `security/`, `.github/`);
   * the **postmortem template**.
4. **Preserved, not lost.** The complete Live-mode lab is kept on the local branch `archive/live-lab` (commit `4ef9118`) and in git history.
5. **Principles.** Principles about real infrastructure (production parity on real deployments, Infrastructure as Code environments, cloud presets, physical vs virtual resources, dual metrics, Live-mode scope) are removed or restated for a model. Principle 15 ("modelled, never scripted, always labelled") becomes the governing rule for every value ForgeLab shows.
6. **Dashboard.** The Sandbox is the home page (`/` redirects to `/sandbox`); the navigation is Sandbox, Pipelines, and Learning path.
7. **Roadmap.** Phases 0 to 9 stay in `ROADMAP.md` as history, marked as retired. Phase 10 continues: the event deck and incident responses, then goals and unlocks.

## Consequences

### Positive

* One product, one runtime: `make serve` plus `make dashboard-dev` is the whole experience, with no Docker, cloud accounts, or free ports beyond two.
* A much smaller codebase and documentation set to maintain.
* All effort goes into the game: events, incidents, missions, and balance.

### Negative / Trade-offs

* **No ground truth.** The Sandbox's formulas can no longer be checked against a real running stack. The model must stay simple, documented, and labelled as simulated; formula changes need tests that pin the behavior they claim.
* **Lost capability.** Running a real application, real chaos drills, real observability, and cloud presets is no longer possible from `main`; it requires checking out `archive/live-lab`.
* **Learning path evidence is manual.** Automatic completion returns only when Sandbox missions can verify goals (the goals-and-unlocks milestone).
* Historical ADRs describe code that no longer exists on `main`; they are kept for the record and marked superseded.

## Alternatives Considered

* **Keep both modes (ADR-0013 as written)** — rejected by the project owner: two products to maintain for one audience.
* **Delete without an archive** — rejected: the lab is substantial work; a local archive branch costs nothing.
* **Keep the Resource Virtualization Engine for the Sandbox** — rejected: it models the host's hardware, which the Sandbox does not use; the Sandbox's capacities come from its ruleset.
* **Drop the pipeline simulator and learning path too** — rejected by the project owner: both are simulated or study aids and fit the game.

## References

* `docs/decisions/0013-production-sandbox-game.md`
* `docs/architecture.md`, `docs/principles.md`, `docs/vision.md`
* `ROADMAP.md` Phase 10
