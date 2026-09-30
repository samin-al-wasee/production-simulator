# ADR-0002: Local Single-Node Runtime and Go Core Layout (Phase 1)

**Status:** superseded by [ADR-0014](0014-sandbox-only-platform.md) (was proposed; the code it describes is on the `archive/live-lab` branch)
**Date:** 2026-09-18

## Context

Phase 1 ("Local single-node") authorizes the first runnable ForgeLab experience: run a user-provided application in a production-like environment on one machine. Two decisions shape every later increment:

1. **Execution model** — how the platform deploys the minimal topology (reverse proxy + one application + one database) on a laptop. It must be reproducible (IaC), close to production behavior, and cheap to start/stop. This also determines what the `local` environment preset contains.
2. **Go core location** — ADR-0001 fixed the core language as Go and the repo as a composable monorepo that will later also hold a Next.js dashboard and an optional Python analytics service. The Go module must live somewhere that keeps those layers separable and follows the `golang-standards/project-layout` discipline.

## Decision

1. **The `local` environment preset is a single-node Docker Compose stack.** The minimal Phase 1 topology is: a front-facing reverse proxy, the user's application (one container), and one database (PostgreSQL). Everything is declared in Compose files under `environments/local/`; nothing is started manually. Specific proxy/database images are implementation details chosen during the runtime increment, not fixed here. Because every layer is optional, more components compose in later phases without restructuring the preset. The devcontainer provides Docker-in-Docker so these stacks can be created and destroyed without touching the host daemon (which also suits later destructive drills).

2. **The Go core lives in `core/` at the repository root**, as module `github.com/samin-al-wasee/production-simulator/core`, using a `golang-standards/project-layout`-style shape (`cmd/`, `internal/`, `pkg/`). The dashboard and analytics layers will live elsewhere (e.g. `web/` for Next.js) without ever sharing a module boundary with Go.

## Consequences

### Positive

- The local preset is declarative, reproducible, and mirrors real production (containers, proxy, DB) rather than emulating it.
- Docker-in-Docker in the devcontainer isolates lab experiments from the host.
- `core/` keeps the simulation/runtime logic pure Go and headless-testable, independent of the UI layers that arrive later.
- Compose files are a natural IaC baseline and can be reused as the starting point for `staging`/`cloud` presets.

### Negative / Trade-offs

- The local preset requires a working Docker daemon (via the devcontainer's Docker-in-Docker or Docker Desktop on the host), adding a runtime prerequisite beyond the Go toolchain.
- Docker-in-Docker runs a nested daemon; on Windows this is heavier than using the host daemon directly.
- `core/` as a separate module introduces an extra `go.mod`/`go.work` boundary to manage in the monorepo. `.github/` CI and `Makefile` targets must target the subdirectory.

## Alternatives Considered

- **Local processes only (Runtime Manager, no containers)** — cataloged as a future component, but Phase 1 wants the production-parity posture of real containers, and process orchestration is a different mechanics to build. Postponed.
- **Single root Go module** — rejected: would merge the simulation core's dependency surface with any accidentally-adjacent code and make the eventual Next.js/Python split messier.
- **Minikube / kind as the Phase 1 base** — closer to `cloud`, but far too heavy and operationally slow for the laptop "single-node" promise of Phase 1. Kubernetes remains Phase 4.
- **Podman-compose** — viable, but Docker Compose is the wider ecosystem default and Docker is already the documented runtime target.

## References

- `docs/architecture.md` (Compute and Data & Messaging layers, optionality rule)
- `docs/decisions/0001-apply-stack-foundations.md` (Go core, composable monorepo)
- `ROADMAP.md` Phase 1 (authorization for the local single-node runtime)
- `.devcontainer/` (development environment where local stacks run)