# ForgeLab Learning Path

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

A progressive path through the Production Sandbox and the pipeline simulator. Each stage has a goal and a few exercises; play them in order, or dip in where you need to. The machine-readable form is [`learning/path.yaml`](../learning/path.yaml) (6 stages, 17 exercises).

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

---

## Tracking your progress

```sh
make learn                             # or: forgelab learn status
forgelab learn next                    # the next exercise and how to play it
forgelab learn complete s1-serve       # mark an exercise done
```

Progress lives in `.forgelab/progress.json` (git-ignored) and is shown on the dashboard's Learning path page. Exercises are marked done by you; automatic completion from in-game goals arrives with the goals-and-unlocks milestone (ROADMAP Phase 10). Order is guidance, not a gate.
