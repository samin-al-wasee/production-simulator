# ForgeLab core (Go)

The pure-Go simulation/runtime core, separable from any CLI, API, or UI layer (ADR-0001, ADR-0002). The simulation logic here is deterministic and testable headlessly.

## Layout

```text
core/
├── cmd/forgelab/          CLI: headless interface to the core
├── internal/manifest/     Application-manifest validation against the JSON Schema
├── internal/calibration/  Host detection (CPU, RAM, disk; cgroup v2 limits) — Phase 0.5
├── internal/budget/       Physical resource budget: host minus reserve — Phase 0.5
├── internal/virtualcluster/ Virtual cluster spec, node profiles — Phase 0.5
├── internal/scale/        Scale factor between physical budget and virtual capacity — Phase 0.5
├── internal/capacity/     Virtual capacity, utilization, saturation, exhaustion — Phase 0.5
├── internal/metrics/      Dual Metrics Mode translation — Phase 0.5
├── internal/retry/        Retry backoff and dead-letter semantics — Phase 3
├── internal/chaos/        Declared fault-injection experiments — Phase 5
├── internal/loadtest/     Open-loop HTTP load generator — Phase 5
└── internal/costguard/    Terraform plan cost guard — Phase 6
```

## Commands (from the repository root, inside the devcontainer)

```sh
cd core && go mod tidy
cd core && go test ./...
cd core && gofmt -l .
cd core && go vet ./...
make validate FILE=manifests/hello.example.yaml
cd core && go run ./cmd/forgelab host [-reserve 0.25] [-json]
cd core && go run ./cmd/forgelab retry -max-attempts 4 -failures 10
cd core && go run ./cmd/forgelab loadtest -url http://localhost:8080/api/work -rps 50 -duration 10s
cd core && go run ./cmd/forgelab chaos plan ../scenarios/failures/db-outage/experiment.yaml
cd core && go run ./cmd/forgelab costguard check -rules ../environments/cloud/cost-guard.yaml <plan.json>
cd core && go run ./cmd/forgelab cluster [-rps N] [-json] ../manifests/cluster.example.yaml
```

Validation support is provided by `github.com/santhosh-tekuri/jsonschema/v6` (JSON Schema draft 2020-12) and `sigs.k8s.io/yaml` (YAML→JSON). The canonical schema stays at `manifests/application.schema.yaml`; the CLI reads it from that path by default (`forgelab validate -schema <path>` overrides it).