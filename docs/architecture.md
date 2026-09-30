# ForgeLab Architecture

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))
**Primary decision records:** [ADR-0001](decisions/0001-apply-stack-foundations.md) (stack), [ADR-0013](decisions/0013-production-sandbox-game.md) (Sandbox), [ADR-0014](decisions/0014-sandbox-only-platform.md) (Sandbox only)

ForgeLab is the **Production Sandbox**: a city-builder for software production. A new game is an **empty world**, an Internet traffic source and starting cash. The player places components, wires them together, and keeps the system healthy and profitable as users arrive, traffic swings, and incidents happen. Everything is a deterministic model computed in the Go core; nothing runs on the host.

```mermaid
flowchart LR
    subgraph CORE["backend/ (Go)"]
        SB["internal/sandbox<br/>game engine"]
        PL["internal/pipeline<br/>CI/CD simulator"]
        LE["internal/learning<br/>learning path"]
        SS["internal/secretscan<br/>repo hygiene"]
        API["internal/api<br/>HTTP + SSE"]
        CLI["cmd/forgelab<br/>CLI"]
    end
    UI["frontend/ (Next.js)<br/>Sandbox · Pipelines · Learning path"]

    SB & PL & LE --> API
    PL & LE & SS --> CLI
    API -- "state, SSE" --> UI
    UI -- commands --> API
```

| Part | Where | Role |
|---|---|---|
| Sandbox engine | `backend/internal/sandbox` | World state, ruleset, flow solver, economy, meters, save/replay |
| Sandbox API | `backend/internal/api/sandbox.go` | Games, commands, speed, step, save/replay, SSE stream |
| Sandbox canvas | `frontend/app/sandbox`, `frontend/components/Sandbox*` | Build palette, React Flow canvas, meters, inspector |
| Pipeline simulator | `backend/internal/pipeline`, `manifests/pipelines/` | Build, test, and deploy on a virtual clock (rolling, canary, blue-green, rollback) |
| Learning path | `learning/path.yaml`, `backend/internal/learning` | Missions played in the Sandbox and the pipeline simulator; completed automatically by Sandbox goals or by hand; progress in `.forgelab/progress.json` |
| Secret scan | `backend/internal/secretscan`, `security/secretscan.yaml` | Keeps credentials out of the repository |

## Engine layout

```mermaid
flowchart LR
    subgraph CORE["backend/internal/sandbox (pure Go, deterministic)"]
        CMD["Command log<br/>place · connect · resize · scale · respond"]
        WORLD["World state<br/>components · edges · users · cash"]
        CAT["Component catalog<br/>(ruleset data)"]
        TRF["Traffic model<br/>users × engagement × diurnal curve"]
        FLOW["Flow solver<br/>load → utilization → latency/errors"]
        ECO["Economy & meters<br/>revenue · cost · health · satisfaction"]
        EVT["Event deck<br/>(seeded)"]
    end
    API["/api/v1/sandbox<br/>(forgelab serve)"]
    UI["Dashboard Sandbox screen<br/>palette · React Flow canvas · HUD"]

    UI -- commands --> API --> CMD --> WORLD
    CAT --> WORLD
    WORLD --> TRF --> FLOW --> ECO --> WORLD
    EVT --> TRF & FLOW
    WORLD -- tick state (SSE) --> API --> UI
```

A game is fully defined by **(ruleset version, seed, ordered command log)**. Replaying the same triple yields the same world at every tick, so games are reproducible, testable in CI, and saved as data under `.forgelab/sandbox/`.

## One tick

Simulated time advances in fixed ticks (initially five simulated minutes; tuning lives in the ruleset). Each tick:

1. **Apply commands** queued since the last tick, after validation (cash, allowed connections, limits).
2. **Draw events** from the seeded deck; odds depend on complexity (failures) and popularity (surges, attacks). Each tick draws from its own random stream built from `(seed, tick)`, so a replay deals the same cards. The events active this tick become the tick's *effects* on model inputs (below).
3. **Generate traffic:** `RPS = active users × engagement × diurnal(t) × traffic events`, plus attack traffic of `attack magnitude × RPS` during a DDoS.
4. **Solve the flow** through the topology (below).
5. **Update the economy:** revenue for successful requests, cost for every component, operations overhead from complexity.
6. **Update the meters and the userbase:** satisfaction from latency, errors, and availability; growth from popularity; churn from low satisfaction.
7. **Track events:** record each event's lowest health, and judge it one hour after it ends (recovered when health is at least 80).
8. **Check goals:** a goal whose conditions held for its required ticks is reached, permanently, and may unlock component kinds.
9. **Publish** the tick state to subscribers.

## Flow solver (initial model)

The solver walks the topology from the Internet node. The formulas are deliberately simple and documented, so a learner can check them:

* **Routing** — a node splits outgoing load across its downstream edges in proportion to downstream capacity, so a failed node (capacity 0) receives nothing while a healthy peer exists. An application instance sends reads (80%) to a cache if connected, otherwise to database primaries and replicas; writes (20%) to a queue if connected, otherwise to primaries; and a further 10% of requests also need object storage. Missing a required target fails that share of requests. A CDN serves 30% of requests at the edge; a cache serves `hit ratio × reads` (80%) and sends misses to the database. A queue accepts writes up to its capacity and backlog limit and hands them to workers as they have capacity; the backlog carries over between ticks.
* **Utilization** — for a node with capacity `μ` (per replica) and `c` replicas receiving `λ`: `ρ = λ / (c·μ)`.
* **Latency** — `service time / (1 − ρ)` for `ρ < 1` (an M/M/1-style approximation per replica), capped at the timeout. End-to-end latency is the success-weighted mean along the request paths; p95 is approximated as `mean × ln 20 ≈ 3 × mean` (an exponential latency distribution), capped at the timeout. Asynchronous work behind a queue does not add to request latency.
* **Saturation** — when `ρ ≥ 1`, the excess `λ − c·μ` is dropped or queued (queues accumulate backlog up to their limit, then drop). Dropped and timed-out requests are errors.
* **Success** — a request succeeds only if every node on its path serves it; availability and error rate follow from that.

