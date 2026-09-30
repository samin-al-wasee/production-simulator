# Sandbox Components

The Production Sandbox: a model-driven game in which the player builds a production system from an empty world and runs it under growth, events, and incidents. Nothing here starts a container; every value is modelled in the Go core and labelled as simulated (Principle 1).

**Status:** Phase 10 complete (ADR-0013, ADR-0014); Phase 11 in progress (ADR-0016), starting with a configurable Internet.

## Components

| Component | Provides | Status |
|---|---|---|
| Sandbox Engine | Deterministic world state, command log, tick loop, save/replay | implemented — see [sandbox-engine/](sandbox-engine/) |
| Sandbox Ruleset | Placeable component kinds, capacities, costs, and tuning as versioned data | implemented — see [sandbox-engine/](sandbox-engine/) |
| Flow Solver | Per-tick load routing, utilization, latency, saturation, and errors | implemented — see [sandbox-engine/](sandbox-engine/) |
| Economy & Meters | Revenue, cost, cash, health, satisfaction, popularity, engagement, complexity, scale | implemented — see [sandbox-engine/](sandbox-engine/) |
| Event Deck | Seeded, state-dependent events and incidents | implemented — see [sandbox-engine/](sandbox-engine/) |
| Traffic Model | The Internet's configuration: market or load-test volume, traffic groups, endpoint mix, regions, retries | implemented — see [sandbox-engine/](sandbox-engine/) |
| Sandbox API | Games, commands, and a tick stream under `/api/v1/sandbox/` | implemented — see [sandbox-api/](sandbox-api/) |
| Sandbox Canvas | Dashboard build palette, topology canvas, HUD, and inspector | implemented — see [sandbox-canvas/](sandbox-canvas/) |

The model is described in [`docs/architecture.md`](../../docs/architecture.md). The Sandbox is ForgeLab's whole product (ADR-0014).
