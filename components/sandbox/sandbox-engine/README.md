# Sandbox Engine

The deterministic engine behind the Production Sandbox game.

**Status:** implemented (Phase 10, ADR-0013); configurable Internet traffic (ADR-0016), the application instance model (ADR-0017), and traffic components (ADR-0018) added in Phase 11

## Purpose

Run a player-built production system as a model: traffic, load, latency, errors, money, and users are computed each tick from declared capacities and the topology. Nothing is started on the host; every value is simulated (Principle 15).

## Provided

- `backend/internal/sandbox`: one package covering the Sandbox Engine, Sandbox Ruleset, Flow Solver, Economy & Meters, and Event Deck catalog entries.
- **Ruleset `sandbox/v1`:** ten placeable kinds (CDN, load balancer, API gateway, application instance, worker, database primary, read replica, cache, message queue, object storage), sizes `small`/`medium`/`large`, and economy, growth, and SLO tuning.
- **Ruleset `sandbox/v2`:** v1 plus the Event Deck and incident responses, with revenue per request cut to $0.00015 and starting cash raised to $1,500 so that over-provisioning and incidents cost money. `balance_test.go` pins that economy.
- **Ruleset `sandbox/v3`:** v2 plus fifteen goals (`goals.go`) checked after every tick, and kinds locked until a goal is reached (load balancer after the first request; cache, read replica, queue, worker, and API gateway at 10k users; CDN at 100k users with health of 80 or more). `Game.Goals()` reports each goal's conditions with their current values.
- **Ruleset `sandbox/v4`:** v3 plus a configurable Internet (`traffic.go`): a default configuration of four traffic groups, six endpoints, and four regions; up to 3 retries; load tests up to 1,000,000 RPS; and a CDN that answers 45% of cacheable reads.
- **Ruleset `sandbox/v5`:** v4 plus the application instance model (`app.go`). Sizes give an instance its vCPU, memory, and network, and the ruleset adds the middleware and framework catalogs and the runtime constants.
- **Ruleset `sandbox/v6`:** v5 with traffic components instead of the Internet (`client.go`). A game starts empty; the catalog drops the CDN, load balancer, and API gateway until they have a traffic contract; the ruleset carries the default client, the client-type and region shares, and the protocols.
- **Ruleset `sandbox/v13`** (new games) and the **edge** (`edge.go`): the load balancer, API gateway, and CDN with listeners, `LBConfig`, `GatewayConfig`, and `CDNConfig`, per-endpoint forwarding (`forward`) and outcomes (`finishEdge`, `epOutcome`), traffic adopting through the edge, and `NodeStats.Edge`.
- **Ruleset `sandbox/v12`** and **event streams** (`stream.go`): the `event-stream` kind, `StreamConfig` (partitions, retention, event size, key skew), throughput from partitions and brokers, the `stream` dependency, consumer groups with fan-out, lag carried in `Node.Lags`, retention loss, and `NodeStats.Stream`.
- **Ruleset `sandbox/v11`** and the **queue and worker model** (`queue.go`): `QueueConfig` (backlog, visibility timeout, max deliveries) with redelivery from the workers' failure share and a dead-letter count, `WorkerConfig` (concurrency, handler) run through the application model (`workerApp`), and `NodeStats.Queue`.
- **Ruleset `sandbox/v10`** and the **object storage model** (`storage.go`): `StorageConfig` (class, prefixes, object size), throttling per prefix, first byte and transfer latency, usage pricing that replaces the per-replica price, and `NodeStats.Storage`; storage refuses `resize` and `scale`.
- **Ruleset `sandbox/v9`** and the **cache model** (`cache.go`): `CacheConfig` (engine, eviction, TTL, value size, max connections), the hit ratio from fit, freshness, and warmth, CPU and network limits, refused connections, and `NodeStats.Cache`; a node's `Warmth` carries over ticks. `Game.FreeBuild` (in the save) unlocks every kind.
- **Ruleset `sandbox/v8`** and the **database model** (`db.go`): `DBConfig` (max connections, read and write query profiles, hot rows, lock time), sizes with IOPS, data that grows with users, the buffer hit ratio, CPU/IOPS/lock limits, refused connections, replicas applying writes with lag, and `NodeStats.DB`. `Ruleset.ValidateDB` lists every problem.
- **Ruleset `sandbox/v7`** and **connections** (`conn.go`): listeners per kind, a `Connection` on every edge adopted on connect and following its target, the contract on every edge, `Call`s from routes to services by name (sync or async), per-endpoint load (`epMix`), call outcomes with timeouts and retries, pool limits, and `Flow.Edges`. `Ruleset.ValidateConn` and `ValidateListener` list every problem. `testdata/replay.golden` pins every frozen ruleset's replay.
- **Traffic components** (`client.go`): `ClientConfig` is one population (client type, region, protocol, scheme, port, keep-alive, timeout, retries, market or load test, endpoint mix). Each tick `clientLoads` gives every component its segment of the market or its pattern, with retries from its own last failure rate; `contract` refuses a protocol, port, or TLS mismatch; `clientMix` builds each application's endpoint mix from its inputs; `finishClient` applies the shorter timeout and reports `NodeStats.Traffic`. `Ruleset.ValidateClient` lists every problem.
- **Application model** (`app.go`): `AppConfig` covers sync/async processing, workers, concurrency, backlog, connections, timeout, TLS, keep-alive, middleware, and routes with their costs and dependencies.
  - Capacity emerges each tick as the smallest of the CPU, slot, connection, and network limits.
  - Overload fills the backlog, then requests time out and are rejected.
  - Running out of memory crashes the instance, and it restarts.
  - Health is derived, never set. `NodeStats.App` reports the runtime state and per-route outcomes. `Ruleset.ValidateApp` lists every problem. `place` may carry an `AppConfig`, checked before anything is paid; v6 offers application types (routes with typical shares) and stacks in `templates.go`, which the engine never reads; a route's `share` is what an adopting traffic component takes.
