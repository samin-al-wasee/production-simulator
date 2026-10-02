# ADR-0023: Message queues with redelivery, and workers as consumers

**Status:** accepted
**Date:** 2026-10-02

## Context

Up to `sandbox/v10` a message queue accepts writes up to its capacity and backlog limit and hands them to workers as they have capacity; a worker is a flat 40 operations per second that writes to a database. Nothing fails asynchronously: a worker's failure is never retried, never dead-lettered, never slower than its flat service time, and the delay a message waits is not shown. Asynchronous processing teaches at-least-once delivery, redelivery storms, poison messages, visibility timeouts, and consumer sizing, none of which the game could show.

## Decision

1. **Queue configuration** (`QueueConfig`, `configure` with `queue`): engine label, max backlog per replica, visibility timeout, max deliveries.
2. **Workers run the application model** ([ADR-0017](0017-application-instance-model.md)). A worker's configuration (`WorkerConfig`, `configure` with `worker`) is a concurrency and a handler: a route with time, CPU, memory, error rate, dependencies, and service calls. It runs as an application with one route (`POST /messages`), as many slots as its concurrency, one process per vCPU, no backlog of its own, and the visibility timeout of the queue that feeds it as its timeout. Its capacity, CPU, memory, connection pools, dependency failures, and health are therefore emergent, as for applications. Workers may connect to databases, caches, storage, and services.
3. **At-least-once delivery:** a message the worker fails, or holds past the visibility timeout, is delivered again. With `f` the share of deliveries the workers failed on the previous tick and `D` max deliveries, each published message takes `1 + f + … + f^(D−1)` deliveries, and `f^D` of messages are dead-lettered. Redeliveries take the queue's room and the workers' capacity like new messages.
4. **The dead-letter queue** accumulates on the queue node (`deadLetters`).
5. **Decoupling:** a publisher succeeds when the queue accepts the message; the workers' failures never reach it. A full backlog rejects publishes, as before.
6. **Reported** as the node's `queue`: published, rejected, delivered, redelivered, dead-lettered and kept, backlog, delay (`backlog ÷ delivery rate`), and the workers' failure share; workers report as applications. The queue-backlog event cuts a worker's capacity through the application model.
7. **Ruleset `sandbox/v11`** (RabbitMQ, 100,000 messages, 30 s visibility, 5 deliveries; workers 10 at a time, a 60 ms handler using 20 CPU-ms that writes to the database). v1 to v10 replay bit for bit.

## Consequences

### Positive

* A failing consumer shows as redeliveries, dead letters, and extra load, while the publishers stay healthy, which is the point of a queue.
* A handler slower than the visibility timeout is redelivered over and over: the classic duplicate-processing bug.
* Workers gain everything applications have: CPU and slot bottlenecks, pools, dependency failures, health.

### Negative / Trade-offs

* Redelivery uses the previous tick's failure share, by design, like client retries.
* Ordering, message priorities, and per-message state are not modelled; messages are rates.
* A worker processing a message it will later redeliver is counted once per delivery, which is what consumers pay for.

## Alternatives Considered

* **A separate worker model** — it would duplicate the application model's resources and pools.
* **Simulating individual messages** — heavier and noisier than rates (Principle 3).

## References

* [ADR-0017](0017-application-instance-model.md), [ADR-0019](0019-connections-and-service-calls.md), [`docs/architecture.md`](../architecture.md#queue-and-worker-model)
* `backend/internal/sandbox/queue.go`, `queue_test.go`
