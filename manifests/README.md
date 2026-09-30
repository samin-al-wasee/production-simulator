# Manifests

Declared inputs for ForgeLab's simulators.

| Folder | Contains | Used by |
|---|---|---|
| `pipelines/` | Build → test → deploy pipelines (`apiVersion: forgelab/v1`, `kind: Pipeline`) with rolling, canary, and blue-green deploy stages | `forgelab pipeline run`, the dashboard's Pipelines page (`core/internal/pipeline`) |

Durations are virtual: a pipeline of many minutes plays out instantly, and nothing is built or deployed.
