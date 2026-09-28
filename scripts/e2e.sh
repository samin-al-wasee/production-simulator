#!/bin/sh
# End-to-end run of the Compose lab: start every overlay, wait for health,
# run the messaging smoke test, two outage drills, and a short benchmark, print
# the learning-path progress, and tear the stack down (volumes are kept).
#
# Usage: scripts/e2e.sh          E2E_KEEP=1 keeps the stack running afterwards.
# Kubernetes, cloud, and security-overlay checks are separate:
#   make k8s-up && sh scripts/k8s-security-smoke.sh, make cloud-validate,
#   make up-secure && make smoke-security.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE="docker compose -f $ROOT/environments/local/compose.yaml -f $ROOT/environments/local/compose.observability.yaml -f $ROOT/environments/local/compose.messaging.yaml"
BIN_DIR="$(mktemp -d)"
FL="$BIN_DIR/forgelab"
APP="http://localhost:${PROXY_HTTP_PORT:-8080}"
step=0
fail() { echo "FAIL: $1" >&2; exit 1; }
run() { step=$((step + 1)); printf '\n[%d] %s\n' "$step" "$1"; shift; "$@"; }

cleanup() {
  status=$?
  rm -rf "$BIN_DIR"
  if [ "${E2E_KEEP:-}" = "1" ]; then
    echo "E2E_KEEP=1: stack left running (make down to stop)"
  else
    echo "tearing down the stack (volumes kept)"
    $COMPOSE down >/dev/null 2>&1 || true
  fi
  [ "$status" -eq 0 ] && echo "e2e: PASSED" || echo "e2e: FAILED"
  exit "$status"
}
trap cleanup EXIT

wait_healthy() {
  for i in $(seq 1 90); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$APP/healthz" || true)" = "200" ] && return 0
    sleep 2
  done
  return 1
}

command -v docker >/dev/null 2>&1 || fail "docker is required"
run "build the forgelab CLI" sh -c "cd '$ROOT/core' && go build -o '$FL' ./cmd/forgelab"
run "start the stack (base + observability + messaging)" sh -c "$COMPOSE up -d --build >/dev/null 2>&1"
run "wait for the application health check" wait_healthy || fail "the application never became healthy"
sleep 20 # let exporters and Prometheus complete a scrape cycle

cd "$ROOT"
run "messaging smoke test (Redis, RabbitMQ dead-lettering, Kafka)" sh scripts/messaging-smoke.sh
run "database outage drill" "$FL" chaos run scenarios/failures/db-outage/experiment.yaml
run "Redis outage drill" "$FL" chaos run scenarios/failures/redis-outage/experiment.yaml
run "benchmark (baseline, ramp, spike)" "$FL" benchmark -url "$APP/api/work" -rps 50 -step-duration 5s
run "secret scan" "$FL" security scan-secrets .
run "learning path progress" "$FL" learn status
