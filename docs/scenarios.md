# ForgeLab Scenarios

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

In the Production Sandbox, a **scenario** is something that happens to the system the player built: a surge, an outage, an attack, a cost shock. Scenarios are drawn from the **Event Deck** (planned, ROADMAP Phase 10). They are seeded, so the same game replays the same events, and their effects are computed by the engine, never scripted (Principle 1).

The Live-mode drills that ran against real containers were retired by ADR-0014; they are on the `archive/live-lab` branch.

## Scenario template

Every scenario is documented with:

* **Goal** — what the player should learn from it.
* **Trigger** — what draws the event, and how the world state changes its odds.
* **Effect on the model** — exactly which values change (traffic multiplier, a component's capacity, a cost), and for how long.
* **Expected symptoms** — what the player sees in the meters and on the canvas.
* **Responses** — what the player can do: build changes and, once available, incident actions (failover, rate limit, rollback).
* **Success criteria** — how the game judges a good response (for example, health back above 80 within an hour).

## Planned scenarios

| Scenario | Effect on the model | What it teaches |
|---|---|---|
| Viral surge | Traffic × 3–10 for a few hours; odds rise with popularity | Headroom, autoscaling by hand, where the first bottleneck is |
| Marketing spike | A planned traffic step the player can see coming | Scaling ahead of demand, and its cost |
| Seasonal dip | Traffic falls for a day | Scaling down to protect margins |
| Instance crash | One component's replicas go down | Redundancy, load balancers skipping failed targets |
| Zone outage | Several components go down at once | Blast radius, spreading replicas |
| Database slowdown | Database service time rises | Latency cascading through every dependent path |
| Cache stampede | Cache hit ratio collapses for a while | What the database really has to carry without the cache |
| Queue backlog | Worker capacity falls | Backlog growth, drain time, async trade-offs |
| DDoS | Junk traffic that earns nothing; odds rise with popularity | Edge capacity, rate limiting, cost of absorbing attacks |
| Cost spike | Running cost of one component kind rises | Economic resilience, provider concentration |
| Third-party outage | A share of requests fails regardless of design | Errors you cannot fix, only absorb |

Higher **complexity** raises the odds of failures; higher **popularity** raises the odds of surges and attacks (ADR-0013).

## Rules

1. A scenario changes model inputs (traffic, capacity, service time, hit ratio, cost), never the output meters directly.
2. Every scenario is seeded and recorded, so replaying a save replays it.
3. Adding or changing a scenario updates this document in the same change.
4. Scenario names are `kebab-case` and describe the condition (`viral-surge`, `db-slowdown`).
