# CI/CD Components

Delivery automation: pipelines, deployment strategies, and artifacts.

**Status:** partially implemented — Pipeline Runner (Phase 7) and Deploy Controller (Phase 4) ship; Artifact Registry is planned.

## Components

| Component | Provides | Status |
|---|---|---|
| Pipeline Runner | Simulated build → test → deploy pipelines | implemented — see [pipeline-runner/](pipeline-runner/) |
| Deploy Controller | Rolling, canary, blue/green rollouts | implemented — see [deploy-controller/](deploy-controller/) |
| Artifact Registry | Versioned build artifacts | planned |

CI/CD is a component like any other — optional, declarative, and reproducible.