- **Traffic model** (`traffic.go`): the Internet's `TrafficConfig` has two parts:
  - **Volume:** from the `market` (users) or a `configured` load test, whose pattern is constant, ramp, spike, burst, periodic, or a daily schedule.
  - **Groups:** each group has a share and its own endpoint mix, region mix, and retries.

  `Ruleset.Validate` lists every problem and corrects nothing. The flow carries a `traffic` breakdown by group, region, and endpoint, with retry RPS and requests in flight.
- **Commands:** `place`, `remove`, `connect`, `disconnect`, `resize`, `scale`, `move`, `respond` (`restart`, `failover`, `rate-limit`, `lift-rate-limit`; v2 and later), and `configure` (the Internet's traffic, v4 and v5; an application instance, v5 and later; a traffic component, v6 and later). Each is validated (allowed connections, one connection per traffic component, no loops, enough cash, replica limits, a valid response target, a valid configuration) and appended to the command log.
- **Event Deck** (`events.go`): eleven seeded cards drawn from day two, with odds raised by popularity or complexity. Each card changes model inputs (traffic, attack traffic, replicas up, service time, hit ratio, capacity, cost, a failure share). Events are judged recovered or not one hour after they end. See [`docs/scenarios.md`](../../../docs/scenarios.md).
- **Tick:** event draws, traffic (market: `users × active share × engagement × diurnal curve × traffic events`; load test: `pattern × traffic events`; then groups, request classes, and retries, plus attack traffic), the flow solver per request class, revenue and cost, satisfaction, popularity, growth and churn (held during a load test), and bankruptcy after a day of negative cash. The model is described in [`docs/architecture.md`](../../../docs/architecture.md#traffic-model).
- **Save and replay:** a save is `(ruleset version, seed, tick, command log)`; `Replay` rebuilds an identical world.
- A rolling history of meters (two simulated days) for sparklines.

## Dependencies

None. The engine is pure Go and headless. The [Sandbox API](../sandbox-api/) and [Sandbox Canvas](../sandbox-canvas/) consume it.

## Configuration

Tuning lives in the ruleset. Any change to a ruleset value changes how saves replay, so it ships as a new ruleset version.
