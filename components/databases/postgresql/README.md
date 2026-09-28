# PostgreSQL

Relational storage with transactions, the default database of the `local`
preset. Part of the Phase 1 minimal topology: proxy → application → database.

**Status:** implemented (Phase 1, `local` preset)

## Purpose

Provides durable relational storage to the application service(s) running in
a ForgeLab environment.

## Provided

- A running PostgreSQL 16 server on the environment network.
- Health reporting via `pg_isready` for dependency ordering.

## Dependencies

| Component | Nature |
|---|---|
| Compute (Docker) | Runs as a container in the `local` preset |
| Storage | Named volume `dbdata` for data persistence |

## Configuration

Declared in `environments/local/compose.yaml` with secrets supplied through
`environments/local/.env` (never committed). Variables:

| Variable | Default | Meaning |
|---|---|---|
| `POSTGRES_USER` | `forgelab` | Superuser name |
| `POSTGRES_PASSWORD` | `forgelab` | Superuser password (override in `.env`) |
| `POSTGRES_DB` | `forgelab` | Initial database name |
| `POSTGRES_PORT` | `5432` | Published host port |

## Status notes

- Phase 1: single instance, no replication. Replication/recovery drills are
  later increments (Reliability domain).