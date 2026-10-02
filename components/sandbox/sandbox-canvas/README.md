# Sandbox Canvas

The dashboard screen where the Production Sandbox is played.

**Status:** implemented (Phase 10, ADR-0013); Internet traffic (ADR-0016), application instance (ADR-0017), and traffic component (ADR-0018) configuration added in Phase 11

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
- **Internet:** selecting the Internet shows its source, pattern, requests, retries, and requests in flight, and a breakdown by group, region, and endpoint. **Configure traffic** opens a form with progressive sections:
  - volume: market or load test, with the pattern and its fields
  - traffic groups, each with its share and its requests and regions
  - endpoints
  - retries

  The form shows running totals but corrects nothing: the engine validates the configuration and the form lists every problem it reports. A load test shows a *load test* badge in the meters and on the Internet node, and the goals strip says goals are paused.
- **Traffic component** (v6): a free palette entry, placed any number of times. A new one asks for nothing and says so; wiring it to an app instance fills in the app's protocol, port, scheme, keep-alive, and routes, **Configure app** updates them, and disconnecting clears them again. Its node is titled by its population and coloured by how its requests fare; a contract problem (wrong protocol or port, a TLS mismatch, not connected) shows on the node, on its edge, and in red in the inspector. The inspector shows the source, requests, retries, attack, successes, failures by reason, latency, and requests in flight. **Configure traffic** opens a form with single choices for clients (name, client type, region), the connection (protocol, scheme, port, client timeout, retries, keep-alive), the volume (market or load test), and the endpoint mix. The engine validates it and the form lists every problem. An older game opens with its own ruleset, so a v5 game still shows the Internet and its forms.
- **Placing an application instance** (v6): clicking or dropping one opens **New application instance**: pick an **application type** (e-commerce, flight booking, ride sharing, social feed, video streaming), which sets the routes and their typical shares, and a **stack** (Django, FastAPI, Express, Rails, Go), which sets the server, processing, workers, port, and middleware; or **define everything manually** in the full app form, where each route also has a typical share. Nothing is placed until it is confirmed, and an invalid configuration is reported in the form.
- **Application instance** (v5): the inspector shows health, bottleneck, CPU, memory, in flight, queued (with the wait), connections, and success, error, timeout, and rejection rates, with per-route RPS and latency. **Configure app** opens a form:
  - application labels and a framework preset
  - server and concurrency (sync or async, workers, backlog, connections, timeout, TLS, keep-alive), with a note on the workers' memory
  - middleware
  - routes, with their costs and dependencies

  The engine validates the configuration and the form lists every problem. Nodes are coloured by health.
- **Inside the Internet:** clicking the Internet node opens it on the canvas with a short zoom animation. It shows regions → traffic groups → endpoints → the components the Internet sends to, with the engine's RPS on each node. Edges are animated while traffic flows, and their width and label show the configured share. **← System** or Esc goes back, and the selection is kept.
- **Inside a traffic component:** clicking one opens it the same way, in three columns: its clients (type, region, RPS) → its endpoints (share and RPS, a 404 marked) → the app it sends to (successes and latency, or the contract problem in red). The connection (protocol, scheme, port, keep-alive, timeout) and source are in the bar; edges carry no labels.
- **Inside an application instance:** clicking an app instance opens it the same way. Its traffic inputs come first (a refused one marked with its reason), then a request's path runs connections → backlog → workers → middleware → routes → dependencies → response, with the engine's values over all replicas: connections and rejections, queued and wait, in flight with CPU and memory, per-route RPS, latency, and errors, and success, error, and timeout rates. The bottleneck, failing parts, and routes that answer 404 are marked. The inspector lists the inputs with their rates and successes. Each dependency names the components it reaches (a cache call falls back to the database, a write goes to a connected queue) or says it is not connected. Edge width and label show each route's share of the traffic.
- **Connections** (v7): clicking an edge opens it in the inspector: attempts, retries, failures, latency, the contract problem, and its client side (protocol, port, TLS, pool, timeout, retries) with **Configure connection**. Data components show what they listen on, with **change**. Edges whose calls fail turn red and name a broken contract. Applications are titled by their service name; their inside view lists every connection with what it carried, linked from the routes that reach it. The app form adds **calls to services** per route (service, endpoint, async).
- **Database** (v8): the inspector shows health, bottleneck, CPU, disk IOPS, buffer hit ratio, data, working set, and buffer pool, connections against the maximum, refused queries, reads and writes with latency, queue and lock waits, and a replica's replication and lag. **Configure database** sets the engine, max connections, read and write queries, and locks. Clicking a database opens its inside view: connections (and replication) → queue → CPU → buffer cache → disk and row locks → response.
- **Cache** (v9): the inspector shows health, the hit ratio and its parts (fits, fresh, warm), hits and misses, memory and keys, evictions, bottleneck, CPU, network, and connections. **Configure cache** sets the engine, eviction, TTL, value size, and max connections. Its inside view: reads → memory → hits and misses → the database behind.
- **Object storage** (v10): a managed service with no size or replicas. The inspector shows requests against the prefixes' rate, throttling, latency as first byte plus transfer, stored GB, egress, and the hourly bill by part. **Configure storage** sets the class, prefixes, and object size. Its inside view: GETs → prefixes (and throttling) → first byte → transfer → bill.
- **Queue and worker** (v11): a queue's inspector shows published, rejected, delivered, redelivered (with the workers' failure share), dead letters, backlog, and delay, with **Configure queue**; its inside view shows publishers and redeliveries → backlog → workers → acknowledged and the dead-letter queue. A worker's inspector shows its health, bottleneck, capacity, CPU, in flight, acknowledged, and failed messages, with **Configure worker** (concurrency, handler cost, errors, dependencies).
- **Event stream** (v12): the inspector shows produced events against the capacity and its limit, throttling, data kept, and each consumer group's rate and lag (or loss), with **Configure stream**. Its inside view: producers → partitions → every consumer group, each reading the whole log at its own pace. Routes and worker handlers gain the `stream` dependency.
- **Free build:** the start screen offers a free-build game, with every component unlocked; the footer says so.
- **Controls:** pause, 1×, 2×, 4×, 8×, skip an hour or a day, save, new game. The game id is kept in browser storage so a reload resumes it.

## Dependencies

- Sandbox API
- `@xyflow/react` (dashboard dependency, approved in ADR-0013)

## Configuration

None beyond the dashboard's `FORGELAB_API_URL`.
