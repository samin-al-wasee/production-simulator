# ForgeLab Scenarios

**Document status:** v3.0 (the Event Deck is implemented in ruleset `sandbox/v2`)

In the Production Sandbox, a **scenario** is something that happens to the system the player built: a surge, an outage, an attack, a cost shock. Scenarios are cards in the **Event Deck** (`core/internal/sandbox/events.go`). They are seeded, so the same game replays the same events, and their effects are computed by the engine, never scripted (Principle 1).

The Live-mode drills that ran against real containers were retired by ADR-0014; they are on the `archive/live-lab` branch.

## How the deck works

* **Ruleset.** The deck is ruleset data. `sandbox/v1` has no deck, so v1 saves replay exactly as before. New games use `sandbox/v2`.
* **Draws.** From the second simulated day (after a grace period of 288 ticks), each tick rolls once for every card, in deck order. A card is drawn when its roll is below `perDay × (tick length / one day) × driver`. Each tick uses its own random stream, built from `(seed, tick)`.
* **Drivers.** Some cards have a driver that changes their odds. The driver factor is:
  * popularity cards (surges, attacks): `popularity / 50`
  * complexity cards (failures): `complexity / 10`
  * cards without a driver: 1

  Driver factors are clamped between 0 and 3. So a popular system draws more surges and attacks, and a complicated one draws more failures.
* **Limits.** No card can run twice at once. At most three events are pending at a time, counting announced ones. A card that needs a target, for example a database, is skipped when nothing matches.
* **Lifecycle.** Each event goes through four phases: *upcoming* (only if announced ahead), *active*, *recovering*, then *over*.
* **Outcome.** An event is judged one hour (`recoveryTicks`, 12) after it ends. It is **recovered** if system health is at least 80 at that point, and **not recovered** otherwise. The lowest health seen while the event ran is recorded as well.
* **Rule 1 holds throughout.** Every card changes a model input: traffic, capacity, service time, hit ratio, cost, or a failure share. The meters then follow from the flow solver.

## Incident responses

Players respond with `respond` commands, which are recorded in the command log like every other command. Build changes (scale, resize, add replicas, rewire) are always available as well.

| Action | Target | Effect | Refused when |
|---|---|---|---|
| `restart` | a component with crashed replicas | The instance crash ends after `restartTicks` (2 ticks, 10 simulated minutes) | The component has no crashed replicas, or its replicas were lost with their zone |
| `failover` | a database primary | Promotes the healthiest read replica that receives traffic from one of the primary's senders; the two swap roles. Senders that can only write to a primary (workers) move to the new primary. | No healthy read replica shares a sender |
| `rate-limit` / `lift-rate-limit` | an API gateway | Blocks 90% of attack traffic and, by mistake, 1% of real users | The target is not an API gateway, or the limit is already in that state |

Rollback is not a Sandbox response, because the Sandbox models capacity and has no releases. Rollback is taught by the pipeline simulator.

## Scenarios

Magnitudes and durations are drawn from the ranges below. A tick is five simulated minutes.

### `viral-surge`

* **Goal:** learn about headroom, and see where the first bottleneck appears when demand multiplies.
* **Trigger:** about 0.15 per day, multiplied by the popularity factor.
* **Effect on the model:** real traffic × 3–10 for 2–6 hours.
* **Expected symptoms:** RPS jumps. The weakest component on the request path saturates, drops load, and turns red. Latency and errors rise, and satisfaction falls if the surge isn't absorbed.
* **Responses:** scale the saturated component, or add a cache or CDN to take load off it. Scale back down afterwards to protect margins.
* **Success criteria:** health is at least 80 an hour after the surge ends.

### `marketing-spike`

* **Goal:** learn to scale ahead of demand you can see coming, and what that costs.
* **Trigger:** about 0.1 per day. It is **announced two hours ahead**, so it shows as *upcoming*.
* **Effect on the model:** real traffic × 2–3 for 3–6 hours.
* **Expected symptoms:** the same as a surge, but you can see it on the event strip before it arrives.
* **Responses:** scale during the lead time, then scale back down.
* **Success criteria:** health is at least 80 an hour after it ends.

### `seasonal-dip`

* **Goal:** learn to scale down to protect margins.
* **Trigger:** about 0.1 per day.
* **Effect on the model:** real traffic × 0.5–0.7 for a day.
* **Expected symptoms:** revenue falls while cost stays the same, so profit per hour shrinks or goes negative.
* **Responses:** remove idle replicas, and add them back when traffic returns.
* **Success criteria:** health is at least 80 an hour after it ends. The real test is cash.

### `instance-crash`

* **Goal:** learn about redundancy, and how load balancers skip failed targets.
* **Trigger:** about 0.4 per day, multiplied by the complexity factor. It hits one placed component that has a replica up.
* **Effect on the model:** one replica of that component is down for 1–3 hours. A component with one replica is **down**: its capacity is 0 and a balancer sends it nothing.
* **Expected symptoms:** the node shows `DOWN` or `n/m replicas up`. A single-replica component on the only path fails every request that needs it.
* **Responses:** `restart` (the replica is back in 10 minutes), or run two or more replicas so a crash only removes part of the capacity.
* **Success criteria:** health is at least 80 an hour after the crash ends.

