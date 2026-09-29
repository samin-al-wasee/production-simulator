# Sandbox Canvas

The dashboard screen where the Production Sandbox is played.

**Status:** implemented (Phase 10, ADR-0013)

## Purpose

Let a player build and run a production system visually. The canvas renders the state the [Sandbox API](../sandbox-api/) streams and sends commands back; it never computes a simulated value.

## Provided

- `dashboard/app/sandbox/` and `dashboard/components/Sandbox*.tsx`, at <http://localhost:3001/sandbox>.
- **Build palette:** every placeable kind with build and hourly cost; drag onto the canvas or click to place. Kinds the player cannot afford are disabled.
- **Topology canvas** (`@xyflow/react`): drag between handles to connect (only connections the ruleset allows are offered), drag to move, Delete or Backspace to remove a component or connection. Nodes show load, latency, drops, backlog, and a utilization bar; saturated connections turn red.
- **Meters:** cash, profit, revenue, cost, users, RPS, p95 latency, errors, health, satisfaction, popularity, complexity, and tier, with sparklines over the last two simulated days. Labelled *simulated*.
- **Inspector:** per-component load and cost, size, replicas, remove, and downstream connections.
- **Controls:** pause, 1×, 2×, 4×, 8×, skip an hour or a day, save, new game. The game id is kept in browser storage so a reload resumes it.

## Dependencies

- Sandbox API
- `@xyflow/react` (dashboard dependency, approved in ADR-0013)

## Configuration

None beyond the dashboard's `FORGELAB_API_URL`.
