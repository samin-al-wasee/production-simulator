# ForgeLab dashboard

Next.js (App Router, TypeScript) dashboard: launch, monitor, and inspect experiments. It is a thin consumer of the `forgelab` core API and never reimplements simulation logic (ADR-0010).

**Status:** implemented (Phase 7).

## Pages

| Page | Shows |
|---|---|
| Overview | Dual Metrics Mode: physical host measurements next to simulated virtual values, the active scale factor, and the local stack's containers |
| Experiments | Declared chaos experiments with hypothesis, a Run button, live log, and per-probe results |
| Pipelines | Simulated build → test → deploy runs with seed, warm cache, bad release, and forced-failure options and a stage timeline |

The scale factor badge is in the header of every page. Virtual values are labelled and visually distinct and are never presented as hardware measurements.

## Run

```sh
make serve            # core API on 127.0.0.1:8090 (add RUNS=1 to allow starting experiments)
make dashboard-dev    # npm install + dev server on http://localhost:3001
```

Experiment runs are disabled by default; without `RUNS=1` the Run buttons are inactive and say why. The API refuses `-enable-runs` on non-loopback addresses.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `FORGELAB_API_URL` | `http://127.0.0.1:8090` | Core API base URL (used by server rendering and the `/api/forgelab/*` rewrite) |

## Checks

```sh
make lint-fe    # eslint + tsc --noEmit
make test-fe    # vitest (formatting and timeline helpers)
cd dashboard && npm run build
```

## Layout

```text
dashboard/
├── app/          pages (Overview, Experiments, Pipelines) and layout
├── components/   DualMetrics, ScaleBadge, ExperimentCard, PipelineSimulator
└── lib/          API types and clients, formatting and timeline helpers (tested)
```
