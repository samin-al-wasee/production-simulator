# ADR-0030: An application layer and a Postgres application store

**Status:** accepted
**Date:** 2026-10-04

## Context

ForgeLab becomes a product (Phase 13, [ADR-0014](0014-sandbox-only-platform.md)): real users, persistent work they return to, and — later — learning content, problems, submissions, and evaluation. The simulation core is complete and must stay what it is: deterministic, headless, and free of users, passwords, and databases (Principle 8). Identity and persistence are application concerns, not simulation concerns.

Until now the only persisted state sat in `.forgelab` files (learning progress and saved games), keyed to a repository rather than a person. That does not scale to accounts, ownership, or content, and it cannot answer "whose sandbox is this?".

## Decision

1. **A new application layer owns persistence.** It lives beside the simulation packages (`internal/store` for the datastore and migrations; identity, learning content, problems, submissions, and evaluation join it in later slices). Dependencies point **one way**: the application layer imports the Sandbox engine to replay and evaluate a game; no `internal/sandbox`, `internal/pipeline`, or `internal/learning` package imports the store. The core stays runnable with no database.
2. **Postgres is the application datastore**, reached through a `pgx/v5` connection pool. It is the application's database, not the lab infrastructure that ADR-0014 retired: the only container is the app's own deployment image, and the database runs as a local service or a managed instance.
3. **Schema lives in numbered SQL files embedded in the binary**, applied at startup in lexical order, each in its own transaction, recorded in a `schema_migrations` table. The whole run is serialized with a Postgres advisory lock, so several replicas starting at once apply each migration exactly once. There is no external migration tool.
4. **Configuration is one value:** `DATABASE_URL` (or `-database`). When it is unset the API runs without a store, exactly as before — the CLI, the simulation packages, and their tests never need a database.
5. **The database stores application data only:** users, sandbox ownership, the replay record that reproduces a game, progress, learning content, problems, submissions, and evaluation results. A game remains a function of **(ruleset version, seed, command log)**; the database stores that triple, never a tick stream, and never becomes the source of simulation truth.
6. **`GET /readyz`** reports whether the store is reachable; `GET /healthz` stays the liveness check.

## Consequences

### Positive

* The core keeps its guarantees: deterministic, testable without a database, and ignorant of accounts.
* Ownership, content, and evaluation have one home, so B2B requirements can attach to it later without touching the engine.
* Migrations self-apply on boot; deploying a slice is deploying the binary.

### Negative / Trade-offs

* Every running instance opens a database pool; a database is now a runtime dependency of `serve` when configured.
* A startup migration needs a database that accepts the advisory lock; a locked or read-only database fails fast rather than serving stale schema.

## Alternatives Considered

* **A database inside the simulation core** — rejected: it would break determinism and the headless boundary, and duplicate what the engine already recomputes.
* **Storing simulation ticks** — rejected: the replay triple already reproduces every tick exactly; storing ticks would add no truth and much volume.
* **`golang-migrate` or a migration binary** — rejected for now: a second tool and dependency for a job a few dozen lines cover. Revisit if down-migrations or out-of-band migration runs are needed.
* **SQLite or a file-backed store** — rejected: the product is multi-user and hosted; Postgres is the production-appropriate choice the roadmap names.

## References

* [ADR-0014](0014-sandbox-only-platform.md) (Sandbox-only platform), [ADR-0031](0031-oauth-identity.md) (identity)
* [`docs/architecture.md`](../architecture.md), [`ROADMAP.md`](../../ROADMAP.md) Phase 13
