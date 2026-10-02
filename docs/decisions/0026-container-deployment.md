# ADR-0026: Deploying the API as a container

**Status:** accepted
**Date:** 2026-10-02

## Context

[ADR-0014](0014-sandbox-only-platform.md) retired every piece of real infrastructure (Docker Compose labs, Kubernetes, Terraform, sample applications), and AGENTS.md forbids reintroducing it without an ADR. The project owner asked for a Dockerfile so the backend can be deployed on Render. This is not the retired lab: nothing in ForgeLab starts containers for the player, and the game stays a model. It is how the API itself is hosted.

## Decision

1. **`backend/Dockerfile`** builds `forgelab` (static, `CGO_ENABLED=0`) and runs `forgelab serve -repo /app` on a distroless, non-root image (about 16 MB). The build context is the repository root, because the API reads `learning/` and `manifests/`; `.dockerignore` sends only `backend/`, `learning/`, and `manifests/`.
2. **Configuration from the environment:** `serve` listens on `:$PORT` when `PORT` is set and `-addr` is not, and takes `FORGELAB_ALLOW_ORIGIN` as `-allow-origin`. Flags still win. It shuts down cleanly on SIGTERM as well as Ctrl-C.
3. **`render.yaml`** is a Render blueprint: a Docker web service with `dockerfilePath: ./backend/Dockerfile`, `dockerContext: .`, health check `/healthz`, and `FORGELAB_ALLOW_ORIGIN` to fill in.
4. **State:** saves and learning progress are written under `/app/.forgelab`. On a host without a persistent disk they are lost when the container is replaced; games in memory are lost on restart, as they are locally.
5. **Scope:** only the API is containerized. The dashboard still runs with `make frontend-dev`, or on any Next.js host pointed at the API.

## Consequences

### Positive

* The API deploys in one step on Render or any container host, small and without a shell.
* Local development is unchanged.

### Negative / Trade-offs

* Games live in memory: a single instance, no horizontal scaling, and a restart loses unsaved games.
* Tests run in `make check`, not in the image build, because they read files relative to the repository.

## Alternatives Considered

* **Render's native Go runtime** — it cannot include `learning/` and `manifests/` from outside `backend/` as simply as a root build context does.
* **An Alpine runtime image** — larger and with a shell the server does not need.

## References

* [ADR-0014](0014-sandbox-only-platform.md), `backend/Dockerfile`, `render.yaml`, `.dockerignore`
