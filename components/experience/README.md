# Experience Components

The end-to-end ForgeLab experience: measure the lab, follow the learning path, and run everything together.

**Status:** implemented (Phase 9, ADR-0012).

## Components

| Component | Provides | Status |
|---|---|---|
| Benchmark Reporter | Baseline, ramp, and spike benchmarks with SLO verdicts and Markdown/JSON reports | implemented — see [benchmark-reporter/](benchmark-reporter/) |
| Learning Tracker | Machine-readable learning path with progress and automatic completion | implemented — see [learning-tracker/](learning-tracker/) |
| End-to-End Runner | One command that runs the whole Compose lab | implemented — see [e2e-runner/](e2e-runner/) |

Every component here is optional.
