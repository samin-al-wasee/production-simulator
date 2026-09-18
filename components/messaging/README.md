# Messaging Components

Asynchrony and streams: brokers, queues, consumer groups, and dead-letter handling.

**Status:** cataloged in `docs/component-catalog.md` — nothing implemented yet.

## Planned components

| Component | Provides | Status |
|---|---|---|
| RabbitMQ | AMQP queues, exchanges, work distribution | planned |
| Kafka | Append-only streams, consumer groups, replay | planned |
| DLQ Handler | Dead-letter queue UI, retry, analysis | planned |

Every component here is optional; sync-only applications can skip this layer entirely.