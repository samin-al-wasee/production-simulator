# ADR-0006: Messaging & data overlay with declared retry and dead-letter semantics (Phase 3)

**Status:** superseded by [ADR-0014](0014-sandbox-only-platform.md) (was accepted; the code it describes is on the `archive/live-lab` branch)
**Date:** 2026-09-28

## Context

Phase 3 adds Redis, RabbitMQ, Kafka, and dead-letter/retry semantics. As with observability, these must stay optional layers on the minimal `local` stack, must be declared as code, and must be observable (Phase 2). Retry behavior also needs a deterministic model in the core so it can be reasoned about and tested without a broker.

## Decision

1. **Overlay:** `environments/local/compose.messaging.yaml` (`make up-msg`, or `make up-all` with observability) adds Redis 7, RabbitMQ 4 (management), and a single-node KRaft Kafka. The base stack is unchanged without it.
2. **Redis** is configured as a cache: `maxmemory 128mb`, `allkeys-lru`, no persistence. The sample application uses it for cache-aside (`/api/cache`) and fixed-window rate limiting (`/api/limited`) through a small dependency-free RESP client.
3. **RabbitMQ retry and dead-lettering are declared, not coded:** `work.jobs` is a quorum queue with `x-delivery-limit: 3` and a dead-letter exchange `work.dlx` routing to `work.jobs.dlq`. A message that consumers reject three times lands in the DLQ. Topology is imported by a one-shot `rabbitmq-init` service through the management API, so credentials stay in `.env` (definitions files would need password hashes).
4. **Kafka** topics `jobs`, `jobs.retry`, and `jobs.dlq` (3 partitions) are created by a one-shot `kafka-init`; retry and dead-letter topics are the convention for consumers, since Kafka has no broker-side retry.
5. **DLQ handling:** the RabbitMQ management UI for inspection and `scripts/dlq-requeue.sh` to move messages back to `work`.
6. **Retry semantics model:** `core/internal/retry` (pure Go) defines exponential backoff, attempt limits, and the dead-letter decision; `forgelab retry` prints the schedule for a policy.
7. **Observability:** exporters (`redis_exporter`, `kafka-exporter`, RabbitMQ's built-in Prometheus endpoint with detailed queue metrics) are scraped with an `optional="true"` label so the generic `TargetDown` alert stays quiet when the overlay is off; new alerts cover dead-letter messages, consumer lag, Redis memory, and `MessagingTargetDown`.
8. `scripts/messaging-smoke.sh` (`make smoke-msg`) verifies cache-aside, rate limiting, delivery-limit dead-lettering, and Kafka produce/consume with zero lag against a running stack.

## Consequences

### Positive

- Retry and dead-letter behavior is visible in broker configuration and testable end to end.
- Every new component is scraped and alertable.
- No client libraries were added to the sample application.

### Negative / Trade-offs

- The quorum-queue delivery limit retries immediately; delayed backoff needs a TTL retry queue, which the core model describes but the overlay does not yet declare.
- Kafka runs as a single node with replication factor 1; it teaches consumer groups and lag, not replication failure.
- The RESP client opens one connection per command and is not for production use.

## Alternatives Considered

- **Consumer code implementing retry in the sample app** — rejected: needs AMQP/Kafka client libraries and hides semantics in code.
- **TTL + dead-letter-exchange retry loop** — valid for delayed backoff; deferred, documented as a follow-up.
- **RabbitMQ `load_definitions`** — rejected: users in definitions require password hashes, and without users the default user has no permissions.
- **Redpanda instead of Kafka** — rejected: the catalog and learning path target Apache Kafka.

## References

- `ROADMAP.md` Phase 3 — Messaging & data
- `docs/decisions/0005-observability-overlay.md`
