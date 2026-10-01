# Sandbox Engine

The deterministic engine behind the Production Sandbox game.

**Status:** implemented (Phase 10, ADR-0013); configurable Internet traffic (ADR-0016) and the application instance model (ADR-0017) added in Phase 11

## Purpose

Run a player-built production system as a model: traffic, load, latency, errors, money, and users are computed each tick from declared capacities and the topology. Nothing is started on the host; every value is simulated (Principle 15).

## Provided

- `backend/internal/sandbox`: one package covering the Sandbox Engine, Sandbox Ruleset, Flow Solver, Economy & Meters, and Event Deck catalog entries.
- **Ruleset `sandbox/v1`:** ten placeable kinds (CDN, load balancer, API gateway, application instance, worker, database primary, read replica, cache, message queue, object storage), sizes `small`/`medium`/`large`, and economy, growth, and SLO tuning.
- **Ruleset `sandbox/v2`:** v1 plus the Event Deck and incident responses, with revenue per request cut to $0.00015 and starting cash raised to $1,500 so that over-provisioning and incidents cost money. `balance_test.go` pins that economy.
- **Ruleset `sandbox/v3`:** v2 plus fifteen goals (`goals.go`) checked after every tick, and kinds locked until a goal is reached (load balancer after the first request; cache, read replica, queue, worker, and API gateway at 10k users; CDN at 100k users with health of 80 or more). `Game.Goals()` reports each goal's conditions with their current values.
- **Ruleset `sandbox/v4`:** v3 plus a configurable Internet (`traffic.go`): a default configuration of four traffic groups, six endpoints, and four regions; up to 3 retries; load tests up to 1,000,000 RPS; and a CDN that answers 45% of cacheable reads.
- **Ruleset `sandbox/v5`** (new games): v4 plus the application instance model (`app.go`). Sizes give an instance its vCPU, memory, and network, and the ruleset adds the middleware and framework catalogs and the runtime constants.
- **Application model** (`app.go`): `AppConfig` covers sync/async processing, workers, concurrency, backlog, connections, timeout, TLS, keep-alive, middleware, and routes with their costs and dependencies.
  - Capacity emerges each tick as the smallest of the CPU, slot, connection, and network limits.
  - Overload fills the backlog, then requests time out and are rejected.
  - Running out of memory crashes the instance, and it restarts.
  - Health is derived, never set. `NodeStats.App` reports the runtime state and per-route outcomes. `Ruleset.ValidateApp` lists every problem.
- **Traffic model** (`traffic.go`): the Internet's `TrafficConfig` has two parts:
  - **Volume:** from the `market` (users) or a `configured` load test, whose pattern is constant, ramp, spike, burst, periodic, or a daily schedule.
  - **Groups:** each group has a share and its own endpoint mix, region mix, and retries.

  `Ruleset.Validate` lists every problem and corrects nothing. The flow carries a `traffic` breakdown by group, region, and endpoint, with retry RPS and requests in flight.
- **Commands:** `place`, `remove`, `connect`, `disconnect`, `resize`, `scale`, `move`, `respond` (`restart`, `failover`, `rate-limit`, `lift-rate-limit`; v2 and later), and `configure` (the Internet's traffic, v4 and later; an application instance, v5 and later). Each is validated (allowed connections, no loops, enough cash, replica limits, a valid response target, a valid traffic configuration) and appended to the command log.
- **Event Deck** (`events.go`): eleven seeded cards drawn from day two, with odds raised by popularity or complexity. Each card changes model inputs (traffic, attack traffic, replicas up, service time, hit ratio, capacity, cost, a failure share). Events are judged recovered or not one hour after they end. See [`docs/scenarios.md`](../../../docs/scenarios.md).
- **Tick:** event draws, traffic (market: `users × active share × engagement × diurnal curve × traffic events`; load test: `pattern × traffic events`; then groups, request classes, and retries, plus attack traffic), the flow solver per request class, revenue and cost, satisfaction, popularity, growth and churn (held during a load test), and bankruptcy after a day of negative cash. The model is described in [`docs/architecture.md`](../../../docs/architecture.md#traffic-model).
- **Save and replay:** a save is `(ruleset version, seed, tick, command log)`; `Replay` rebuilds an identical world.
- A rolling history of meters (two simulated days) for sparklines.

## Dependencies

None. The engine is pure Go and headless. The [Sandbox API](../sandbox-api/) and [Sandbox Canvas](../sandbox-canvas/) consume it.

## Configuration

Tuning lives in the ruleset. Any change to a ruleset value changes how saves replay, so it ships as a new ruleset version.
