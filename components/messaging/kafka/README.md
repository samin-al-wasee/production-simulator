# Kafka

Append-only streams, consumer groups, replay, and partitioning.

**Status:** implemented (Phase 3, `local` preset overlay)

## Purpose

Append-only streams, consumer groups, replay, and partitioning.

## Provided

- Single-node Apache Kafka 3.9 in KRaft mode on `KAFKA_PORT` (9092); auto topic creation disabled.
- Topics `jobs`, `jobs.retry`, `jobs.dlq` (3 partitions) created by `kafka-init`.
- Consumer group lag through `kafka-exporter`; alert `KafkaConsumerLagHigh`.

## Dependencies

- Compute (Docker)

## Configuration

`environments/local/compose.messaging.yaml`. Retry and dead-letter handling is a consumer-side convention using the `jobs.retry` and `jobs.dlq` topics.
