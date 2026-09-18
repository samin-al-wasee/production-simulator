# Manifests

Application manifests are ForgeLab's declaration of intent: they say **what** runs, **which** platform components it needs, and **how** it should behave in a given environment. The platform reads manifests to compose environments; nothing is hardcoded.

**Nothing is implemented yet.** This folder holds the schema and conventions only.

## `application.schema.yaml`

The canonical schema for an application manifest. Curated example manifests live alongside it as `<name>.example.yaml` and are validated against this schema by the core CLI:

```sh
# from the repository root
make validate FILE=manifests/hello.example.yaml
make validate FILE=manifests/shop-backend.example.yaml
```

## Example manifests

| File | Demonstrates |
|---|---|
| `hello.example.yaml` | Minimal single-container monolith (runtimes, ports) |
| `shop-backend.example.yaml` | Multi-component app (database + reverse proxy, scaling, env) |

## Manifest principles

* **Declarative** — describes the desired end state, not steps.
* **Composable** — lists exactly the components used; every component is optional.
* **Environment-aware** — the same base manifest can target `local`, `staging`, or `cloud`.
* **Versioned** — a manifest references concrete versions of app and components.
* **Validated** — a manifest that fails schema validation never reaches a runtime.

## Adding a manifest

1. Extend `application.schema.yaml` (schema changes require updating this README and any affected docs).
2. Add an example under `manifests/<name>.example.yaml`.
3. Reference it from the target environment (`environments/<env>/`).