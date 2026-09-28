# Tempo

Distributed trace storage and query.

**Status:** implemented (Phase 2, `local` preset overlay)

## Purpose

Distributed trace storage and query.

## Provided

- Single-binary Tempo with local storage and 72h retention, receiving OTLP from the collector.
- Trace search and trace-to-logs correlation through Grafana.

## Dependencies

- OpenTelemetry Collector

## Configuration

`environments/local/observability/tempo/tempo.yaml`.
