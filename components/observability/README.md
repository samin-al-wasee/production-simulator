# Observability Components

Seeing what is happening: metrics, logs, traces, and dashboards. ForgeLab requires **observable by default** — completed components ship observability, never an afterthought.

**Status:** implemented for the `local` preset (Phase 2) via `environments/local/compose.observability.yaml` (ADR-0005).

## Planned components

| Component | Provides | Status |
|---|---|---|
| OpenTelemetry | Instrumentation protocol, collectors, exporters | implemented |
| Prometheus | Metrics collection, alert rules | implemented |
| Grafana | Dashboards, alerting UI | implemented |
| Loki | Log aggregation and search | implemented |
| Tempo | Distributed traces | implemented |

A component that cannot be observed is not complete.