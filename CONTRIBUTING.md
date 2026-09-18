# Contributing to ForgeLab

Thanks for contributing. ForgeLab is a **Production Systems Laboratory** — a platform, not an app. Before you open a PR, please read:

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
| New scenario | Issue + scenario folder following the template in `docs/scenarios.md` |
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
   - Go: `go test ./...`, `gofmt -l`, `go vet ./...`
   - Next.js: `lint` and `typecheck` app scripts
4. Update `ROADMAP.md` / `CHANGELOG.md` if the change moves a phase or deserves a changelog entry.

## Pull requests

- One logical change per PR.
- Reference the issue(s) the PR resolves.
- Describe what changed, why, and how it was verified.
- Keep the PR small enough to review in one sitting. If it grows, split it.

## Scenarios and safety

- Scenarios must be **reproducible** and follow the standard template (Goal / Components involved / Expected symptoms / Investigation / Success criteria).
- Never add a scenario that runs destructive or irreversible actions on a shared environment without explicit approval.

## Licensing

By contributing you agree that your contributions are licensed under the [Apache License 2.0](LICENSE).