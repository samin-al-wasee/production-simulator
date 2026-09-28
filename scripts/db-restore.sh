#!/bin/sh
# Restore the local PostgreSQL database from a dump made by db-backup.sh.
# Usage: scripts/db-restore.sh <backup-file>
# The dump contains DROP ... IF EXISTS statements, so restoring replaces the
# objects it contains.
set -eu

FILE="${1:-}"
[ -f "$FILE" ] || { echo "usage: $0 <backup-file>" >&2; exit 2; }
CONTAINER="${DB_CONTAINER:-forgelab-db}"
DB_USER="${POSTGRES_USER:-forgelab}"
DB_NAME="${POSTGRES_DB:-forgelab}"

gunzip -c "$FILE" | docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -q -U "$DB_USER" -d "$DB_NAME" >/dev/null
echo "restored $FILE into $DB_NAME"
