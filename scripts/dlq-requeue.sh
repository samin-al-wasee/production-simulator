#!/bin/sh
# Move messages from a RabbitMQ dead-letter queue back to the work exchange.
# Usage: scripts/dlq-requeue.sh [max-messages]   (default 100)
# Requires the messaging overlay (make up-msg). Inspect first in the
# management UI (http://localhost:15672) - requeueing a poison message sends
# it around the retry cycle again.
set -eu

USER_NAME="${RABBITMQ_USER:-forgelab}"
PASS="${RABBITMQ_PASSWORD:-forgelab}"
API="http://localhost:${RABBITMQ_MANAGEMENT_PORT:-15672}/api"
MAX="${1:-100}"
moved=0

while [ "$moved" -lt "$MAX" ]; do
  msg=$(curl -fs -u "$USER_NAME:$PASS" -H 'content-type: application/json' \
    -X POST "$API/queues/%2F/work.jobs.dlq/get" \
    -d '{"count":1,"ackmode":"ack_requeue_false","encoding":"auto"}')
  [ "$msg" = "[]" ] && break
  payload=$(printf '%s' "$msg" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)[0]["payload"]))')
  curl -fs -u "$USER_NAME:$PASS" -H 'content-type: application/json' \
    -X POST "$API/exchanges/%2F/work/publish" \
    -d "{\"properties\":{\"delivery_mode\":2},\"routing_key\":\"jobs\",\"payload\":$payload,\"payload_encoding\":\"string\"}" >/dev/null
  moved=$((moved + 1))
done
echo "requeued $moved message(s) from work.jobs.dlq to work.jobs"
