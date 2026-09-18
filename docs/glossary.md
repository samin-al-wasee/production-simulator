# ForgeLab Glossary

**Document status:** Baseline · v1.0

Common production systems terms as used in ForgeLab. Written for learners, not as exhaustive definitions.

## Networking

### Load Balancer
A component that distributes incoming traffic across multiple backend instances so no single instance is overloaded. Typically health-checks its backends and pauses routing to unhealthy ones. Examples: HAProxy, nginx, cloud LBs.

### Reverse Proxy
A server that sits in front of applications, forwarding client requests. It terminates TLS, can buffer/limit requests, and hides backend topology from clients. Unlike a forward proxy (which acts on behalf of clients), it acts on behalf of servers.

### API Gateway
A single entry point for APIs that applies cross-cutting concerns before a request reaches an app: authentication, authorization, rate limiting, request shaping, routing, and aggregation. Often built on a reverse proxy with added policy engine.

### Ingress
In Kubernetes, the object/controller that exposes HTTP(S) routes from outside the cluster to services inside it. It is the cluster-native "load balancer + reverse proxy" concept.

### Service Discovery
The mechanism by which services find each other's network addresses without hardcoded hostnames/ports. Registration (services announce themselves) plus lookup (consumers resolve them) with periodic health updates.

## Resilience

### Circuit Breaker
A stateful failure-protection pattern around a dependency call. While closed, calls flow; when failures cross a threshold it opens and short-circuits (fails fast) for a cooldown, giving the dependency time to recover, then half-opens to probe recovery.

### HPA
**Horizontal Pod Autoscaler** — a Kubernetes controller that adjusts the number of pod replicas based on observed metrics (CPU, memory, or custom metrics) to match demand.

### Replica
A copy of a workload instance. Running N replicas provides redundancy and capacity; in declarative systems (Kubernetes) the desired replica count is the target the system continuously reconciles.

### DLQ
**Dead-letter queue/letter.** Where messages that consumers repeatedly fail to process get moved, so they do not block the main queue and can be inspected, retried manually, or requeued after a fix. The DLQ is the starting point of most "my messages disappeared" investigations.

## Observability

### Trace
An end-to-end record of a single request's journey across all services and systems it touches, composed of spans linked by trace IDs. The backbone of distributed systems debugging.

### Span
A single unit of work within a trace (one operation in one service). Carries name, timing, status, and attributes; spans nest to form parent/child relationships.

### Metrics
Numeric measurements of system behavior over time (request rate, latency, error rate, saturation). Optimized for cheap, high-frequency sampling and alerting — they tell you *something is wrong*, not necessarily *why*.

## Data & Patterns

### Idempotency
The property where applying an operation multiple times has the same effect as applying it once. Critical for retries (e.g. a duplicate payment request must not double-charge) — achieved via idempotency keys, deduplication, or deterministic side effects.

### Saga
A pattern for long-running business transactions across multiple services: a sequence of local transactions, each with a compensating action, so that when a step fails the saga runs compensations in reverse to undo partial work. Trade-off vs 2PC: availability over atomicity.

### CQRS
**Command Query Responsibility Segregation.** Splitting the write path (commands, optimized for consistency) from the read path (queries, optimized for reads) — sometimes using separate models, stores, or services. Improves scaling and modeling clarity at the cost of consistency complexity.