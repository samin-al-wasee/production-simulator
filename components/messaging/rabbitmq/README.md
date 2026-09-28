# RabbitMQ

AMQP queues, exchanges, work distribution, and dead-letter queues.

**Status:** implemented (Phase 3, `local` preset overlay)

## Purpose

AMQP queues, exchanges, work distribution, and dead-letter queues.

## Provided

- RabbitMQ 4 with the management UI (`RABBITMQ_MANAGEMENT_PORT`, 15672) and AMQP on `RABBITMQ_PORT` (5672).
- Declared topology (`messaging/rabbitmq-definitions.json`, imported by `rabbitmq-init`): exchange `work` → quorum queue `work.jobs` (`x-delivery-limit: 3`) → on exhaustion dead-lettered through `work.dlx` to `work.jobs.dlq`.
- Detailed queue metrics scraped by Prometheus; alert `RabbitMQDeadLetterMessages`.

## Dependencies

- Compute (Docker)

## Configuration

Credentials: `RABBITMQ_USER` / `RABBITMQ_PASSWORD` in `.env`. Move messages out of the DLQ with `scripts/dlq-requeue.sh`.
