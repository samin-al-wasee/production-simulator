# OpenTelemetry

OTLP entry point for traces.

**Status:** implemented (Phase 2, `local` preset overlay)

## Purpose

OTLP entry point for traces.

## Provided

- OpenTelemetry Collector (contrib) receiving OTLP over HTTP (4318) and gRPC (4317) and exporting to Tempo; its own metrics are scraped by Prometheus.
- Applications enable export by setting `OTEL_EXPORTER_OTLP_ENDPOINT` (the overlay sets it for the sample application).

## Dependencies

- Tempo

## Configuration

`environments/local/observability/otel-collector/config.yaml`.
