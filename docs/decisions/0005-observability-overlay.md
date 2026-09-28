# ADR-0005: Observability as a composable Compose overlay (Phase 2)

**Status:** accepted
**Date:** 2026-09-28

## Context

Phase 2 makes every component observable by default. The `local` preset must stay a minimal three-service stack (ADR-0002) for users who do not want observability, while users who do need metrics, logs, and traces wired end to end, with reference dashboards and alerts.

## Decision

1. **Overlay, not a change to the base stack.** Observability is declared in `environments/local/compose.observability.yaml`, composed on top of `compose.yaml` (`make up-obs`). Removing the file returns the base stack unchanged, keeping every layer optional.
2. **Standard open-source backends:** Prometheus (metrics, alert rules), Grafana (provisioned datasources and the `ForgeLab Overview` dashboard), Loki with Promtail (logs via Docker service discovery, scoped to the compose project), Tempo (traces), and an OpenTelemetry Collector as the OTLP entry point in front of Tempo.
3. **Exporters for components we do not control:** `nginx-prometheus-exporter` (fed by an internal-only `stub_status` on port 8081 of the proxy) and `postgres-exporter`.
4. **The sample application is instrumented without third-party dependencies:** Prometheus text metrics on `/metrics`, JSON access logs carrying `trace_id`, W3C `traceparent` propagation, and OTLP/HTTP JSON span export enabled by `OTEL_EXPORTER_OTLP_ENDPOINT`. This keeps `applications/sample-web` buildable offline and demonstrates what any Mode A application must provide.
5. **Reference alerts** (`observability/prometheus/alerts.yml`): target down, 5xx ratio, p95 latency, database down, database connection saturation.
6. Grafana links logs to traces through a `trace_id` derived field, and traces back to logs.

## Consequences

### Positive

- Metrics, logs, and traces are queryable together and correlated by trace id.
- The base stack and its documentation are unaffected; observability is opt-in.
- Image versions are pinned; everything is declared in versioned files.

### Negative / Trade-offs

- Promtail needs the Docker socket (read-only), which is acceptable for a local lab but not a production pattern.
- Hand-written telemetry in the sample app is not a substitute for the OpenTelemetry SDK; real applications should use their language SDK.
- The default Grafana credentials (`admin`/`admin`) are for local use only and are overridable through `.env`.

## Alternatives Considered

- **Fold observability into `compose.yaml`** — rejected: it would make the minimal topology heavy and violate "every layer is optional".
- **Grafana Alloy instead of Promtail** — reasonable later; Promtail is simpler for Docker discovery today.
- **OpenTelemetry SDK in the sample app** — rejected for now to avoid external dependencies (AGENTS.md §13).
- **Trace export directly to Tempo** — the collector is kept so processors and additional exporters can be added without touching applications.

## References

- `ROADMAP.md` Phase 2 — Observability
- `docs/principles.md` principle 4 (observable by default)
- `docs/decisions/0002-local-single-node-runtime.md`
