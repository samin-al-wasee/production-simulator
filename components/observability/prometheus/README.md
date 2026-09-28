# Prometheus

Metrics collection and alert rules for the `local` preset.

**Status:** implemented (Phase 2, `local` preset overlay)

## Purpose

Metrics collection and alert rules for the `local` preset.

## Provided

- Scrapes the app (`/metrics`), the reverse proxy (via nginx exporter), PostgreSQL (via postgres exporter), the OpenTelemetry Collector, and itself every 15s.
- Reference alert rules: `TargetDown`, `HighErrorRate`, `HighLatencyP95`, `DatabaseDown`, `DatabaseConnectionsHigh`.
- 3-day retention, published on `PROMETHEUS_PORT` (9090).

## Dependencies

- Compute (Docker)
- Scrape targets: application, `nginx-exporter`, `postgres-exporter`, `otel-collector`

## Configuration

Declared in `environments/local/compose.observability.yaml`; scrape config and rules in `environments/local/observability/prometheus/`.
