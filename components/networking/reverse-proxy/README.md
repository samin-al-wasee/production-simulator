# Reverse Proxy

Front-facing entry point that terminates HTTP and routes requests to the
application behind it.

**Status:** implemented (Phase 1, `local` preset)

## Purpose

Terminates inbound HTTP and forwards every request to the selected
application service, preserving client identity headers for downstream
services. Part of the Phase 1 minimal topology: proxy → application → database.

## Provided

- HTTP routing from the published host port to the `app` service.
- Standard `X-Forwarded-*` and `Host` headers.

## Dependencies

| Component | Nature |
|---|---|
| Compute (Docker) | Runs as a container in the `local` preset |
| — | The application it routes to |

## Configuration

Delivered in the `local` preset as `environments/local/proxy/nginx.conf`,
mounted read-only into `environments/local/compose.yaml` (nginx image).
Published host port is `PROXY_HTTP_PORT` (default `8080`).

## Status notes

- Phase 1: single upstream (`app`), no TLS. TLS termination is a later
  increment (Security domain).
- Health-checked via `GET /healthz` on the application behind it.