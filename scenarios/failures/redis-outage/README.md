# redis-outage

Category: failures · Status: implemented

## Goal

Verify that a cache outage degrades only cache-backed features, fast, and heals after Redis returns.

## Components involved

Redis (deliberately stopped), sample-web, Chaos Engine. Setup: `make up-msg` (or `make up-all`).

## Expected symptoms

- `/api/cache` returns 503 immediately (connection refused); `/healthz` still returns 200.
- After Redis restarts, `/api/cache` returns 200; the first read is an `origin` read because the cache is empty (no persistence).
- With `make up-all`: `MessagingTargetDown` fires for the `redis` job.

## Investigation

```sh
make chaos FILE=scenarios/failures/redis-outage/experiment.yaml
curl -s localhost:8080/api/cache?key=drill    # observe source: origin then cache
```

## Success criteria

- The experiment reports PASSED.
- You can explain the cold-cache effect after recovery and what a stampede on the origin would look like at scale.
