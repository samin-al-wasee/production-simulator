# ForgeLab Scenarios

**Document status:** Baseline · v1.0

Scenarios are reproducible drills that make production conditions inspectable: traffic, failures, security, scaling, and performance. Scenarios marked `implemented` have a folder with a README and can be run today; the rest are cataloged so later phases can build them.

## Scenario template

Every scenario is one folder under `scenarios/<category>/<name>/` with a `README.md` following exactly this structure:

```text
# <scenario-name>

Category: <category> · Status: planned

## Goal
What is being learned or exercised?

## Components involved
Which components must be running for this scenario, and which is deliberately
being broken/loaded?

## Expected symptoms
What should the operator observe (metrics, logs, traces, user-visible errors)?

## Investigation
The steps and signals used to find the root cause.

## Success criteria
How does the operator prove the scenario is understood/resolved?
```

## Rule for new scenarios

* One category from the list below; one folder; one README.
* Must be **declared, reproducible, and safe** — never destructive/irreversible on a shared environment without explicit approval (`AGENTS.md` §10).
* Updating a scenario requires updating this file in the same change.

## Catalog

### Traffic (`scenarios/traffic/`)

| Scenario | Folder | Status |
|---|---|---|
| Traffic spike | `traffic/traffic-spike/` | planned |
| Traffic distribution change | `traffic/traffic-distribution-change/` | planned |

### Failures (`scenarios/failures/`)

| Scenario | Folder | Status |
|---|---|---|
| Cache failure | `failures/cache-failure/` | planned |
| Database slowdown | `failures/db-slowdown/` | planned |
| Database outage | `failures/db-outage/` | implemented |
| Database lock contention | `failures/db-lock-contention/` | planned |
| Connection pool exhaustion | `failures/connection-pool-exhaustion/` | planned |
| Redis outage | `failures/redis-outage/` | implemented |
| Kafka outage | `failures/kafka-outage/` | implemented |
| Redis eviction | `failures/redis-eviction/` | planned |
| RabbitMQ backlog | `failures/rabbitmq-backlog/` | planned |
| Kafka consumer lag | `failures/kafka-consumer-lag/` | planned |
| CPU saturation | `failures/cpu-saturation/` | planned |
| External API latency | `failures/external-api-latency/` | planned |
| External API failure | `failures/external-api-failure/` | planned |
| Retry storm | `failures/retry-storm/` | planned |
| Timeout cascade | `failures/timeout-cascade/` | planned |
| Service failure | `failures/service-failure/` | planned |
| Deployment regression | `failures/deployment-regression/` | planned |
| Pod crash | `failures/pod-crash/` | planned |
| Memory leak | `failures/memory-leak/` | planned |
| Network latency | `failures/network-latency/` | implemented |
| Packet loss | `failures/packet-loss/` | planned |
| Rolling deployment | `failures/rolling-deployment/` | implemented |
| Canary deployment | `failures/canary-deployment/` | implemented |
| Blue/Green deployment | `failures/blue-green-deployment/` | implemented |
| Disaster recovery | `failures/disaster-recovery/` | implemented |

### Security (`scenarios/security/`)

| Scenario | Folder | Status |
|---|---|---|
| Brute force and probing | `security/brute-force-and-probing/` | implemented |
| TLS downgrade attempt | `security/tls-downgrade-attempt/` | implemented |
| Secrets exposure | `security/secrets-exposure/` | implemented |
| RBAC privilege escalation | `security/rbac-privilege-escalation/` | implemented |
| Network segmentation breach | `security/network-segmentation-breach/` | implemented |
| Privileged workload admission | `security/privileged-workload-admission/` | implemented |

### Scaling (`scenarios/scaling/`)

| Scenario | Folder | Status |
|---|---|---|
| Autoscaling under load | `scaling/hpa-scale-out/` | implemented |

### Performance (`scenarios/performance/`)

| Scenario | Folder | Status |
|---|---|---|
| Load testing baseline | `performance/load-test-baseline/` | implemented |

## Synthetic applications & scenarios

**Synthetic applications are the designed substrate for scenarios.** Because they model production operations (HTTP, DB, cache, messaging, workers, latency) they can be driven through realistic incident chains without shipping a real business application:

```text
Generate traffic
       ↓
Synthetic application
       ↓
Database becomes saturated
       ↓
Latency increases
       ↓
Queue backlog grows
       ↓
Workers scale
       ↓
Database remains bottleneck
       ↓
Incident
```

Scenario targets designed to run against synthetic apps (many already cataloged above): cache failure, database slowdown, database lock contention, connection pool exhaustion, Redis eviction, RabbitMQ backlog, Kafka consumer lag, CPU saturation, memory pressure, external API latency, external API failure, retry storm, timeout cascade, traffic spike, traffic distribution change, pod failure, service failure, network latency, packet loss, deployment regression. Synthesis and coordination are described in `docs/architecture.md` → Synthetic Applications / Workload Engine.

## Planned scenario notes (`docs/scenarios.md` placeholder sections)

Each scenario below will own a folder with the five-section template. The notes here are the starting sketch for the catalog-defined ones.

### Traffic spike
- **Goal:** Observe how added capacity behaves through a load spike.
- **Components involved:** Load generator, load balancer, compute (autoscaler), database.
- **Expected symptoms:** Raising latency, growing queue depth, autoscaler reaction, possible throttling.
- **Investigation:** Compare metrics/latency curves; correlate with replica count and DB saturation.
- **Success criteria:** Diagnosis of the bottleneck and evidence of the control loop working (or its limits).

