# End-to-End Runner

One command that exercises the whole Compose lab.

**Status:** implemented (Phase 9)

## Purpose

One command that exercises the whole Compose lab.

## Provided

- `make e2e` / `scripts/e2e.sh`: start base, observability, and messaging; wait for health; messaging smoke test; database and Redis outage drills; benchmark; secret scan; learning progress; teardown (volumes kept).
- `E2E_KEEP=1` leaves the stack running; the run fails as soon as any step fails.
- Related but separate: `sh scripts/k8s-security-smoke.sh`, `make cloud-validate`, `make smoke-security`, `make compliance`.

## Dependencies

- Docker
- Go toolchain

## Configuration

Uses `PROXY_HTTP_PORT` (default 8080). Set `FORGELAB_PROGRESS=/tmp/scratch.json` to keep the run from recording learning progress.
