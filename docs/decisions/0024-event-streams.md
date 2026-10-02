# ADR-0024: Event streams and event-driven architecture

**Status:** accepted
**Date:** 2026-10-02

## Context

A message queue ([ADR-0023](0023-queue-and-worker-model.md)) hands each message to one consumer. Event-driven systems are built on a different primitive: a durable, partitioned log that any number of services read independently, each at its own pace, for as long as the log retains the events. The game had no way to publish an event once and let several services react to it, and so no way to teach fan-out, partitioning, consumer groups, consumer lag, ordering keys, or retention.

## Decision

1. **A new kind, `event-stream`** (one topic of a Kafka-like log), placeable from v12, listening on Kafka :9092. Applications and workers connect to it to publish, and it connects to workers and applications that consume.
2. **Configuration** (`StreamConfig`, `configure` with `stream`): partitions, retention, event size, and key skew (the share of events on the hottest key).
3. **Publishing:** a route's new `stream` dependency publishes an event to a connected stream; a publish succeeds when the stream accepts it.
4. **Throughput** is the smaller of:
   * partitions: `10 MB/s ÷ event size ÷ hottest partition's share`, the share being `max(1 ÷ partitions, key skew)`, so a hot key funnels into one partition;
   * brokers: `replicas × the size's network ÷ event size`.

   Events above it are throttled and fail their publishers.
5. **Consumer groups:** every component the stream connects to is a group, with each of its replicas a member, and **every group reads every event** (fan-out), unlike a queue. A group's members beyond the partition count sit idle: its rate is `capacity × min(members, partitions) ÷ members`. A worker group runs its handler; an application group receives `POST /events`.
6. **Lag:** a group reads at most what it can handle; the rest accumulates as its lag, carried between ticks, reported in events and seconds (`lag ÷ consumption rate`). Events older than the retention are deleted unread and reported as lost.
7. **Health** is derived: unhealthy when throttling over 20% or losing events, degraded when a group is more than a minute behind or the stream is near capacity.
8. **Templates:** an "Order events" application type publishes orders to a stream and consumes `POST /events`.
9. **Ruleset `sandbox/v12`** (6 partitions, 24 h retention, 1 KB events, no skew; $3 per broker-hour). v1 to v11 replay bit for bit.

## Consequences

### Positive

* One event can drive any number of services, and a slow consumer never slows the producer or the other consumers: it only falls behind.
* Partitions cap both write throughput and consumer parallelism; a hot key defeats them.
* Retention turns lag into data loss, the failure event-driven systems fear most.

### Negative / Trade-offs

* Ordering within a partition is implied by the partition limits, not checked per event.
* A group's processing failures are not redelivered as a queue's are; consumers retry inside their handler and the stream does not see it.
* One stream is one topic; a system with several topics places several streams.

## Alternatives Considered

* **Topics inside one stream component** — more configuration in one node for little extra behavior; the canvas already shows one node per concept.
* **Fan-out through several queues** — that is a different pattern (a queue per consumer) and loses replay and retention.

## References

* [ADR-0019](0019-connections-and-service-calls.md), [ADR-0023](0023-queue-and-worker-model.md), [`docs/architecture.md`](../architecture.md#event-stream-model)
* `backend/internal/sandbox/stream.go`, `stream_test.go`
