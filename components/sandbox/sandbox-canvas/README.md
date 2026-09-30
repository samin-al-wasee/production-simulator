# Sandbox Canvas

The dashboard screen where the Production Sandbox is played.

**Status:** implemented (Phase 10, ADR-0013)

## Purpose

Let a player build and run a production system visually. The canvas renders the state the [Sandbox API](../sandbox-api/) streams and sends commands back; it never computes a simulated value.

## Provided

- `frontend/app/sandbox/` and `frontend/components/Sandbox*.tsx`, at <http://localhost:3001/sandbox>.
- **Build palette:** every placeable kind with build and hourly cost; drag onto the canvas or click to place. Kinds the player cannot afford, or that are still locked (🔒, with the goal that unlocks them), are disabled.
- **Goals strip:** goals reached out of the total, and the next three with a progress bar, each condition's value against its target, and what they unlock. A notice announces each goal reached and the kinds it unlocked.
- **Topology canvas** (`@xyflow/react`): drag between handles to connect (only connections the ruleset allows are offered), drag to move, Delete or Backspace to remove a component or connection. Nodes show load, latency, drops, attack load, backlog, replicas up, rate limiting, and a utilization bar; saturated connections turn red.
- **Meters:** cash, profit, revenue, cost, users, RPS, attack RPS (during an attack), p95 latency, errors, health, satisfaction, popularity, complexity, and tier, with sparklines over the last two simulated days. Labelled *simulated*.
- **Event strip:** upcoming and active events with the model input each one changes and the time left, and how recent events were judged.
- **Inspector:** per-component load (including attack and blocked traffic) and cost, size, replicas, remove, downstream connections, and **Respond** actions (restart a crashed component, fail a primary over to a replica, rate-limit a gateway). An action is offered only when the engine would accept it, and the engine validates every command.
- **Controls:** pause, 1×, 2×, 4×, 8×, skip an hour or a day, save, new game. The game id is kept in browser storage so a reload resumes it.

## Dependencies

- Sandbox API
- `@xyflow/react` (dashboard dependency, approved in ADR-0013)

## Configuration

None beyond the dashboard's `FORGELAB_API_URL`.
