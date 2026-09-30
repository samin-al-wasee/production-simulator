# Learning Tracker

Progress through the seven-stage learning path.

**Status:** implemented (Phase 9, re-scoped by ADR-0014; automatic completion from Sandbox goals in Phase 10)

## Purpose

Guide a learner from a first working system to growth at scale, with exercises played in the Production Sandbox and the pipeline simulator.

## Provided

- `learning/path.yaml`: 7 stages and 20 exercises, the machine-readable form of `docs/learning-path.md`. Fifteen exercises name a Sandbox goal as evidence.
- `forgelab learn status|next|complete <id>` (`backend/internal/learning`); progress in `.forgelab/progress.json` (git-ignored, override with `FORGELAB_PROGRESS`).
- API (`/api/v1/learning`, `POST /api/v1/learning/{id}/complete`) and the dashboard's Learning path page.
- Automatic completion: when a Sandbox game reaches a goal, the API completes every exercise whose evidence names it (`RecordGoal`), recorded as `sandbox <game id>`. `forgelab serve -progress <file>` records to another file.

## Dependencies

- None; the dashboard is optional.

## Configuration

The path file is validated on load (unique ids, titles, `manual` or `goal` evidence with a goal name). An API test checks that every goal the path names exists in the latest Sandbox ruleset.
