# Learning Tracker

Progress through the six-stage learning path.

**Status:** implemented (Phase 9, re-scoped by ADR-0014)

## Purpose

Guide a learner from a first working system to growth at scale, with exercises played in the Production Sandbox and the pipeline simulator.

## Provided

- `learning/path.yaml`: 6 stages and 17 exercises, the machine-readable form of `docs/learning-path.md`.
- `forgelab learn status|next|complete <id>` (`core/internal/learning`); progress in `.forgelab/progress.json` (git-ignored, override with `FORGELAB_PROGRESS`).
- API (`/api/v1/learning`, `POST /api/v1/learning/{id}/complete`) and the dashboard's Learning path page.

## Dependencies

- None; the dashboard is optional.

## Configuration

The path file is validated on load (unique ids, titles, `manual` evidence). Exercises are marked complete by the learner; automatic completion from in-game goals is planned with the goals-and-unlocks milestone.
