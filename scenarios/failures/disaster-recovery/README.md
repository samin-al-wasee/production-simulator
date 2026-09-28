# disaster-recovery

Category: failures · Status: implemented

## Goal

Rehearse losing the database and its data completely, then restore from backup and measure recovery time.

## Components involved

PostgreSQL, Backup / Restore scripts, Docker. Setup: `make up`.

**Destructive:** the drill deletes the local database volume and requires `FORGELAB_CONFIRM_DESTRUCTIVE=1`. Never run it against an environment holding data you need.

## Expected symptoms

- Seeded rows disappear when the volume is removed and the database restarts empty.
- After restore the rows are back and match; the drill prints the recovery time.

## Investigation

```sh
make backup                                             # non-destructive backup to backups/
FORGELAB_CONFIRM_DESTRUCTIVE=1 sh scripts/dr-drill.sh   # full destroy-and-restore drill
```

## Success criteria

- The drill prints PASS with a recovery time.
- You can state the recovery point (the time of the last backup) and what data written after it would have been lost.
