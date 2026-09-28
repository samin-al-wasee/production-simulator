# Local Environment

Laptop-runnable presets: Docker Compose. Fastest iteration loop.

**Status:** implemented (Phase 1).

## Topology

Minimal single-node stack (ADR-0002): reverse proxy → one application → one
database (PostgreSQL). Everything is declared in `compose.yaml`; nothing is
started manually.

```text
browser ──► nginx (proxy, :8080) ──► sample-web (app, :8080) ──► PostgreSQL (db, :5432)
```

## Requirements

- Docker with the Compose plugin (`docker compose version`).
- The `.devcontainer/` provides this out of the box (Docker-in-Docker).

## Quick start

```sh
cp environments/local/.env.example environments/local/.env   # optional overrides
make up        # build and start the stack
make ps        # show status
curl localhost:8080/          # through the reverse proxy
curl localhost:8080/healthz   # app health incl. database reachability
make logs      # tail container logs
make down      # stop and remove the stack (keeps the database volume)
```

Remove the local database data with:

```sh
docker compose -f environments/local/compose.yaml down -v
```

## How it wires up

| Service | Image | Role |
|---|---|---|
| `proxy` | `nginx:1.27-alpine` | Reverse proxy, publishes `PROXY_HTTP_PORT` (8080) |
| `app` | local build of `applications/sample-web/` | Sample application behind the proxy |
| `db` | `postgres:16-alpine` | Database with a `pg_isready` healthcheck |

Startup ordering is enforced through healthchecks: the app waits for a healthy
database, the proxy waits for a healthy app. All three share a private bridge
network `forgelab`; the database also persists to the `dbdata` volume.

## Observability overlay

```sh
make up-obs    # base stack + Prometheus, Grafana, Loki, Tempo, OpenTelemetry Collector
```

| UI | URL | Notes |
|---|---|---|
| Grafana | http://localhost:3000 | `admin` / `admin` (override in `.env`); dashboard `ForgeLab Overview` |
| Prometheus | http://localhost:9090 | targets, alert rules |

Generate traffic with `curl "localhost:8080/api/work?delay_ms=200"` (add `fail=1` for 500s). Logs and traces are explored through Grafana (Explore → Loki / Tempo). `make down`, `make ps`, and `make logs` cover both files. Details: ADR-0005 and `components/observability/`.

## Messaging overlay

```sh
make up-msg     # base stack + Redis, RabbitMQ, Kafka
make up-all     # base + observability + messaging
make smoke-msg  # verify cache, rate limit, RabbitMQ DLQ, Kafka against the running stack
```

| Service | Endpoint |
|---|---|
| Redis | `localhost:6379` |
| RabbitMQ | AMQP `localhost:5672`, UI http://localhost:15672 (`forgelab` / `forgelab`, override in `.env`) |
| Kafka | `localhost:9092` |

Details: ADR-0006 and `components/messaging/`, `components/databases/redis/`.

## Configuration

Variables come from `environments/local/.env` (see `.env.example`). Defaults
apply if the file is absent. Never commit real credentials — see
`components/databases/postgresql/README.md`.

## Extending

The preset is composable: later phases add observability, messaging, and
reliability services as additional Compose files or services without
restructuring this one.