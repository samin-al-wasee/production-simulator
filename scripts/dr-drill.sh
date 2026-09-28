#!/bin/sh
# Disaster-recovery drill for the local stack: seed data, back it up, destroy
# the database container AND its data volume, rebuild it, restore, and verify.
#
# DESTRUCTIVE: deletes the local database volume. It refuses to run unless
# FORGELAB_CONFIRM_DESTRUCTIVE=1 is set. Requires the running local stack
# (make up).
set -eu

if [ "${FORGELAB_CONFIRM_DESTRUCTIVE:-}" != "1" ]; then
  echo "refusing to run: this drill deletes the local database volume." >&2
  echo "re-run with FORGELAB_CONFIRM_DESTRUCTIVE=1 to proceed." >&2
  exit 2
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE="docker compose -f $ROOT/environments/local/compose.yaml"
CONTAINER="${DB_CONTAINER:-forgelab-db}"
DB_USER="${POSTGRES_USER:-forgelab}"
DB_NAME="${POSTGRES_DB:-forgelab}"
BACKUP="$ROOT/backups/dr-drill-$$.sql.gz"
psql() { docker exec -i "$CONTAINER" psql -q -t -A -U "$DB_USER" -d "$DB_NAME" "$@"; }
fail() { echo "FAIL: $1" >&2; exit 1; }

echo "1/6 seed data"
psql -c "DROP TABLE IF EXISTS dr_drill; CREATE TABLE dr_drill (id serial PRIMARY KEY, note text NOT NULL); INSERT INTO dr_drill (note) VALUES ('alpha'), ('beta'), ('gamma');" >/dev/null
[ "$(psql -c 'SELECT count(*) FROM dr_drill')" = "3" ] || fail "seed did not insert 3 rows"

echo "2/6 backup"
sh "$ROOT/scripts/db-backup.sh" "$BACKUP" >/dev/null

echo "3/6 disaster: remove the database container and its volume"
start=$(date +%s)
$COMPOSE rm -sf -v db >/dev/null 2>&1
docker volume rm -f forgelab-local_dbdata >/dev/null

echo "4/6 rebuild an empty database"
$COMPOSE up -d db >/dev/null 2>&1
for i in $(seq 1 60); do
  [ "$(docker inspect -f '{{.State.Health.Status}}' "$CONTAINER" 2>/dev/null)" = "healthy" ] && break
  sleep 2
done
[ "$(psql -c "SELECT count(*) FROM information_schema.tables WHERE table_name='dr_drill'")" = "0" ] || fail "rebuilt database was not empty"

echo "5/6 restore"
sh "$ROOT/scripts/db-restore.sh" "$BACKUP" >/dev/null

echo "6/6 verify"
[ "$(psql -c 'SELECT count(*) FROM dr_drill')" = "3" ] || fail "restored table does not have 3 rows"
[ "$(psql -c "SELECT note FROM dr_drill ORDER BY id LIMIT 1")" = "alpha" ] || fail "restored data does not match"
end=$(date +%s)

psql -c "DROP TABLE dr_drill" >/dev/null
rm -f "$BACKUP"
echo "PASS: database restored from backup (recovery time $((end - start))s, recovery point = time of backup)"
