# Sandbox Engine

The deterministic engine behind the Production Sandbox game.

**Status:** implemented (Phase 10, ADR-0013)

## Purpose

Run a player-built production system as a model: traffic, load, latency, errors, money, and users are computed each tick from declared capacities and the topology. Nothing is started on the host; every value is simulated (Principle 15).

## Provided

- `core/internal/sandbox`: one package covering the Sandbox Engine, Sandbox Ruleset, Flow Solver, and Economy & Meters catalog entries.
- **Ruleset `sandbox/v1`:** ten placeable kinds (CDN, load balancer, API gateway, application instance, worker, database primary, read replica, cache, message queue, object storage), sizes `small`/`medium`/`large`, and economy, growth, and SLO tuning.
- **Commands:** `place`, `remove`, `connect`, `disconnect`, `resize`, `scale`, `move`. Each is validated (allowed connections, no loops, enough cash, replica limits) and appended to the command log.
- **Tick:** traffic (`users × active share × engagement × diurnal curve`), the flow solver, revenue and cost, satisfaction, popularity, growth and churn, and bankruptcy after a day of negative cash. The model is described in [`docs/architecture.md`](../../../docs/architecture.md#production-sandbox-sandbox-mode).
- **Save and replay:** a save is `(ruleset version, seed, tick, command log)`; `Replay` rebuilds an identical world.
- A rolling history of meters (two simulated days) for sparklines.

## Dependencies

None. The engine is pure Go and headless. The [Sandbox API](../sandbox-api/) and [Sandbox Canvas](../sandbox-canvas/) consume it.

## Configuration

Tuning lives in the ruleset. Any change to a ruleset value changes how saves replay, so it ships as a new ruleset version.
