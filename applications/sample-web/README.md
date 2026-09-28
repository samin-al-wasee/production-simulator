# Sample Web Application

Minimal Phase 1 sample application used by the `local` environment preset.
A plain Go HTTP server with **no external dependencies**, so it builds offline.

**Status:** implemented (Phase 1)

## Purpose

Demonstrates a real (Mode A) application plugged into ForgeLab: a container
that runs behind a reverse proxy with a database dependency. It is sample
code only — never part of the platform.

## Provided

- `GET /` — a static HTML page.
- `GET /healthz` — JSON health report; returns `503` while the configured
  PostgreSQL database is unreachable (TCP check).

- `GET /version` — `{version, pod}` from `APP_VERSION` and the hostname, to see which release served a request.
- `GET /metrics` — Prometheus metrics (`http_requests_total`, `http_request_duration_seconds`, `http_requests_in_flight`).
- `GET /api/work?delay_ms=N&cpu_ms=N&fail=1` — simulated work with tunable latency (max 10000 ms), CPU burn (max 2000 ms), and failure, for dashboards, alerts, autoscaling, and drills.
- `GET /api/cache?key=k` — cache-aside through Redis (`source` is `origin` or `cache`); `GET /api/limited` — fixed-window rate limit (429 after 10 requests / 10 s). Both need `REDIS_ADDR` and return 503 without it.
- Structured JSON access logs with `trace_id`; W3C `traceparent` is propagated and spans are exported over OTLP/HTTP when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `APP_ADDR` | `:8080` | Listen address |
| `DB_HOST` | — | Database host (probe target) |
| `DB_PORT` | `5432` | Database port |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | — | Reserved for future database use |
| `APP_VERSION` | `dev` | Release identity reported by `/version` |
| `APP_UNHEALTHY` | — | `1` makes `/healthz` fail (simulates a bad release) |
| `REDIS_ADDR` | — | Redis `host:port`; Redis features are off when unset |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP/HTTP base URL; tracing is off when unset |

## Run

Standalone (no database dependency):

```sh
go run .
```

Within the `local` preset stack, build and run via Docker Compose from the
repository root:

```sh
make up
```