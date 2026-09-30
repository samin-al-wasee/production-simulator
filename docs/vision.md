# ForgeLab Vision

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

## Why ForgeLab exists

Most engineers learn production systems the hard way: **in production**, with real customers, real money, and real pager burnout. Textbook knowledge ("use a queue", "add a cache", "the database is the bottleneck") does not turn into intuition until you watch a system you built buckle under load and have to decide what to change.

## A city-builder for software production

ForgeLab is the **Production Sandbox**, a game in the spirit of SimCity, but the city is a production system.

* You open the game to an **empty production**: the Internet, full of users, and a little money in the bank.
* You **place every component yourself**: load balancers, application instances, databases, read replicas, caches, queues, workers, CDNs, object storage. Then you wire them together.
* The world **runs**: users arrive, traffic follows the day, popularity rises and falls, and every component has capacity, latency, and a running cost.
* Your choices trade off **cost, revenue, system health, complexity, popularity, user engagement, scale, userbase, and RPS**. Over-provision and you go bankrupt; under-provision and your users leave.
* **Events and incidents** (sudden surges, outages, attacks, cost spikes) test the system you built. *(Event deck: Phase 10, in progress.)*

Everything is a deterministic model computed in the Go core. Nothing is started on your machine, so a system with hundreds of instances and millions of users runs instantly on a laptop. Bottlenecks come from your design, not from a script.

## What it teaches

| Hard to learn in production | How the Sandbox teaches it |
|---|---|
| Where the bottleneck is | Every component shows utilization, latency, and drops; the one that saturates first is the one you under-built |
| Why architectures look the way they do | You add the load balancer, cache, replica, or queue when you need it, and see what it fixes and what it costs |
| Scaling is an economic decision | Revenue, cost, and cash make over- and under-provisioning concrete |
| Users react to latency and errors | Satisfaction drives churn, engagement, popularity, and growth |
| Shipping changes is risky | The pipeline simulator plays rolling, canary, and blue-green deploys, with rollbacks |
| Incidents are expensive to practice | A game can be broken, saved, replayed, and started again |

## Target audience

* **Students and learners** building intuition about production systems
* **Backend and full-stack developers** learning scaling, caching, queues, and resilience
* **Platform and SRE engineers** explaining trade-offs to their teams
* **Educators** running controlled, reproducible exercises: the same seed and moves give the same game

## Long-term goals

1. **A believable model.** Simple, documented formulas that produce the dynamics engineers recognise: saturation, queueing, cascading slowdowns, and the bottleneck that moves when you fix it.
2. **Events and incidents.** Surges, outages, attacks, and cost shocks, with responses such as failover, rate limiting, and rollback.
3. **Goals and missions.** Scenarios with targets ("reach 100k users with health above 80") that tie into the learning path.
4. **Always reproducible.** Any game can be saved, shared, and replayed exactly.
5. **Behavior over business functionality.** Reproduce the engineering characteristics of production systems, not any company's product.
