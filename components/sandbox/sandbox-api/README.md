# Sandbox API

HTTP endpoints that let the dashboard, or any client, play Sandbox games.

**Status:** implemented (Phase 10, ADR-0013); `configure` added in Phase 11 (ADR-0016, ADR-0017)

## Purpose

Expose the [Sandbox Engine](../sandbox-engine/) over HTTP. The API holds games and runs their clocks; every value it returns is computed by the engine.

## Provided

Served by `forgelab serve` (`backend/internal/api/sandbox.go`):

| Endpoint | Does |
|---|---|
| `GET /api/v1/sandbox/ruleset` | The latest ruleset (`sandbox/v6`): placeable kinds, sizes with their resources, tuning, the event deck, goals, regions, the default client and application configurations, the client-type and region shares, protocols, and the middleware and framework catalogs. `?version=sandbox/v5` serves an older one; `400` for an unknown version |
| `GET /api/v1/sandbox/games` | Games held by this server |
| `POST /api/v1/sandbox/games` | New empty game (`{"seed": n, "ruleset": "sandbox/v6"}`, both optional; the latest ruleset by default), or a replay (`{"save": {...}}`) |
| `GET /api/v1/sandbox/games/{id}` | Full state: meters, nodes (the Internet with its `traffic` configuration up to v5; traffic components with their `client` configuration from v6), edges, per-node flow (with `traffic` stats on traffic components) and the `traffic` breakdown, history, events (upcoming, active, recently judged), and goals with their progress |
| `DELETE /api/v1/sandbox/games/{id}` | End a game and its streams |
| `POST /api/v1/sandbox/games/{id}/commands` | Apply a command, including `{"type": "respond", "action": ..., "node": ...}` `{"type": "configure", "node": "internet", "traffic": {...}}` (v4 and v5), `{"type": "configure", "node": "traffic-1", "client": {...}}` (v6), and `{"type": "configure", "node": "app-instance-1", "app": {...}}`; `422` with the reason (every problem, separated by `; `) when invalid |
| `POST /api/v1/sandbox/games/{id}/speed` | `{"speed": 0\|1\|2\|4\|8}` ticks per real second; `0` pauses |
| `POST /api/v1/sandbox/games/{id}/step` | Advance `{"ticks": n}` at once (1 to 2880) |
| `POST /api/v1/sandbox/games/{id}/save` | Write the replayable save to `.forgelab/sandbox/<id>.json` (git-ignored) and return it |
| `GET /api/v1/sandbox/games/{id}/stream` | Server-Sent Events: the state on connect, then after every tick and command |

## Dependencies

- Sandbox Engine

## Configuration

When a game reaches a goal, the server completes the learning-path exercises tied to it in the progress file (best effort; the game never reads progress). Games live in memory, at most 20 per server (creating one more drops the oldest), and end when the server stops. Every state carries a `revision` that increases with each change; stream events and command responses travel separately, so clients keep the state with the highest revision. The endpoints change only in-memory game state and the save directory.
