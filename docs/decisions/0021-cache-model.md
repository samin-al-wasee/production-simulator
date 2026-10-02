# ADR-0021: The cache as a modelled store

**Status:** accepted
**Date:** 2026-10-02

## Context

Up to `sandbox/v8` a cache serves a fixed 80% of the reads it receives, at 5,000 operations per second per replica. The hit ratio never depends on how much memory the cache has, how long values live, how the cache evicts, how much traffic it sees, or whether it just restarted, which are the questions caching teaches. The database model ([ADR-0020](0020-database-model.md)) now gives a working set that grows with users, which a cache can be measured against.

## Decision

1. **Configuration on the node** (`CacheConfig`, `configure` with `cache`): engine label (Redis, Memcached), eviction policy (LRU, LFU, none), TTL, value size, max connections.
2. **Memory from the size:** 90% of a replica's memory holds values. The keyspace is the database's working set (20% of the data) divided into values of the configured size; requests concentrate on the hottest 5% of keys.
3. **Hit ratio** `= fits^skew × fresh`, at most `warmth`:
   * `fits = min(1, memory ÷ working set)`; `skew` is 0.5 for LRU, 0.4 for LFU (it keeps the hottest), and 1 for none (it stops admitting once full);
   * `fresh = 1 − exp(−TTL × reads/s ÷ hot keys)`: the chance a hot key is read again before it expires (Poisson arrivals);
   * `warmth` is the share of hot keys loaded. A new or restarted cache starts at 0; every miss loads one key. The cache stampede event still caps the hit ratio.
4. **Capacity:** CPU at 0.02 ms per operation and network at the value size per operation, the smaller reported as the bottleneck. Latency is 0.2 ms over the usual utilization factor.
5. **Connections:** as for databases, the callers' pools above max connections are refused, and refused reads never reach the database.
6. **Misses read through** to the database behind, as before. A full cache evicts a key for every key a miss loads.
7. **Health** is derived: stopped, unhealthy (more than 20% failing), starting (not yet warm), degraded, healthy.
8. **Free build.** A game can be created with every kind unlocked (`freeBuild`, kept in the save), so components that the goals unlock late can be studied from the start; goals are still tracked.
9. **Ruleset `sandbox/v9`** (Redis, LRU, 300 s TTL, 2 KB values, 10,000 connections). v1 to v8 replay bit for bit.

## Consequences

### Positive

* Cache size, TTL, eviction, traffic volume, restarts, and data growth all move the hit ratio for reasons a learner can compute.
* A restart is visible as a burst of misses on the database.
* Free build lets a learner study any component without first reaching the goals that unlock it.

### Negative / Trade-offs

* A quiet system hits less: with few requests per hot key, keys expire before they are read again. That is how caches behave, but the early game sees lower hit ratios than v8's fixed 80%.
* Writes and invalidation are not modelled; caches only take reads.
* The skew exponents stand for an access distribution instead of computing one.

## Alternatives Considered

* **A configured hit ratio** — rejected; the hit ratio must come from the design (Principle 2).
* **A full Zipf model** — exact but not checkable by hand; the exponent keeps the effect and the formula short (Principle 3).

## References

* [ADR-0020](0020-database-model.md), [`docs/architecture.md`](../architecture.md#cache-model)
* `backend/internal/sandbox/cache.go`, `cache_test.go`
