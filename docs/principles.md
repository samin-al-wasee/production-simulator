# ForgeLab Principles

**Document status:** Baseline · v1.0

These principles guide every decision in ForgeLab. If a proposal conflicts with a principle, the principle wins unless a documented decision (ADR) intentionally overrides it.

## 1. Platform over application

ForgeLab is the stage, not the playwright. The platform — runtime, components, observability, scenarios — is the product; applications are inputs. The platform never contains an application's business logic.

## 2. Composition over configuration

Users enable exactly the components they need. The default for every feature is **off**. There is no "standard deployment" and no implied stack; every environment is assembled from declared pieces.

## 3. Production parity

The lab must behave like real production. No shortcuts that make failures "nicer" than they would be in the real world. If a real deployment would time out, experience latency, or exhaust memory, the lab does too.

## 4. Observable by default

Everything ships with metrics, logs, and traces. Observability is not an add-on; a component that cannot be observed is not complete. No special-casing required for a component to be seen.

## 5. Failure is a feature

Incidents are study material, not defects to hide. ForgeLab's value is in breaking things deliberately, safely, and reproducibly. Reliability work (recovery, DR, retries) is a first-class scenario, not an afterthought.

## 6. Infrastructure as Code

Everything is declared, versioned, and reproducible. No manual, ad-hoc configuration is allowed in an environment. If it isn't in code, it doesn't exist.

## 7. Cloud agnostic

Local-first: the same manifest runs on a laptop and in the cloud. Cloud presets (AWS, GCP) are opt-in layers, never the baseline. Avoid vendor lock-in and provider-specific assumptions in the core.

## 8. Documentation first

Architecture and decisions precede implementation. A component may not be built before it is documented in the catalog and authorized by a roadmap phase. Docs and code ship together.

## 9. Physical ≠ Virtual

ForgeLab's physical resources (the user's actual hardware) are distinct from its virtual production resources (the simulated environment it presents). The two are never conflated: virtual capacity is an artifact of the simulator, not a claim about the host.

## 10. Never fake behavior, only virtualize capacity

The simulator scales how much capacity the production view *displays* — nodes, RAM, RPS, replicas — but never fakes how a real system would *behave*. Bottlenecks, latency, backpressure, and exhaustion occur for the same reasons they would in real production, at the scale the view claims.

**Scope:** this principle governs **Live mode** — every surface backed by real containers, processes, or cloud resources. **Sandbox mode** has no physical layer and is governed by Principle 15 instead (ADR-0013). The Sandbox is never precedent for faking a Live-mode value.

## 11. Preserve bottlenecks and system dynamics

Scaling the displayed capacity must not erase the system dynamics that make production interesting. If the database is the bottleneck at ×1, it remains the bottleneck at ×128. The scale factor changes capacity, not the shape of the system.

## 12. Always expose both physical and virtual metrics

Every infrastructure surface shows both host metrics and virtual production metrics, with the current simulation scale (e.g. ×64, ×128) always visible. Virtual values are labeled and never presented as real hardware measurements.

In Sandbox mode there is no host footprint to show; instead every value carries a *simulated* label (Principle 15).

## 13. Optimize for system behavior, not business functionality

ForgeLab's goal is not to recreate Amazon, Uber, a banking platform, or a travel platform. It is to reproduce the **engineering characteristics** of such systems: read-heavy, write-heavy, CPU-heavy, memory-heavy, DB-heavy, cache-heavy, network-heavy, message-heavy, concurrency-heavy, latency-sensitive, failure-prone, highly distributed. Reproducing those characteristics lets ForgeLab generate thousands of different production-system configurations without requiring thousands of custom applications.

## 14. Business labels are visualization; operations are the model

Business-domain terminology ("Checkout", "Search Flights", "Transfer Money") is a feature label used for visualization. The actual simulation model is the underlying **operational workload** — the HTTP requests, database reads/writes, cache operations, messaging, latency, and concurrency. A feature is not behavior; behavior is the operations. The same workload may be presented under any feature label without changing the system.

## 15. Sandbox values are modelled, never scripted, and always labelled

Sandbox mode is a model of a production system, not a recording of one. Every Sandbox value — latency, errors, RPS, users, cost, satisfaction — is derived each tick from declared capacities, service times, and the player's topology, never from scripted curves or random metric values. Randomness is limited to seeded events (a surge, an outage); how the system responds to them is computed. Every Sandbox surface is labelled as simulated, and the formulas are documented so a learner can check the model against Live mode.

---

## Derived rules

* **Deterministic core** — simulation logic runs identically every time and is testable without a UI (see coding philosophy in `AGENTS.md`). The scale factor is deterministic for a given host and manifest.
* **Behavior over features** — synthetic applications model operations (requests, DB/cache/messaging, latency, concurrency), never business logic; feature labels are a presentation concern.
* **Clean boundaries** — application, runtime, and dashboard stay separable; dependencies flow one way.
* **No secrets** — never log, store, or commit passwords, tokens, or keys.
* **Explicit before magical** — plain, reviewable code beats framework cleverness.