# ForgeLab Glossary

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

Common production systems terms as used in ForgeLab. Written for learners, not as exhaustive definitions.

## Sandbox

### Production Sandbox
ForgeLab itself: a model-driven game in which the player builds a production system from an empty world in the dashboard, and the Go core computes every value from declared capacities and the player's topology. Nothing is started on the host; every value is labelled as simulated (ADR-0013, ADR-0014).

### Utilization
Offered load divided by capacity for one component. Near 1 the component slows down sharply; above 1 it drops the excess.

### Satisfaction
The Sandbox's "happiness" meter (0–100): a rolling score of how users experience latency, errors, and outages. It drives churn, engagement, and popularity.

### Tick
One fixed step of simulated time in the Sandbox (initially five simulated minutes). Commands take effect at tick boundaries.

### Ruleset
Versioned Sandbox data: the placeable component kinds with their capacity, service time, cost, and complexity weight, plus economy and event tuning. A save records its ruleset version so it can be replayed.

### Command Log
The ordered list of player actions (place, connect, resize, scale, respond). With the seed and ruleset version, it fully reproduces a game.

### Flow Solver
The Sandbox step that routes each tick's load through the topology and derives utilization, latency, saturation, and errors.

### Event Deck
The seeded set of cards (surges, crashes, zone outage, slowdowns, DDoS, cost spikes, and more) that the Sandbox draws from each tick. A card changes model inputs while it is active, and is judged *recovered* when health is back above 80 an hour after it ends. See `docs/scenarios.md`.

### Incident Response
A `respond` command answering an event: restart a crashed component, fail a database primary over to a read replica, or rate-limit an API gateway.

### Attack Traffic
Requests sent during a DDoS. They take capacity like real traffic but earn nothing; a rate-limited API gateway blocks most of them.

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