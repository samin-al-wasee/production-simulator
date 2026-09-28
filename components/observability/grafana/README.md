# Grafana

Dashboards and data exploration across metrics, logs, and traces.

**Status:** implemented (Phase 2, `local` preset overlay)

## Purpose

Dashboards and data exploration across metrics, logs, and traces.

## Provided

- Provisioned datasources: Prometheus, Loki, Tempo (with log-to-trace and trace-to-log links).
- Provisioned dashboard `ForgeLab Overview`: request rate, 5xx ratio, p50/p95/p99 latency, in-flight requests, scrape targets, PostgreSQL connections, proxy connections, application logs.
- Published on `GRAFANA_PORT` (3000).

## Dependencies

- Prometheus
- Loki
- Tempo

## Configuration

Provisioning and dashboards in `environments/local/observability/grafana/`. Credentials come from `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` (default `admin`/`admin`, local use only).
