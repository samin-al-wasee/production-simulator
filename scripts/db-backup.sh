#!/bin/sh
# Back up the local PostgreSQL database to a compressed SQL dump.
# Usage: scripts/db-backup.sh [output-file]
# Default output: backups/forgelab-<UTC timestamp>.sql.gz (git-ignored).
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTAINER="${DB_CONTAINER:-forgelab-db}"
DB_USER="${POSTGRES_USER:-forgelab}"
DB_NAME="${POSTGRES_DB:-forgelab}"
OUT="${1:-$ROOT/backups/forgelab-$(date -u +%Y%m%dT%H%M%SZ).sql.gz}"

mkdir -p "$(dirname "$OUT")"
docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" --clean --if-exists | gzip > "$OUT"
[ -s "$OUT" ] || { echo "backup is empty: $OUT" >&2; rm -f "$OUT"; exit 1; }
echo "$OUT"
