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

---

## Derived rules

* **Deterministic core** — simulation logic runs identically every time and is testable without a UI (see coding philosophy in `AGENTS.md`).
* **Clean boundaries** — application, runtime, and dashboard stay separable; dependencies flow one way.
* **No secrets** — never log, store, or commit passwords, tokens, or keys.
* **Explicit before magical** — plain, reviewable code beats framework cleverness.