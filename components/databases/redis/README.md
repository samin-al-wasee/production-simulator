# Redis

Cache, pub/sub-capable in-memory store, and rate-limit counter store.

**Status:** implemented (Phase 3, `local` preset overlay)

## Purpose

Cache, pub/sub-capable in-memory store, and rate-limit counter store.

## Provided

- Redis 7 with `maxmemory 128mb` and `allkeys-lru` eviction, no persistence, on `REDIS_PORT` (6379).
- Used by the sample application for cache-aside (`GET /api/cache?key=`) and fixed-window rate limiting (`GET /api/limited`, 10 requests / 10 s per client).
- Metrics through `redis_exporter`; alert `RedisMemoryHigh`.

## Dependencies

- Compute (Docker)

## Configuration

`environments/local/compose.messaging.yaml`, `environments/local/messaging/redis.conf`.