* **Events** change inputs, never outputs. They can:
  * take replicas down: capacity becomes `up replicas × per-replica capacity`, and a component with no replica up is *down*
  * multiply service time and divide capacity (database slowdown)
  * override a cache's hit ratio
  * multiply capacity (workers) or a kind's running cost
  * fail a fixed share of requests whatever the design (third-party outage)

  Replicas that are down still cost money. A down queue keeps its backlog but neither accepts nor delivers messages.
* **Attack traffic** is tracked alongside real traffic through every node. It takes capacity like real traffic, so it crowds out users, but success, errors, and revenue count real requests only. A rate-limited API gateway blocks 90% of the attack traffic it serves and 1% of real requests (false positives).

Bottlenecks are therefore a property of the player's design, not a script (Principle 2).

## Events and responses

The Event Deck is ruleset data: eleven cards, each with odds per day, a driver (popularity or complexity), a duration range, a magnitude range, target kinds, and an optional announcement lead time. The cards and their teaching goals are documented in [`scenarios.md`](scenarios.md). The player answers with `respond` commands:

* `restart`: an instance crash ends in ten minutes
* `failover`: promotes a read replica to primary
* `rate-limit` / `lift-rate-limit`: toggles rate limiting on an API gateway

Build changes are always available as well. Responses are validated and logged like every command.

## Meters

| Meter | Driven by |
|---|---|
| RPS, userbase (total / active) | Traffic model, growth, churn |
| p95 latency, error rate, availability | Flow solver |
| System health (0–100) | Error rate, latency against the SLO, availability |
| Satisfaction (0–100) | Latency, errors, outages over a rolling window |
| Popularity | Satisfaction and events; drives new-user acquisition |
| Engagement | Requests per active user; rises with satisfaction |
| Complexity | Component weights and connections; raises incident odds and operations cost |
| Scale | Tier from userbase and peak RPS |
| Revenue, cost, cash | Successful requests × rate; component and operations cost |

The economy is generic: revenue per successful request, cost per component-hour. No business domain enters the model (Principle 7). A game is lost when cash stays negative past a grace period.

## Rulesets

| Version | What it is |
|---|---|
| `sandbox/v1` | The first ruleset: component kinds, sizes, economy, and growth. It has no events, and stays unchanged so v1 saves replay exactly. |
| `sandbox/v2` | v1 plus the Event Deck and incident responses, with a rebalanced economy. |
| `sandbox/v3` | v2 plus goals and unlocks. New games use it; an older version can be chosen when a game is created. |

**v2 rebalancing.** Under v1, one application instance costing $2/h earned about $90/h at capacity. Over-provisioning therefore always paid, and incidents never threatened solvency.

v2 cuts revenue per successful request from $0.0005 to $0.00015. At that rate, the balance test in `backend/internal/sandbox/balance_test.go` shows the intended shape. It plays a simple design for a simulated week over ten seeds, re-provisioning every hour:
* a design with about 1.5× headroom earns the most and stays solvent
* no headroom loses money at peaks
* 5× over-provisioning gives up at least 30% of the profit
* unanswered events cost money

Starting cash rises from $1,000 to $1,500 to cover the thinner early margin.

## Goals and unlocks

Goals (missions) are ruleset data checked by the engine after every tick. A goal is reached when all of its conditions hold for its required number of ticks, after the goal it requires. There are three kinds of condition:
* a meter within bounds, or its change over a window (for example, cash now against one day ago)
* a statistic over every component of a kind: total served, highest utilization, total replicas, or backlog
* the number of events judged recovered, optionally only those that pushed health below a level

Reaching a goal is permanent and deterministic, so a replay reaches the same goals at the same ticks.

**Unlocks.** A kind can be locked until a goal is reached, and `place` refuses it until then. In `sandbox/v3` a game starts with application instances, database primaries, and object storage:
* the first successful request unlocks the load balancer
* 10,000 users (the startup tier) unlock the cache, read replica, message queue, background worker, and API gateway
* 100,000 users with health of 80 or more unlock the CDN

**The learning path.** Exercises in `learning/path.yaml` can name a goal as their evidence. When a game reaches a goal, the API completes those exercises in the learner's progress file. Data only flows from the game to progress, never back, so a game stays a function of `(ruleset, seed, command log)`. The mapping of goals to exercises is in [`learning-path.md`](learning-path.md).

## Boundaries

* The engine is **headless**; the CLI, tests, or a future analytics service can drive it without the dashboard.
* The dashboard **renders state and sends commands**. It never computes a simulated value (Principle 8).
* Nothing in ForgeLab starts a container, process, or cloud resource; every component is a model.

## Pipeline simulator

`backend/internal/pipeline` plays out a declared pipeline (`manifests/pipelines/*.yaml`) on a virtual clock: build and test steps with durations, cache hits, flaky retries, and a deploy stage using a rolling, canary, or blue-green strategy, with rollback on a bad release. The same pipeline, seed, and options always produce the same run. It is served at `/api/v1/pipelines` and shown on the dashboard's Pipelines page.

## Rules

1. **Nothing is implied.** A new game has no components; the player adds every one.
2. **The dashboard is a consumer.** It renders what the API returns and sends commands; it never reimplements simulation logic.
3. **Everything is simulated and labelled.** No value is presented as a measurement of real hardware or a real service.
4. **Documentation precedes implementation.** A feature is built only when its roadmap phase authorizes it.
