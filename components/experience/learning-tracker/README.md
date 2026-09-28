# Learning Tracker

Progress through the ten-stage learning path.

**Status:** implemented (Phase 9)

## Purpose

Progress through the ten-stage learning path.

## Provided

- `learning/path.yaml`: 10 stages, 21 exercises, each with manual, experiment, or benchmark evidence.
- `forgelab learn status|next|complete <id>` (`core/internal/learning`); progress in `.forgelab/progress.json` (git-ignored, override with `FORGELAB_PROGRESS`).
- Automatic completion when a mapped chaos experiment passes (CLI or dashboard) or a benchmark meets its SLOs.
- API (`/api/v1/learning`) and the dashboard's Learning path page.

## Dependencies

- Chaos Engine
- Benchmark Reporter
- Dashboard (optional)

## Configuration

The path file is validated on load (unique ids, known evidence types); a test checks that every referenced experiment exists.
