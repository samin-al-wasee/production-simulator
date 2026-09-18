# Compute Components

Where applications run: local processes, containers, orchestration, and scaling.

**Status:** cataloged in `docs/component-catalog.md` — nothing implemented yet.

## Planned components

| Component | Provides | Status |
|---|---|---|
| Docker | Container runtime, images, networks, volumes | planned |
| Kubernetes | Orchestration, scheduling, scaling, rollouts | planned |
| Runtime Manager | Local process runner | planned |
| Autoscaler (HPA) | Horizontal scaling on metrics | planned |

Any component here is optional; a manifest may run on a single process with no compute layer at all.