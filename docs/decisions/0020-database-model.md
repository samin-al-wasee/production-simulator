# ADR-0020: The database as a modelled data store

**Status:** accepted
**Date:** 2026-10-02

## Context

Up to `sandbox/v7` a database primary or read replica is a flat 300 operations per second per replica with latency `8 ms ÷ (1 − ρ)`. A learner cannot see why a database is slow, why it slows down as the business grows, why indexes matter, why connection pools run out, or why read replicas do not scale writes. With connections in place ([ADR-0019](0019-connections-and-service-calls.md)), the database is the next component to deepen.

## Decision

1. **Configuration on the node** (`DBConfig`, set with `configure`, `db`): engine label, max connections, a read and a write query profile (CPU-ms, pages touched, indexed or not), hot rows and lock time. It applies to each replica.
2. **Resources from the priced size:** vCPU and memory as for applications, and disk IOPS (small 1,000, medium 3,000, large 8,000). 75% of memory is the buffer pool.
3. **Data grows with users:** `200 MB + 50 KB × users`; 20% of it is the working set. The buffer **hit ratio** is `min(1, buffer pool ÷ working set)`.
4. **Query costs:**
   * pages: the profile's pages, plus one page per MB of data for an unindexed query (a scan grows with the data);
   * CPU: `query CPU + 0.005 ms × pages`;
   * disk: reads miss `pages × (1 − hit ratio)`; a write also logs once and writes its pages back;
   * service time: `CPU + disk × 0.5 ms`.
5. **Capacity** is the smallest limit at this tick's read/write mix, reported as the bottleneck:
   * CPU: `vCPU × 1000 ÷ CPU per query`;
   * IOPS: `IOPS ÷ disk operations per query`;
   * locks (a primary): `hot rows × 1000 ÷ lock time` writes per second.

   Latency is service time plus the usual queue wait `mean service × ρ ÷ (1 − ρ)`; writes add a lock wait `lock time × ρ_lock ÷ (1 − ρ_lock)`. Above capacity the excess is dropped, as before.
6. **Connections:** callers open their whole pool from each replica (ADR-0019). Above max connections × replicas, that share of queries is refused.
7. **Replication:** a read replica applies every write the primaries it shares a sender with served on the previous tick, at half a write's CPU and its disk writes, before serving reads. Writes it cannot apply accumulate; **lag** is that backlog over the write rate. A replica more than 10 s behind is degraded. Stale reads are not modelled.
8. **Health** is derived: stopped, unhealthy (more than 20% failing), degraded (ρ > 0.85, more than 1% failing, or lagging), healthy.
9. **Splitting load:** callers split queries among databases by capacity at the previous tick's mix (80/20 before any).
10. **Reported** in the node's flow as `db`; the dashboard adds a database panel, a configuration form, and an inside view.
11. **Ruleset `sandbox/v8`:** the default (2.5 CPU-ms reads over 4 pages, 5 CPU-ms writes over 3, indexed; 100 connections; 500 hot rows at 1 ms) gives a small database about 331 queries per second at the default mix, against v7's 300. v1 to v7 replay bit for bit.

## Consequences

### Positive

* The database slows down as data outgrows memory, and a larger size fixes it; an unindexed query gets worse every day.
* Connection pools and max connections interact: scaling an application out can exhaust its database.
* Replicas visibly do not scale writes: every replica pays for every write, and an undersized replica falls behind.

### Negative / Trade-offs

* A little more capacity per small database than v7 makes over-provisioning a little cheaper: the v8 balance test pins at least 20% of profit lost at 5× headroom, against 25% before.
* Data is shared by every database; per-table sizes, query plans, and transactions are not modelled.
* Replication uses the previous tick's writes, by design.

## Alternatives Considered

* **Free memory or IOPS knobs** — they break the priced economy, as for applications.
* **Per-query sampling** — heavier and noisier than per-class rates (Principle 3).
* **Stale-read errors on lagging replicas** — needs per-key consistency; left for later.

## References

* [ADR-0017](0017-application-instance-model.md), [ADR-0019](0019-connections-and-service-calls.md), [`docs/architecture.md`](../architecture.md#database-model)
* `backend/internal/sandbox/db.go`, `db_test.go`
