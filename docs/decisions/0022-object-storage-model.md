# ADR-0022: Object storage as a priced, rate-limited service

**Status:** accepted
**Date:** 2026-10-02

## Context

Up to `sandbox/v9` object storage is a flat component: 1,000 operations per second per replica, 20 ms of service time, and $1 per replica-hour. Real object storage is a managed service: it has no instance sizes, it is limited by request rate per key prefix, a read costs a first byte plus a transfer that depends on the object, and the bill is for what is stored, requested, retrieved, and sent out, where egress usually dominates. The game teaches none of that.

## Decision

1. **Configuration on the node** (`StorageConfig`, `configure` with `storage`): storage class, number of prefixes, mean object size.
2. **A managed service:** object storage has no size or replicas in v10; `resize` and `scale` are refused with the reason. It scales by prefixes.
3. **Rate:** each prefix sustains 5,500 GETs per second. Above that, requests are **throttled** (SlowDown) and fail.
4. **Latency:** the class's time to first byte (standard 20 ms, infrequent access 30 ms, archive 2,000 ms) plus the transfer, `object size ÷ 80 Mbps`, over the usual utilization factor.
5. **Usage pricing** replaces the per-replica price, per hour:
   * stored: `stored GB × class price per GB-month ÷ 730`, with stored data `1 GB + 2 MB × users`;
   * requests: `GETs × class price per 1,000`;
   * retrieval: `GB read × class retrieval price` (infrequent and archive);
   * egress: `GB read × $0.09`.

   Prices follow the public list prices of the major clouds in shape: standard is cheap to read and dear to keep, archive the reverse.
6. **Health** is derived from throttling and utilization. **Reported** as the node's `storage`, with the bill by part.
7. **Ruleset `sandbox/v10`** (standard, 1 prefix, 200 KB objects). v1 to v9 replay bit for bit.

## Consequences

### Positive

* Egress becomes visible as the largest storage cost, setting up the CDN (11.10) as a cost decision as well as a latency one.
* Storage classes trade storage cost against read cost and latency.
* Prefix throttling appears at high request rates.

### Negative / Trade-offs

* Only GETs are modelled; uploads are part of the routes' dependencies as reads, as before.
* Stored data grows with users, not with what the routes write.

## Alternatives Considered

* **Keeping replicas** — object storage has no instances to scale; prefixes are the real lever.
* **Charging egress to the application instead** — egress is billed by the store that sends it.

## References

* [ADR-0019](0019-connections-and-service-calls.md), [`docs/architecture.md`](../architecture.md#object-storage-model)
* `backend/internal/sandbox/storage.go`, `storage_test.go`
