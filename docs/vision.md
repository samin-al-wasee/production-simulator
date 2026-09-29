# ForgeLab Vision

**Document status:** Baseline · v1.0

## Why ForgeLab exists

Most engineers learn production systems the hard way: **in production**, with real customers, real money, and real pager burnout.

## A simulator, not merely automation

ForgeLab is **not merely infrastructure automation**. It is a **Production Systems Simulation and Engineering Platform** — capable of representing production systems much larger than the underlying hardware through deterministic **resource virtualization** and accepting applications, *synthetic* applications, and raw workload definitions as inputs.

### Three inputs, one production model

ForgeLab accepts three kinds of input and reduces them all to a single production model:

```text
                    ForgeLab
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
   Real Application  Synthetic    Workload
                     Application   Definition
          │            │            │
          └────────────┼────────────┘
                       ▼
              Production Model
                       │
                       ▼
             Simulation Engine
                       │
       ┌───────────────┼────────────────┐
       ▼               ▼                ▼
 Infrastructure     Traffic          Scenarios
       │               │                │
       └───────────────┼────────────────┘
                       ▼
                Observability
                       │
                       ▼
                 Analysis/UI
```

* **Real Applications** — the user brings existing code (Rails, Phoenix, Django, FastAPI, Spring Boot, Node.js, Next.js, etc.) and ForgeLab deploys and observes it.
* **Synthetic Applications** — ForgeLab generates a production-like application from configuration alone; no business logic is required. The generated application models the **operations** (HTTP, DB, cache, messaging, workers, latency) that create real system behavior.
* **Workload Definitions** — the future Workload DSL, declaring the same operations directly.

All three feed one **Production Model**, which the Simulation Engine runs against infrastructure, traffic, and scenarios, under observability, presented through the analysis/UI layer.

A laptop has maybe 16 GB of RAM and 8 CPU cores; the production system ForgeLab simulates may have hundreds of servers, terabytes of memory, millions of RPS, and virtually unlimited horizontal scaling. The simulator does not fake how those systems *behave* — it virtualizes how much capacity they *display*. Bottlenecks, latency, and exhaustion dynamics remain real and honest at whatever scale the user selects (×64, ×128, …), and every surface shows both physical host metrics and virtual production metrics.

That distinction — *physical resources* vs *virtual production resources* — is a core ForgeLab concept, detailed in `docs/architecture.md` (Resource Virtualization Engine) and governed by the principles in `docs/principles.md`.

### Two modes: Live and Sandbox

ForgeLab runs a production system in two ways:

* **Live mode** — real containers, Kubernetes, and cloud presets, with virtualized capacity. The ground truth.
* **Sandbox mode** — a city-builder for software production. The player opens the dashboard to an **empty production**, places every component (load balancers, application instances, databases, caches, queues), wires them together, and runs the result as a living system. Users arrive, traffic rises and falls, surges and incidents happen, and every choice trades off **cost, revenue, health, complexity, popularity, engagement, scale, userbase, and RPS**. The model is deterministic and computed in the Go core; bottlenecks come from the player's design, not a script (ADR-0013).

Live mode teaches how production *behaves*. Sandbox mode teaches how production *evolves*, and why architectures end up the way they do.

Textbook knowledge ("use a queue", "add a retry", "the database is the bottleneck") does not translate into intuition. Intuition about production comes from *observing systems under realistic conditions* — high traffic, degraded infrastructure, failing dependencies, cascading failures, and the messy work of finding out what is actually happening.

ForgeLab exists to make that experience deliberate, safe, and repeatable:

* **Deliberate** — you choose the failure, the traffic, and the architecture.
* **Safe** — it is a lab. Breaking things is the point, and nothing real is lost.
* **Repeatable** — every environment, incident, and benchmark is declared and versioned.

## Problems it solves

| Problem | How ForgeLab solves it |
|---|---|
| Production systems are opaque | The lab is **observable by default** — metrics, logs, and traces on every component |
| Incidents are expensive to learn from | Incidents are **run as scenarios**: reproducible, inspectable, restartable |
| "Architecture diagrams" don't match reality | The lab **runs the actual architecture** you declared |
| Scaling knowledge is theoretical | Traffic, latency, and load are **applied and measured**, not imagined |
| Hardware is finite | A laptop simulates **production-scale capacity** — hundreds of nodes, terabytes of RAM, millions of RPS — through deterministic resource virtualization |
| Building a real app is a prerequisite | **Synthetic applications** are generated from configuration alone; no business logic required to run production-like workloads |
| Real apps are hard to model at scale | The **workload engine** turns operations (HTTP, DB, cache, messaging, workers) into observable behavior |
| Distributed-systems failure modes are abstract | Partial failure, partitioning, and network chaos are **exercised directly** |
| Onboarding new platforms is slow/fearful | A **throwaway playground** where engineers can mutate anything |

## Target audience

* **Students and learners** building intuition about production systems
* **Platform and SRE engineers** prototyping runtimes, observability, and recovery procedures
* **Backend/full-stack developers** learning distributed systems, queues, and resilience
* **Engineering teams** practicing on-call skills and incident response
* **Educators and trainers** running controlled, reproducible exercises

No audience needs to know ForgeLab's internals. They bring their app and their goals.

## Long-term goals

1. **Run any application in a production-like environment** with a single manifest.
2. **Enable any subset of production components** — from a lone database to a full microservice platform.
3. **Provide realistic scenarios** — traffic, failures, security, scaling, and performance drills that behave like production incidents.
4. **Be observable by default** — every experiment produces inspectable metrics, logs, and traces.
5. **Stay cloud agnostic and infrastructure-as-code** — identical behavior from a laptop to a managed cloud.
6. **Be a learning platform** — a guided path from "first Docker container" to "run a distributed incident simulation."
7. **Represent production at any scale** — virtualize capacity so environments much larger than the host hardware are simulated honestly, with physical and virtual metrics always both visible.
8. **Run production workloads without building an app** — generate synthetic applications and workload definitions from configuration alone, so anyone can exercise production behavior immediately.
9. **Optimize for system behavior, not business functionality** — reproduce the engineering characteristics of production systems (read-heavy, write-heavy, message-heavy, failure-prone, …) rather than recreating specific products.

## Application vs platform

ForgeLab makes these distinctions primary:

* An **application** is an input, in one of two forms:
  * A **real application** the user brings: a Rails app, a Phoenix service, an SPA + API, a set of microservices.
  * A **synthetic application** ForgeLab generates from a workload definition — production-like behavior with no business logic.
* The **platform** is what ForgeLab provides: the runtime, the components, the observability, the scenarios, and the dashboard. It is the **product**.

Consequences:

* ForgeLab must **never embed business logic** from any application.
* Applications must be **declared, not hardcoded** — anything shipped in this repo is a sample.
* The platform must be **composable**: every component is optional and removable.
* Synthetic applications model **behavior, not features** — business terminology is a visualization layer.
* The line between platform and app is a boundary, not a blur — documented in `docs/architecture.md`.