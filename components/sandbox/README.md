# Sandbox Components

The Production Sandbox: a model-driven game in which the player builds a production system from an empty world and runs it under growth, events, and incidents. Nothing here starts a container; every value is modelled in the Go core and labelled as simulated (Principle 15).

**Status:** in progress (Phase 10, ADR-0013). The engine is implemented; the API, dashboard canvas, and event deck are planned.

## Components

| Component | Provides | Status |
|---|---|---|
| Sandbox Engine | Deterministic world state, command log, tick loop, save/replay | implemented — see [sandbox-engine/](sandbox-engine/) |
| Sandbox Ruleset | Placeable component kinds, capacities, costs, and tuning as versioned data | implemented — see [sandbox-engine/](sandbox-engine/) |
| Flow Solver | Per-tick load routing, utilization, latency, saturation, and errors | implemented — see [sandbox-engine/](sandbox-engine/) |
| Economy & Meters | Revenue, cost, cash, health, satisfaction, popularity, engagement, complexity, scale | implemented — see [sandbox-engine/](sandbox-engine/) |
| Event Deck | Seeded, state-dependent events and incidents | planned |
| Sandbox API | Games, commands, and a tick stream under `/api/v1/sandbox/` | planned |
| Sandbox Canvas | Dashboard build palette, topology canvas, HUD, and inspector | planned |

The model is described in [`docs/architecture.md`](../../docs/architecture.md#production-sandbox-sandbox-mode). Sandbox mode is optional: Live mode works without it.
