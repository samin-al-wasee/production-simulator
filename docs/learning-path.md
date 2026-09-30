# ForgeLab Learning Path

**Document status:** v2.1 (goals and automatic completion, Stage 7)

A progressive path through the Production Sandbox and the pipeline simulator. Each stage has a goal and a few exercises; play them in order, or dip in where you need to. The machine-readable form is [`learning/path.yaml`](../learning/path.yaml) (7 stages, 20 exercises).

Most exercises are tied to a **Sandbox goal**: when a game reaches the goal, the exercise completes itself. Goals are checked by the engine, shown on the Sandbox screen with their progress, and some unlock component kinds (see [architecture](architecture.md#goals-and-unlocks)). Exercises that need your judgment (explaining, comparing, writing) are marked done by hand.

| Exercise | Goal that completes it | Unlocks |
|---|---|---|
| s1-serve | `first-request` | Load balancer |
| s1-profit | `profitable-day` | — |
| s2-saturate | `saturate-app` | — |
| s2-scale-out | `scale-out` | — |
| s2-bottleneck-moves | `database-bottleneck` | — |
| s3-cache | `cache` | — |
| s3-replicas | `read-replicas` | — |
| s4-backlog | `backlog` | — |
| s4-drain | `drain-backlog` | — |
| s5-startup | `startup-tier` | Cache, read replica, message queue, background worker, API gateway |
| s5-scale-up | `scale-up-tier` | CDN |
| s5-margin | `healthy-margin` | — |
| s7-recover | `recover-incident` | — |
| s7-ddos | `weather-ddos` | — |
| s7-zone | `survive-zone-outage` | — |

Stages 3 and 4 use kinds unlocked at the startup tier, so in practice they come after Stage 5's first exercise.

## Stage 1 — First production

- **Goal:** Turn an empty world into a system that serves users and pays for itself.
- **Skills:** Request paths, dependencies, reading the meters.
- **Exercises:** serve your first successful request; explain the errors a missing component causes; end a simulated day with more cash than you started it.

## Stage 2 — Capacity and bottlenecks

- **Goal:** Find the component that limits the system and move the limit.
- **Skills:** Utilization, saturation, horizontal vs vertical scaling, load balancing.
- **Exercises:** saturate an application instance; scale out behind a load balancer; watch the bottleneck move to the database; compare a larger size with more replicas.

## Stage 3 — The data tier

- **Goal:** Take read load off the database.
- **Skills:** Caching, hit ratios, read replicas.
- **Exercises:** put a cache in front of the database; spread reads across read replicas.

## Stage 4 — Asynchronous work

- **Goal:** Trade latency for backlog with queues and workers.
- **Skills:** Queues, backlog, worker sizing, drain time.
- **Exercises:** build a backlog; size workers to drain it.

## Stage 5 — Growth and economics

- **Goal:** Grow the userbase without losing money or users.
- **Skills:** Capacity planning, satisfaction and churn, cost vs revenue.
- **Exercises:** reach the startup tier (10k users); reach the scale-up tier (100k users) with health above 80; keep cost below half of revenue at scale-up.

## Stage 6 — Shipping changes safely

- **Goal:** Compare deploy strategies and their rollbacks.
- **Skills:** Rolling, canary, and blue-green deploys; rollback; postmortems.
- **Exercises:** run the three pipeline strategies; watch a bad release roll back; write a postmortem for the worst moment of your game with [`templates/postmortem/postmortem.md`](../templates/postmortem/postmortem.md).

## Stage 7 — Incidents

- **Goal:** Keep the system you built alive through what the Event Deck deals it.
- **Skills:** Incident response (restart, failover, rate limiting), redundancy, blast radius.
- **Exercises:** recover from an incident that pushed health below 80; weather a DDoS attack; survive a zone outage. See [scenarios](scenarios.md).

---

## Tracking your progress

```sh
make learn                             # or: forgelab learn status
forgelab learn next                    # the next exercise and how to play it
forgelab learn complete s1-serve       # mark an exercise done
```

Progress lives in `.forgelab/progress.json` (git-ignored) and is shown on the dashboard's Learning path page. An exercise tied to a goal completes itself when any Sandbox game reaches that goal (recorded as `sandbox <game id>`); any exercise can also be marked done by hand. `forgelab serve -progress <file>` records to a different file. Order is guidance, not a gate.