### Database slowdown
- **Goal:** Learn how a slow dependency degrades a healthy app.
- **Components involved:** PostgreSQL, application, observability.
- **Expected symptoms:** Rising app latency, connection pool saturation, slow-query metrics.
- **Investigation:** Trace slow queries, examine pool utilization, find affected endpoints.
- **Success criteria:** Reproduce, isolate to the DB, and show connection/timeout behavior.

### Database outage
- **Goal:** Experience full DB failure and recovery.
- **Components involved:** PostgreSQL, application, backups.
- **Expected symptoms:** Errors, retries, degraded reads, failover or restore.
- **Investigation:** Confirm outage source, observe retry/backoff, drive recovery.
- **Success criteria:** Recovery within target and no silent data corruption.

### Redis outage
- **Goal:** See what breaks when a cache/presence dependency disappears.
- **Components involved:** Redis, application, observability.
- **Expected symptoms:** Cache-miss storms, latency spikes, features failing open/closed.
- **Investigation:** Identify cache-dependent paths, observe fallback behavior.
- **Success criteria:** Explain the app's fail-open/fail-closed stance for Redis.

### RabbitMQ backlog
- **Goal:** Observe queue depth growth and consumer pressure.
- **Components involved:** RabbitMQ, workers, application.
- **Expected symptoms:** Rising message depth, consumer lag, downstream latency.
- **Investigation:** Inspect queue/channel state, consumer throughput, prefetch.
- **Success criteria:** Find the slow consumer and show backlog draining on fix.

### Kafka consumer lag
- **Goal:** Understand consumer-group lag and replay.
- **Components involved:** Kafka, consumers, topic partitions.
- **Expected symptoms:** Growing consumer lag, stale reads, rebalances.
- **Investigation:** Inspect group offsets, lag metrics, per-partition imbalance.
- **Success criteria:** Quantify lag, identify the offender, and show recovery.

### Pod crash
- **Goal:** Observe crash-loop behavior and restart/backoff.
- **Components involved:** Kubernetes, application.
- **Expected symptoms:** Restart cycling, crash-loop backoff, error logs.
- **Investigation:** Review pod status, logs, liveness/readiness probes.
- **Success criteria:** Root-cause the crash and verify the rollout recovers.

### Memory leak
- **Goal:** Track a climbing memory footprint to a leak.
- **Components involved:** Compute, application, Prometheus/Grafana.
- **Expected symptoms:** Monotonic memory growth, OOM kills, restarts.
- **Investigation:** Heap profiles, growth-rate charts, retention vs leak.
- **Success criteria:** Confirm leak pattern and validate the fix reduces usage.

### CPU saturation
- **Goal:** Feel the effect of compute exhaustion on a service.
- **Components involved:** Compute, application, load generator.
- **Expected symptoms:** Throughput drop, latency spike, throttling.
- **Investigation:** CPU graphs, goroutine/thread counts, request queue.
- **Success criteria:** Correlate saturation with degradation and show mitigation.

### Network latency
- **Goal:** Model added network delay on the data path.
- **Components involved:** Networking, application, database/Redis.
- **Expected symptoms:** Higher per-request latency, timeouts, retry storms.
- **Investigation:** Trace spans, p95/p99 trends, dependency call paths.
- **Success criteria:** Locate the added latency and quantify its blast radius.

### Packet loss
- **Goal:** Observe retransmission and degraded throughput.
- **Components involved:** Networking, application.
- **Expected symptoms:** Timeouts, retries, degraded throughput, connection resets.
- **Investigation:** Packet-level capture, error counters, end-to-end traces.
- **Success criteria:** Explain retransmission behavior and its user impact.

### Rolling deployment
- **Goal:** Deploy with zero-downtime expectations.
- **Components involved:** Kubernetes / Deploy Controller, application.
- **Expected symptoms:** Progressive availability, version mix, possible errors on startup.
- **Investigation:** Track rollout progress, per-pod health, error windows.
- **Success criteria:** Complete rollout with no dropped traffic beyond thresholds.

### Canary deployment
- **Goal:** Route a small share to a new version.
- **Components involved:** Load balancer / gateway, application.
- **Expected symptoms:** Dual-version traffic split, canary metrics.
- **Investigation:** Compare error/latency across versions, decide promote/rollback.
- **Success criteria:** A data-driven promote/rollback decision.

### Blue/Green deployment
- **Goal:** Switch entire environments.
- **Components involved:** Networking, compute, application.
- **Expected symptoms:** Full version switch, brief cut-over window.
- **Investigation:** Validate green before switch; monitor post-switch regressions.
- **Success criteria:** Fast, reversible switch with clear rollback path.

### Disaster recovery
- **Goal:** Restore service after a total environment loss.
- **Components involved:** Backups, databases, compute, runbooks.
- **Expected symptoms:** Full unavailability until recovery.
- **Investigation:** Restore from snapshots, validate data integrity, bring up stack.
- **Success criteria:** Recovery time and data-loss objectives are measured and met.

### Security attack simulation
- **Goal:** Observe a simulated attack's effect on the app.
- **Components involved:** Security, networking, application, observability.
- **Expected symptoms:** Anomalous requests, rate-limit trips, auth failures, alert fires.
- **Investigation:** Follow the signals to the attacker pattern (WAF/logs/traces).
- **Success criteria:** The attack is identified, contained, and remediated.