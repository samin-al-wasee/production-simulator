# ForgeLab dashboard

Next.js (App Router, TypeScript) dashboard where the Production Sandbox is played. It is a thin consumer of the `forgelab` core API and never reimplements simulation logic (ADR-0010, ADR-0014).

**Status:** implemented (Phase 7; Sandbox in Phase 10).

## Pages

| Page | Shows |
|---|---|
| Sandbox (home: `/` redirects here) | The Production Sandbox game (Phase 10, ADR-0013): build palette, React Flow topology canvas, meters with sparklines, component inspector, speed controls, save. Every value is labelled simulated and comes from `core/internal/sandbox` |
| Pipelines | Simulated build → test → deploy runs with seed, warm cache, bad release, and forced-failure options and a stage timeline |
| Learning path | The six-stage path of Sandbox and pipeline missions, with progress and a Mark done button |

Every value is simulated and labelled as such; nothing is presented as a measurement of real hardware.

## Run

```sh
make serve            # core API on 127.0.0.1:8090
make dashboard-dev    # npm install + dev server on http://localhost:3001
```

The Sandbox is at <http://localhost:3001/sandbox>. The game state streams over Server-Sent Events through the `/api/forgelab/*` rewrite; if the stream cannot be held open the page falls back to polling. The current game id is remembered in the browser so a reload resumes it (games live in the `forgelab serve` process and end when it stops; use Save to keep one).

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
├── app/          pages (Sandbox, Pipelines, Learning path; `/` redirects to Sandbox) and layout
├── components/   Sandbox*, PipelineSimulator, LearningPath, Notice
└── lib/          API types and clients, formatting, timeline, and Sandbox display helpers (tested)
```
