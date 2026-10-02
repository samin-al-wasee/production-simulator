# ForgeLab

**ForgeLab** is the **Production Sandbox**: a city-builder for software production. You start with an empty world (a market of users and a little money), place every component yourself, even the traffic, wire them together, and keep the system healthy and profitable as users arrive, traffic swings, and things break.

> **Start from nothing → build a production system → watch it grow and buckle → learn why architectures look the way they do.**

Everything is a deterministic model computed in a Go core. Nothing runs on your machine, so a system with hundreds of instances and millions of users plays instantly on a laptop, and every value is labelled as simulated.

---

## Status

**Phase 10 (Production Sandbox) is complete.** The engine, API, dashboard canvas, event deck (surges, outages, attacks, with restart, failover, and rate limiting), and goals with unlocks and automatic learning-path completion are playable. **Phases 11 (Deep component simulation) and 12 (Observability) are complete.** Every component has a detailed configuration and model: traffic, applications calling each other over configured connections, databases, caches, object storage, queues and workers, event streams, load balancers, gateways, and CDNs. From ruleset v14 nothing technical is visible until you place telemetry backends and instrument components; then **Observe** explains failures, charts metrics, shows traces and logs, and runs your alerts and SLOs. Traffic is a component: place one per client population (web, mobile, API, bot; one region each), connect it to an app whose protocol, port, TLS, and routes match, or watch its requests be refused, and run load tests with a spike, ramp, burst, or daily schedule. Application instances are modelled backend services: select one to see its bottleneck, queue, timeouts, and health, and configure its workers, middleware, and routes. See [`ROADMAP.md`](ROADMAP.md).

ForgeLab used to be a real-infrastructure lab (Docker Compose, Kubernetes, Terraform, chaos drills). That **Live mode** was retired in favour of the game ([ADR-0014](docs/decisions/0014-sandbox-only-platform.md)); it is kept on the `archive/live-lab` branch.

---

## Getting Started

You need Go and Node.js (the dev container in `.devcontainer/` has both).

```sh
make serve           # terminal 1: core API on 127.0.0.1:8090
make frontend-dev   # terminal 2: dashboard on http://localhost:3001
# or both in one terminal: make dev
```

Open <http://localhost:3001> and click **New game**.

1. Place **Traffic**, an **Application instance**, a **Database primary**, and **Object storage** from the palette (click, or drag onto the canvas).
2. Wire them by dragging between handles: **traffic → app → database**, and **app → storage**. Each traffic component brings its client type and region's share of your users; add more for the populations you want to reach.
3. Press **1×**. Users start paying. Watch the meters: cash, profit, users, RPS, p95 latency, errors, health, satisfaction, popularity, complexity.
4. As users grow, the app saturates (its bar turns red). Add replicas, a cache, read replicas, a queue with workers: whatever the bottleneck calls for, and whatever you can afford.

Games survive a page reload; **Save** writes a replayable save to `.forgelab/sandbox/`.

| What | Command |
|---|---|
| Follow the learning path | [`docs/learning-path.md`](docs/learning-path.md), `make learn` |
| Simulate a deploy pipeline | Dashboard **Pipelines** page, or `forgelab pipeline run manifests/pipelines/web-release-canary.yaml` |
| Lint, unit tests, secret scan | `make check` |
| Browser tests | `make test-e2e` |
| Deploy the API (Render or any container host) | `backend/Dockerfile` built from the repository root; `render.yaml` is a Render blueprint ([ADR-0026](docs/decisions/0026-container-deployment.md)) |
| Deploy the dashboard (Netlify) | `netlify.toml` builds `frontend/`; set `FORGELAB_API_URL` to the API's URL in the site environment (read at build time) |
| Every target | `make help` |

---

## How it works

| Part | Where |
|---|---|
| Sandbox engine: world, ruleset, flow solver, economy, save/replay | `backend/internal/sandbox` |
| API: games, commands, SSE stream | `backend/internal/api` (`forgelab serve`) |
| Dashboard: palette, React Flow canvas, meters, inspector | `frontend/` |
| Pipeline simulator | `backend/internal/pipeline`, `manifests/pipelines/` |
| Learning path | `learning/path.yaml`, `backend/internal/learning` |

The model's formulas (routing, utilization, latency, saturation, the economy) are documented in [`docs/architecture.md`](docs/architecture.md).

---

## Documentation Map

| Document | Purpose |
|---|---|
| [AGENTS.md](AGENTS.md) | Rules governing AI agents and collaborators |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [ROADMAP.md](ROADMAP.md) | What is authorized and what is next |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [docs/vision.md](docs/vision.md) | Why ForgeLab exists and what it teaches |
| [docs/architecture.md](docs/architecture.md) | The engine, its model, the API, the dashboard |
| [docs/principles.md](docs/principles.md) | Design principles |
| [docs/component-catalog.md](docs/component-catalog.md) | Components and in-game component kinds |
| [docs/scenarios.md](docs/scenarios.md) | In-game events and incidents, and how to respond |
| [docs/learning-path.md](docs/learning-path.md) | The seven-stage learning path |
| [docs/glossary.md](docs/glossary.md) | Terminology |
| [docs/security.md](docs/security.md) | Repository security practice |
| [docs/development-loop.md](docs/development-loop.md) | Requirement → plan → implement → review → test → document |
| [docs/repository-structure.md](docs/repository-structure.md) | Directory map and ownership |
| [frontend/README.md](frontend/README.md) | Dashboard: run, pages, tests |
| [docs/decisions/](docs/decisions/) | Architecture Decision Records |

When documents conflict, stop and identify the conflict instead of silently choosing one.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
