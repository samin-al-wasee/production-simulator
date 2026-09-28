# Backup / Restore

Database snapshots and disaster-recovery drills.

**Status:** implemented (Phase 5, `local` preset)

## Purpose

Database snapshots and disaster-recovery drills.

## Provided

- `scripts/db-backup.sh [file]` writes a gzip'd `pg_dump` (default `backups/`, git-ignored); `scripts/db-restore.sh <file>` restores it; `make backup`.
- `scripts/dr-drill.sh`: seed, back up, destroy the database container and volume, rebuild, restore, verify. Requires `FORGELAB_CONFIRM_DESTRUCTIVE=1`.
- Object storage backends are out of scope for the local preset.

## Dependencies

- PostgreSQL
- Docker

## Configuration

`DB_CONTAINER`, `POSTGRES_USER`, `POSTGRES_DB` environment variables (defaults match the local preset).
