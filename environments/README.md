# Environments

Reproducible presets that compose components for a target context. A manifest plus an environment preset defines a fully runnable lab. **Nothing is implemented yet.**

| Environment | Purpose |
|---|---|
| `local/` | Laptop runtimes: processes, Docker Compose; fastest feedback |
| `staging/` | Persistence + full observability; closer to production |
| `cloud/` | Managed clusters: Kubernetes + cloud presets (AWS/GCP) |

## Rules

* Everything in an environment is declared, versioned, and reproducible — no drift.
* Local-first: `cloud` presets are opt-in layers, never the baseline.
* Environments reference components from `components/` and manifests from `manifests/`; they do not re-implement them.