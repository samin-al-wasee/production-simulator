#!/bin/sh
# Verifies the Phase 3 messaging overlay against a running stack
# (make up-msg or make up-all): Redis cache/rate limit, RabbitMQ delivery
# limit -> dead-letter queue, and Kafka produce/consume with consumer lag.
set -eu

RMQ_USER="${RABBITMQ_USER:-forgelab}"
RMQ_PASS="${RABBITMQ_PASSWORD:-forgelab}"
RMQ_API="http://localhost:${RABBITMQ_MANAGEMENT_PORT:-15672}/api"
APP="http://localhost:${PROXY_HTTP_PORT:-8080}"
fail() { echo "FAIL: $1" >&2; exit 1; }
ok() { echo "ok: $1"; }

# Redis: cache-aside and rate limiting through the app.
first=$(curl -fs "$APP/api/cache?key=smoke$$")
echo "$first" | grep -q '"source":"origin"' || fail "first cache read should hit origin: $first"
second=$(curl -fs "$APP/api/cache?key=smoke$$")
echo "$second" | grep -q '"source":"cache"' || fail "second cache read should hit cache: $second"
ok "redis cache-aside"

code=200
for i in $(seq 1 12); do
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "X-Forwarded-For: 198.51.100.$$" "$APP/api/limited")
done
[ "$code" = "429" ] || fail "rate limiter should return 429 after the limit, got $code"
ok "redis rate limiting"

# RabbitMQ: a message rejected past the quorum queue delivery limit lands in the DLQ.
curl -fs -u "$RMQ_USER:$RMQ_PASS" -H 'content-type: application/json' \
  -X POST "$RMQ_API/exchanges/%2F/work/publish" \
  -d '{"properties":{"delivery_mode":2},"routing_key":"jobs","payload":"smoke","payload_encoding":"string"}' \
  | grep -q '"routed":true' || fail "publish was not routed"
for i in 1 2 3 4; do
  curl -fs -u "$RMQ_USER:$RMQ_PASS" -H 'content-type: application/json' \
    -X POST "$RMQ_API/queues/%2F/work.jobs/get" \
    -d '{"count":1,"ackmode":"reject_requeue_true","encoding":"auto"}' >/dev/null
done
# Queue statistics refresh every few seconds, so poll rather than sleep once.
dlq=""
for i in $(seq 1 15); do
  dlq=$(curl -fs -u "$RMQ_USER:$RMQ_PASS" "$RMQ_API/queues/%2F/work.jobs.dlq")
  echo "$dlq" | grep -Eq '"messages_ready":[1-9]' && break
  sleep 2
done
echo "$dlq" | grep -Eq '"messages_ready":[1-9]' || fail "message did not reach work.jobs.dlq"
ok "rabbitmq delivery limit -> dead-letter queue"

# Kafka: produce, consume in a group, then verify the group has no lag.
topic=jobs
printf 'smoke-1\nsmoke-2\n' | docker exec -i forgelab-kafka /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server kafka:9092 --topic "$topic" >/dev/null
got=$(docker exec forgelab-kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server kafka:9092 --topic "$topic" --group smoke --from-beginning \
  --timeout-ms 10000 2>/dev/null || true)
echo "$got" | grep -q smoke-1 || fail "kafka consumer did not receive the produced record"
lag=$(docker exec forgelab-kafka /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server kafka:9092 --describe --group smoke | awk -v t="$topic" '$2==t {s+=$6} END {print s+0}')
[ "$lag" = "0" ] || fail "consumer group lag should be 0, got $lag"
ok "kafka produce/consume with zero lag"

echo "messaging smoke test passed"
