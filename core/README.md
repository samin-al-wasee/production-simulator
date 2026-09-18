# ForgeLab core (Go)

The pure-Go simulation/runtime core, separable from any CLI, API, or UI layer (ADR-0001, ADR-0002). The simulation logic here is deterministic and testable headlessly.

## Layout

```text
core/
├── cmd/forgelab/          CLI: headless interface to the core
└── internal/manifest/     Application-manifest validation against the JSON Schema
```

## Commands (from the repository root, inside the devcontainer)

```sh
cd core && go mod tidy
cd core && go test ./...
cd core && gofmt -l .
cd core && go vet ./...
make validate FILE=manifests/hello.example.yaml
```

Validation support is provided by `github.com/santhosh-tekuri/jsonschema/v6` (JSON Schema draft 2020-12) and `sigs.k8s.io/yaml` (YAML→JSON). The canonical schema stays at `manifests/application.schema.yaml`; the CLI reads it from that path by default (`forgelab validate -schema <path>` overrides it).