### `zone-outage`

* **Goal:** learn about blast radius, and why replicas are spread across zones.
* **Trigger:** about 0.05 per day, multiplied by the complexity factor.
* **Effect on the model:** every placed component loses ⌈replicas / 3⌉ of its replicas for 1–4 hours, because a zone holds a third of each component's replicas.
* **Expected symptoms:** every single-replica component goes down at once, and components with three replicas keep two thirds of their capacity. A design without redundancy serves nothing.
* **Responses:** a restart can't help, since the zone is gone. Adding replicas during the outage restores capacity, because the new replicas land in healthy zones. The lasting fix is three or more replicas on every critical component.
* **Success criteria:** health is at least 80 an hour after the zone returns.

### `db-slowdown`

* **Goal:** see latency cascade through every path that depends on a database.
* **Trigger:** about 0.15 per day, multiplied by the complexity factor. It hits a database primary or read replica.
* **Effect on the model:** that database's service time × 2–4, and its capacity ÷ the same factor, for 1–3 hours.
* **Expected symptoms:** p95 latency climbs across the system. The database's utilization rises and it may start dropping load.
* **Responses:**
  * put a cache in front of the database
  * add read replicas
  * scale or resize the slow database
  * if the slow database is the primary, `failover` to a healthy replica
* **Success criteria:** health is at least 80 an hour after it ends.

### `cache-stampede`

* **Goal:** see what the database really has to carry without the cache.
* **Trigger:** about 0.15 per day, multiplied by the complexity factor. It hits a cache.
* **Effect on the model:** that cache's hit ratio falls to 10–30% for 30 minutes to 2 hours.
* **Expected symptoms:** read load on the databases behind the cache jumps severalfold.
* **Responses:** keep database capacity (replicas) for cache-miss traffic, or run more than one cache.
* **Success criteria:** health is at least 80 an hour after it ends.

### `queue-backlog`

* **Goal:** learn about backlog growth, drain time, and the asynchronous trade-off.
* **Trigger:** about 0.15 per day, multiplied by the complexity factor. It hits a background worker.
* **Effect on the model:** that worker's capacity × 0.2–0.4 for 1–3 hours.
* **Expected symptoms:** the queue's backlog grows. User requests stay fast, because writes are asynchronous, until the backlog reaches its limit and the queue starts dropping.
* **Responses:** scale the workers to drain the backlog, then scale back.
* **Success criteria:** health is at least 80 an hour after it ends.

### `ddos`

* **Goal:** learn about edge capacity, rate limiting, and the cost of absorbing attacks.
* **Trigger:** about 0.1 per day, multiplied by the popularity factor.
* **Effect on the model:** attack traffic of 3–10 × real traffic arrives with it for 1–4 hours. Attack traffic takes capacity like real traffic but earns nothing, and it never counts towards errors or revenue.
* **Expected symptoms:** the *Attack RPS* tile appears and nodes show attack load. The entry path saturates, so real users are crowded out: errors rise and revenue falls.
* **Responses:** put an API gateway on the entry path and `rate-limit` it, which blocks 90% of the attack at the cost of 1% of users. Absorbing the attack with extra capacity also works, but you pay for it.
* **Success criteria:** health is at least 80 an hour after it ends.

### `cost-spike`

* **Goal:** learn about economic resilience and concentration on one component kind.
* **Trigger:** about 0.05 per day. It hits one component kind the player has placed.
* **Effect on the model:** that kind's running cost × 1.5–2.5 for 1–3 days.
* **Expected symptoms:** cost per hour rises and profit shrinks, while health is unchanged.
* **Responses:** trim that kind's replicas, or shift load to other kinds, for example a cache in front of an expensive database.
* **Success criteria:** health is at least 80 an hour after it ends. The real test is staying solvent.

### `third-party-outage`

* **Goal:** learn about errors you can't fix, only absorb.
* **Trigger:** about 0.08 per day.
* **Effect on the model:** 10–30% of real requests fail whatever the design, for 1–3 hours.
* **Expected symptoms:** the error rate rises by that share while every component looks healthy.
* **Responses:** none within the design. Keep the rest of the system healthy so satisfaction survives the outage.
* **Success criteria:** judged like the others. A large outage can end as *not recovered* whatever the player does, and that is part of the lesson.

## Rules

1. A scenario changes model inputs (traffic, capacity, service time, hit ratio, cost), never the output meters directly.
2. Every scenario is seeded and recorded, so replaying a save replays it.
3. Adding or changing a scenario updates this document in the same change. Changing a card's values needs a new ruleset version.
4. Scenario names are `kebab-case` and describe the condition (`viral-surge`, `db-slowdown`).
