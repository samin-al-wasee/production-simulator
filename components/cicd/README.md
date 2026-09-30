# CI/CD Components

Simulated delivery: pipelines and deployment strategies on a virtual clock.

**Status:** implemented (Phase 7, ADR-0010).

## Components

| Component | Provides | Status |
|---|---|---|
| Pipeline Runner | Simulated build → test → deploy pipelines with rolling, canary, and blue-green deploys | implemented — see [pipeline-runner/](pipeline-runner/) |

Nothing is built or deployed: every run is a model played out on a virtual clock.
