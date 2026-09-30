# ForgeLab Principles

**Document status:** v2.0 (re-scoped to the Production Sandbox by [ADR-0014](decisions/0014-sandbox-only-platform.md))

These principles guide every decision in ForgeLab. If a proposal conflicts with a principle, the principle wins unless a documented decision (ADR) intentionally overrides it.

## 1. Modelled, never scripted, always labelled

ForgeLab is a model of a production system, not a recording of one. Every value it shows (latency, errors, RPS, users, cost, satisfaction) is derived each tick from declared capacities, service times, and the player's topology, never from scripted curves or random metric values. Randomness is limited to seeded events such as a surge or an outage; how the system responds to them is computed. Every surface is labelled as simulated.

The one declared input is a **load test** ([ADR-0016](decisions/0016-configurable-internet-traffic.md)): the player may set the Internet's request rate and its pattern (constant, ramp, spike, burst, periodic wave, daily schedule) instead of letting users drive it. The rate is an input the player chose, labelled as a load test. Everything the system does under it is still computed, and a load test earns nothing and moves no user or goal.

## 2. Bottlenecks come from the design

A bottleneck exists because of what the player built: too few instances, a missing cache, a queue without workers. The same design under the same load always has the same bottleneck, and fixing it moves the limit somewhere else, as it would in production. No value is tuned to make a design look better or worse than the model says it is.

## 3. Formulas are explainable

The model uses simple, documented formulas (see `docs/architecture.md`) that a learner can check by hand: utilization is load over capacity, latency rises as utilization approaches one, excess load is dropped. A formula that cannot be explained in a sentence does not belong in the model. Behavior changes ship with tests that pin the claim they make.

## 4. The player composes everything

A new game is an empty world. Nothing is implied: no default database, no free load balancer. Every component is placed and wired by the player, and a system with no path from the Internet to an application instance serves no one.

## 5. Failure is a feature

Saturation, outages, incidents, and bankruptcy are study material, not defects to hide. The game should make it possible to break things, see why, and recover.

## 6. Reproducible by construction

A game is fully defined by its ruleset version, seed, and ordered command log. Replaying the same three always produces the same world at every tick. Tuning lives in versioned ruleset data, so a change to the model never silently changes an old save.

## 7. Behavior over business functionality

ForgeLab reproduces the **engineering characteristics** of production systems (read-heavy, write-heavy, latency-sensitive, failure-prone), not any company's product. The economy is generic: revenue per successful request, cost per component-hour. Business terms, if ever added, are presentation labels on top of the same model.

## 8. Headless core, thin dashboard

All simulation logic lives in the Go core and runs without a UI. The dashboard renders state and sends commands; it never computes a simulated value.

## 9. Documentation first

Architecture and decisions precede implementation. A feature is built only when its roadmap phase authorizes it, and docs and code ship together.

---

## Derived rules

* **Deterministic core** — simulation logic runs identically every time and is testable without a UI (see coding philosophy in `AGENTS.md`).
* **Verify in the browser** — dashboard changes are checked by playing them (`make test-e2e`), not only by unit tests.
* **Clean boundaries** — engine, API, and dashboard stay separable; dependencies flow one way.
* **No secrets** — never log, store, or commit passwords, tokens, or keys.
* **Explicit before magical** — plain, reviewable code beats framework cleverness.
