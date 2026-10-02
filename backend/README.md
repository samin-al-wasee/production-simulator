# ForgeLab backend (Go)

The Go backend: the Production Sandbox engine and the tools around it, separable from any CLI, API, or UI layer (ADR-0001, ADR-0014). Simulation logic here is deterministic and testable headlessly.

## Layout

```text
backend/
├── cmd/forgelab/          CLI: serve, pipeline, learn, security
├── internal/sandbox/      Production Sandbox engine: ruleset, flow solver, economy, save/replay — Phase 10
├── internal/api/          HTTP API for the dashboard (Sandbox games and SSE, pipelines, learning) — Phase 7, 10
├── internal/pipeline/     CI/CD pipeline simulation on a virtual clock — Phase 7
├── internal/learning/     Learning path and progress — Phase 9
└── internal/secretscan/   Committed-secret detection — Phase 8
```

## Commands (from the repository root)

```sh
cd backend && go test ./...
cd backend && gofmt -l .
cd backend && go vet ./...
cd backend && go run ./cmd/forgelab serve -repo ..                                   # API for the dashboard
cd backend && go run ./cmd/forgelab pipeline run ../manifests/pipelines/web-release-canary.yaml
cd backend && go run ./cmd/forgelab learn status
cd backend && go run ./cmd/forgelab security scan-secrets ..
```

YAML files (pipelines, learning path, secret-scan allowlist) are parsed with `sigs.k8s.io/yaml`.

## Deploying

`Dockerfile` builds the API into a small distroless image; build it from the repository root, which holds `learning/` and `manifests/` ([ADR-0026](../docs/decisions/0026-container-deployment.md)):

```sh
docker build -f backend/Dockerfile -t forgelab-api .
docker run -p 8090:8090 forgelab-api            # or -e PORT=10000 -p 10000:10000
```

`serve` listens on `:$PORT` when `PORT` is set, takes `FORGELAB_ALLOW_ORIGIN` for CORS, and stops cleanly on SIGTERM. On Render, `render.yaml` at the repository root defines the service (Docker, context `.`, health check `/healthz`). Saves go to `/app/.forgelab`, which needs a persistent disk to survive a redeploy.
