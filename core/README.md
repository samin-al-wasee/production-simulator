# ForgeLab core (Go)

The pure-Go core: the Production Sandbox engine and the tools around it, separable from any CLI, API, or UI layer (ADR-0001, ADR-0014). Simulation logic here is deterministic and testable headlessly.

## Layout

```text
core/
├── cmd/forgelab/          CLI: serve, pipeline, learn, security
├── internal/sandbox/      Production Sandbox engine: ruleset, flow solver, economy, save/replay — Phase 10
├── internal/api/          HTTP API for the dashboard (Sandbox games and SSE, pipelines, learning) — Phase 7, 10
├── internal/pipeline/     CI/CD pipeline simulation on a virtual clock — Phase 7
├── internal/learning/     Learning path and progress — Phase 9
└── internal/secretscan/   Committed-secret detection — Phase 8
```

## Commands (from the repository root)

```sh
cd core && go test ./...
cd core && gofmt -l .
cd core && go vet ./...
cd core && go run ./cmd/forgelab serve -repo ..                                   # API for the dashboard
cd core && go run ./cmd/forgelab pipeline run ../manifests/pipelines/web-release-canary.yaml
cd core && go run ./cmd/forgelab learn status
cd core && go run ./cmd/forgelab security scan-secrets ..
```

YAML files (pipelines, learning path, secret-scan allowlist) are parsed with `sigs.k8s.io/yaml`.
