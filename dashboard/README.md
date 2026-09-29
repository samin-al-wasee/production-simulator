# ForgeLab dashboard

Next.js (App Router, TypeScript) dashboard: launch, monitor, and inspect experiments. It is a thin consumer of the `forgelab` core API and never reimplements simulation logic (ADR-0010).

**Status:** implemented (Phase 7).

## Pages

| Page | Shows |
|---|---|
| Sandbox | The Production Sandbox game (Phase 10, ADR-0013): build palette, React Flow topology canvas, meters with sparklines, component inspector, speed controls, save. Every value is labelled simulated and comes from `core/internal/sandbox` |
| Overview | Dual Metrics Mode: physical host measurements next to simulated virtual values, the active scale factor, and the local stack's containers |
| Experiments | Declared chaos experiments with hypothesis, a Run button, live log, and per-probe results |
| Pipelines | Simulated build → test → deploy runs with seed, warm cache, bad release, and forced-failure options and a stage timeline |

The scale factor badge is in the header of every page. Virtual values are labelled and visually distinct and are never presented as hardware measurements.

## Run

```sh
make serve            # core API on 127.0.0.1:8090 (add RUNS=1 to allow starting experiments)
make dashboard-dev    # npm install + dev server on http://localhost:3001
```

The Sandbox is at <http://localhost:3001/sandbox>. The game state streams over Server-Sent Events through the `/api/forgelab/*` rewrite; if the stream cannot be held open the page falls back to polling. The current game id is remembered in the browser so a reload resumes it (games live in the `forgelab serve` process and end when it stops; use Save to keep one).

Experiment runs are disabled by default; without `RUNS=1` the Run buttons are inactive and say why. The API refuses `-enable-runs` on non-loopback addresses.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `FORGELAB_API_URL` | `http://127.0.0.1:8090` | Core API base URL (used by server rendering and the `/api/forgelab/*` rewrite) |

## Checks

```sh
make lint-fe    # eslint + tsc --noEmit
make test-fe    # vitest (formatting, timeline, and Sandbox helpers)
make test-e2e   # Playwright: plays the Sandbox in Chromium and fails on any browser error
cd dashboard && npm run build
```

`make test-e2e` starts the core API and the dev server if they are not already running. The first run downloads Chromium; on a fresh Linux container Chromium may also need system libraries (`npx playwright install-deps chromium`, needs sudo).

**Hot reload on a Windows-mounted checkout.** When the repository lives on a Windows drive shared into the dev container (a `9p`/`drvfs` mount under `/workspaces`), file-change events do not reach `next dev`, so edits are not picked up until the dev server is restarted. Clone the repository inside the WSL filesystem instead, or restart `make dashboard-dev` after each change.

## Layout

```text
dashboard/
├── app/          pages (Sandbox, Overview, Experiments, Pipelines, Learning path) and layout
├── components/   DualMetrics, ScaleBadge, ExperimentCard, PipelineSimulator, Sandbox*
└── lib/          API types and clients, formatting, timeline, and Sandbox display helpers (tested)
```
