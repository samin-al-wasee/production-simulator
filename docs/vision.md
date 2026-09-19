# ForgeLab Vision

**Document status:** Baseline · v1.0

## Why ForgeLab exists

Most engineers learn production systems the hard way: **in production**, with real customers, real money, and real pager burnout.

## A simulator, not merely automation

ForgeLab is **not merely infrastructure automation**. It is a **Production Systems Simulator** — capable of representing production environments much larger than the underlying hardware through deterministic **resource virtualization**.

A laptop has maybe 16 GB of RAM and 8 CPU cores; the production system ForgeLab simulates may have hundreds of servers, terabytes of memory, millions of RPS, and virtually unlimited horizontal scaling. The simulator does not fake how those systems *behave* — it virtualizes how much capacity they *display*. Bottlenecks, latency, and exhaustion dynamics remain real and honest at whatever scale the user selects (×64, ×128, …), and every surface shows both physical host metrics and virtual production metrics.

That distinction — *physical resources* vs *virtual production resources* — is a core ForgeLab concept, detailed in `docs/architecture.md` (Resource Virtualization Engine) and governed by the principles in `docs/principles.md`.

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

## Application vs platform

ForgeLab makes this distinction primary:

* An **application** is what the user brings: a Rails app, a Phoenix service, an SPA + API, a set of microservices. It is an **input**.
* The **platform** is what ForgeLab provides: the runtime, the components, the observability, the scenarios, and the dashboard. It is the **product**.

Consequences:

* ForgeLab must **never embed business logic** from any application.
* Applications must be **declared, not hardcoded** — anything shipped in this repo is a sample.
* The platform must be **composable**: every component is optional and removable.
* The line between platform and app is a boundary, not a blur — documented in `docs/architecture.md`.