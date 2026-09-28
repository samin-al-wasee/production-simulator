# Messaging Components

Asynchrony and streams: brokers, queues, consumer groups, and dead-letter handling.

**Status:** implemented for the `local` preset (Phase 3) via `environments/local/compose.messaging.yaml` (ADR-0006).

## Components

| Component | Provides | Status |
|---|---|---|
| RabbitMQ | AMQP queues, exchanges, work distribution | implemented — see [rabbitmq/](rabbitmq/) |
| Kafka | Append-only streams, consumer groups, replay | implemented — see [kafka/](kafka/) |
| DLQ Handler | Dead-letter queue UI, retry, analysis | implemented — see [dlq-handler/](dlq-handler/) |

Every component here is optional; sync-only applications can skip this layer entirely.