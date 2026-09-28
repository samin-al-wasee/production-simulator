# DLQ Handler

Inspecting and requeueing dead-lettered messages.

**Status:** implemented (Phase 3, `local` preset overlay)

## Purpose

Inspecting and requeueing dead-lettered messages.

## Provided

- Inspection through the RabbitMQ management UI.
- `scripts/dlq-requeue.sh [max]` moves messages from `work.jobs.dlq` back to the `work` exchange.
- `forgelab retry` models when a message is dead-lettered for a given retry policy (`core/internal/retry`).

## Dependencies

- RabbitMQ

## Configuration

Uses `RABBITMQ_USER`, `RABBITMQ_PASSWORD`, and `RABBITMQ_MANAGEMENT_PORT` from the environment.
