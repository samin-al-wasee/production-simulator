# ADR-0015: Name the top-level code folders backend/ and frontend/

**Status:** accepted
**Date:** 2026-09-30
**Amends:** the repository layout carried forward from ADR-0002 (Go module in `core/`) and ADR-0010 (Next.js app in `dashboard/`)

## Context

ForgeLab's code lives in two top-level folders: the Go module in `core/` (engine, API, CLI) and the Next.js app in `dashboard/`. The names come from earlier phases, when "core" set the Go simulation apart from Docker, Kubernetes, and Terraform assets, and the dashboard was one surface among several. Since ADR-0014 the repository holds exactly one backend and one frontend. A newcomer looking for "the backend" or "the frontend" has to learn that they are called `core/` and `dashboard/`.

## Decision

1. The Go module moves from `core/` to **`backend/`**. Its module path becomes `github.com/samin-al-wasee/production-simulator/backend`. The package layout inside is unchanged: `cmd/forgelab` and `internal/{api,learning,pipeline,sandbox,secretscan}`.
2. The Next.js app moves from `dashboard/` to **`frontend/`**, with its contents unchanged. The product surface is still called *the dashboard* in prose, and the npm package keeps its name.
3. The Make targets follow the folders: `dashboard-dev` becomes `frontend-dev`, and `build-core` becomes `build-backend`. The other targets keep their names.
4. `forgelab` recognizes the repository root by `ROADMAP.md` and `backend/go.mod`.
5. Earlier ADRs and changelog entries keep the old paths as a historical record.

## Consequences

### Positive

- The two folders say what they are, and each keeps a single owner (AGENTS.md §8): simulation logic in `backend/`, UI in `frontend/`.
- No behavior changes. The API, the game rules, saves, and the learning path are untouched.

### Negative / Trade-offs

- Every current document, the Makefile, the Playwright config, the secret-scan allowlist, and Go imports change paths in one sweep.
- Local habits and scripts that call `make dashboard-dev` or `cd core` have to change. Existing clones need `npm install` only if `node_modules` was not moved with the folder.

## Alternatives Considered

- **`apps/backend` and `apps/frontend`:** the usual layout for multi-app monorepos. It was rejected because there are only two apps, so it adds a level of nesting everywhere for no gain.
- **Keep `core/` and `dashboard/` and only tidy their insides:** no churn, but it keeps the naming problem this ADR exists to fix.
