# Compute Components

Where applications run: local processes, containers, orchestration, and scaling.

**Status:** partially implemented — Kubernetes and the HPA ship in Phase 4; Docker is used by the local preset (Compose); Runtime Manager is planned.

## Components

| Component | Provides | Status |
|---|---|---|
| Docker | Container runtime, images, networks, volumes | planned |
| Kubernetes | Orchestration, scheduling, scaling, rollouts | implemented — see [kubernetes/](kubernetes/) |
| Runtime Manager | Local process runner | planned |
| Autoscaler (HPA) | Horizontal scaling on metrics | implemented — see [autoscaler-hpa/](autoscaler-hpa/) |

Any component here is optional; a manifest may run on a single process with no compute layer at all.