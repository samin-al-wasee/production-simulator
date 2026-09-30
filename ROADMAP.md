# ForgeLab Roadmap

This document is the source of truth for **what is authorized and what is next**. A feature may only be implemented when the phase it belongs to is active.

Legend: ⬜ planned · 🔨 in progress · ✅ done · 🗄️ retired

ForgeLab is the **Production Sandbox** ([ADR-0014](docs/decisions/0014-sandbox-only-platform.md)). Phases 0 to 9 built a real-infrastructure lab (Live mode); most of it was retired and is kept on the `archive/live-lab` branch. What survived is noted below.

## Phase 10 — Production Sandbox *🔨 in progress*

> A city-builder for software production: the player starts from an empty world, places and wires every component in the dashboard, and runs the system as a business under growth, events, and incidents. Fully model-driven; deterministic Go core; the dashboard renders and sends commands. ADR-0013, ADR-0014.

- [x] Documentation and ADR-0013 (architecture, principles, catalog, vision)
- [x] Sandbox engine: world, component catalog, traffic model, flow solver, economy, meters, deterministic ticks (`core/internal/sandbox`)
- [x] Sandbox API: games, commands, SSE tick stream, save/load (`/api/v1/sandbox/`)
- [x] Dashboard Sandbox screen: build palette, React Flow canvas, HUD, inspector, speed controls, browser tests
- [x] Retire Live mode; the Sandbox becomes the whole product (ADR-0014)
- [x] Event deck and incident response actions (surges, outages, DDoS, failover, rate limiting), with economy rebalancing (ruleset `sandbox/v2`)
- [ ] Goals and unlocks (missions tied to the learning path, with automatic completion)

## History: phases 0 to 9

| Phase | What it built | Now |
|---|---|---|
| 0 — Foundation | Repository, docs, conventions, ADR process | ✅ kept (docs re-scoped) |
| 0.5 — Simulation Foundation | Host calibration, resource virtualization, dual metrics | 🗄️ retired |
| 1 — Local single-node | Docker Compose lab, sample application, manifest validation | 🗄️ retired |
| 2 — Observability | Prometheus, Grafana, Loki, Tempo, OpenTelemetry | 🗄️ retired |
| 3 — Messaging & data | Redis, RabbitMQ, Kafka, retries, dead-letter queues | 🗄️ retired |
| 4 — Kubernetes | kind cluster, ingress, HPA, rollout scenarios | 🗄️ retired |
| 5 — Reliability & chaos | Chaos engine, outage drills, load generator, backup/restore | 🗄️ retired |
| 6 — Cloud presets | AWS and GCP Terraform presets, cost guard | 🗄️ retired |
| 7 — CI/CD & dashboard | Pipeline simulator; Next.js dashboard | ✅ pipeline simulator and dashboard kept; experiment pages retired |
| 8 — Security | Security overlay, hardening, compliance checks, secret scan | ✅ secret scan kept; the rest retired |
| 9 — Production Simulator | End-to-end run, benchmarks, learning path | ✅ learning path kept (rewritten as Sandbox missions); the rest retired |

---

## How phases are advanced

1. Documentation for the phase lands first (architecture + catalog updates).
2. Architectural changes get an ADR in `docs/decisions/`.
3. Implementation lands incrementally, small and reviewable.
4. Each phase ends with a `CHANGELOG.md` entry.
