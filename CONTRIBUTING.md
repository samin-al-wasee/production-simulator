# Contributing to ForgeLab

Thanks for contributing. ForgeLab is the **Production Sandbox**, a model-driven game for learning how production systems behave and grow. Before you open a PR, please read:

* [`README.md`](README.md) — project overview
* [`docs/vision.md`](docs/vision.md) — why ForgeLab exists
* [`docs/principles.md`](docs/principles.md) — design principles
* [`docs/architecture.md`](docs/architecture.md) — conceptual architecture
* [`AGENTS.md`](AGENTS.md) — working rules (including for AI agents)

## Code of conduct

Be respectful, be constructive, and assume good intent. This is a learning and experimentation project; gatekeeping and sneering at "simple" questions are both counterproductive.

## Getting started

1. Fork the repository and clone it.
2. Read the documentation before touching code.
3. Confirm the work you plan matches a phase in [`ROADMAP.md`](ROADMAP.md); if not, discuss it first (open an issue).

## Contribution types

| Type | Where |
|---|---|
| Bug report | GitHub issue (use the bug report template) |
| Feature / component request | GitHub issue (feature request template) |
| New component | Issue first, then an ADR if architectural, then a PR |
| New in-game scenario (event) | Issue + an entry following the template in `docs/scenarios.md` |
| Documentation | Direct PR, keeping the documentation map consistent |
| Code | PR against the phase that authorizes it on the roadmap |

## Documentation-first

- Documentation changes ship **in the same PR** as the behavior they describe (`AGENTS.md` §5).
- Any architectural impact requires an ADR under `docs/decisions/` (`AGENTS.md` §6).
- Keep the documentation map consistent with `README.md`.

## Working on a change

1. Branch off an up-to-date `main`: `<type>/<description>` (e.g. `docs/glossary`, `feat/component-catalog`).
2. Make small, focused commits (`AGENTS.md` §12).
3. Before opening the PR, run the relevant test and lint commands (see `.opencode/command/`):
   - `make check` (Go and dashboard lint, unit tests, secret scan)
   - `make test-e2e` for any dashboard change (plays the Sandbox in a browser)
4. Update `ROADMAP.md` / `CHANGELOG.md` if the change moves a phase or deserves a changelog entry.

## Pull requests

- One logical change per PR.
- Reference the issue(s) the PR resolves.
- Describe what changed, why, and how it was verified.
- Keep the PR small enough to review in one sitting. If it grows, split it.

## Scenarios and the model

- Scenarios are seeded, reproducible, and follow the standard template in `docs/scenarios.md` (Goal / Trigger / Effect on the model / Expected symptoms / Responses / Success criteria).
- Changes to the model's formulas or tuning update `docs/architecture.md`; a tuning change ships as a new ruleset version so saved games still replay.
- A scenario changes model inputs (traffic, capacity, cost), never the output meters directly (`docs/scenarios.md`).

## Licensing

By contributing you agree that your contributions are licensed under the [Apache License 2.0](LICENSE).