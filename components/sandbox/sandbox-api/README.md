# Sandbox API

HTTP endpoints that let the dashboard, or any client, play Sandbox games.

**Status:** implemented (Phase 10, ADR-0013)

## Purpose

Expose the [Sandbox Engine](../sandbox-engine/) over HTTP. The API holds games and runs their clocks; every value it returns is computed by the engine.

## Provided

Served by `forgelab serve` (`core/internal/api/sandbox.go`):

| Endpoint | Does |
|---|---|
| `GET /api/v1/sandbox/ruleset` | Placeable kinds, sizes, and tuning for the build palette |
| `GET /api/v1/sandbox/games` | Games held by this server |
| `POST /api/v1/sandbox/games` | New empty game (`{"seed": n}` optional), or a replay (`{"save": {...}}`) |
| `GET /api/v1/sandbox/games/{id}` | Full state: meters, nodes, edges, per-node flow, history |
| `DELETE /api/v1/sandbox/games/{id}` | End a game and its streams |
| `POST /api/v1/sandbox/games/{id}/commands` | Apply a command; `422` with the reason when invalid |
| `POST /api/v1/sandbox/games/{id}/speed` | `{"speed": 0\|1\|2\|4\|8}` ticks per real second; `0` pauses |
| `POST /api/v1/sandbox/games/{id}/step` | Advance `{"ticks": n}` at once (1 to 2880) |
| `POST /api/v1/sandbox/games/{id}/save` | Write the replayable save to `.forgelab/sandbox/<id>.json` (git-ignored) and return it |
| `GET /api/v1/sandbox/games/{id}/stream` | Server-Sent Events: the state on connect, then after every tick and command |

## Dependencies

- Sandbox Engine

## Configuration

Games live in memory, at most 20 per server (creating one more drops the oldest), and end when the server stops. Every state carries a `revision` that increases with each change; stream events and command responses travel separately, so clients keep the state with the highest revision. The endpoints change only in-memory game state and the save directory; unlike experiment runs they need no `-enable-runs` flag.
