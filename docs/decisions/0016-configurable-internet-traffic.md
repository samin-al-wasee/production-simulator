# ADR-0016: A configurable Internet: traffic groups, request mix, and load tests

**Status:** accepted
**Date:** 2026-09-30

## Context

Phase 11 (Deep component simulation) makes each component progressively more realistic, one at a time, starting with the Internet. Until now the Internet was a fixed traffic source with no settings:
* **Volume:** `active users × engagement × diurnal(t) × traffic events`, driven by the economy (growth, churn, satisfaction).
* **Request mix:** fixed ruleset shares. 80% of requests at an application were reads, 20% writes, and 10% also needed object storage. A CDN answered 30% of all requests, writes included.

Learners could not ask the questions production engineers ask about traffic:
* who sends it
* what they request
* where it comes from
* how it changes over time
* what client retries do to a struggling system

The project owner asked for the Internet to stay **one component** on the canvas, with this richness inside it. They also asked for the player to be able to set the volume directly: a rate and a pattern such as a spike, a ramp, or a daily schedule. That conflicts with Principle 1, under which traffic is never a scripted curve, and with the economy, in which traffic follows how well the player runs the system. The owner chose to keep both, switched per game.

## Decision

1. **The Internet has a configuration.** It is a `TrafficConfig` stored on the Internet node and replaced whole by a new `configure` command. Like every command it is validated, logged, and replayed. The configuration holds:
   * a **source**
   * a **pattern**
   * **endpoints** (method, path, cacheable, storage)
   * **traffic groups**, each with a share of the volume, its own endpoint mix and region mix, and a retry count

   Groups, endpoints, and regions are not canvas nodes.
2. **Two sources.**
   * `market` (the default) keeps the existing user model for volume.
   * `configured` is a **load test**: the player sets requests per second and a pattern, and event multipliers still apply. The patterns are:
     * `constant`
     * `ramp` (gradual increase or decrease)
     * `spike`
     * `burst` (a square wave)
     * `periodic` (a cosine wave)
     * `schedule` (step values by hour of the day)

   A load test earns no revenue, holds users, satisfaction, and popularity still, checks no goals, and resets goal hold streaks. Events judged after running into a load test do not count towards goals. Costs and events continue.
3. **The request mix is modelled by class.** The flow solver carries load as a small vector over three request classes (cacheable read, read, write), plus the part of each class that also fetches from object storage. This is the same way it already carried attack traffic alongside real traffic.
   * A CDN answers only cacheable reads.
   * An application routes reads and writes separately.
   * Success and latency are solved per class and combined at the Internet.

   This is the traffic-transformation contract later components (CDN, WAF, gateway, balancer) build on. A component receives a class vector and emits one.
4. **Retries are modelled in expectation.** A group with `N` retries sends `load × Σ_{k=0..N} f^k` attempts, where `f` is last tick's attempt failure rate for the class. A request fails only if all `N + 1` attempts fail. Using last tick's rate makes retry storms emerge over ticks and keeps the model deterministic.
5. **Concurrency is an output, not an input.** Requests in flight = attempts per second × mean latency (Little's law). The player sets rates, not users, for a load test.
6. **Regions are reported, not yet modelled.** Traffic is broken down by abstract region (ruleset data) so later phases can add regional latency, CDN placement, and regional failures without changing the configuration schema.
7. **No per-request randomness.** Traffic is an expected-value model; the seed still drives only the Event Deck.
8. **Ruleset `sandbox/v4`.** It adds the default configuration (four groups, six endpoints, four of six regions), `MaxRetries` 3, `MaxTrafficRPS` 1,000,000, and a CDN hit ratio of 45% on cacheable reads. That keeps the default mix's edge share near v3's 30%. New games use v4.
   * Rulesets v1 to v3 have no traffic configuration, reject `configure`, and route with their fixed shares.
   * The class-based solver reproduces their results to floating-point rounding (relative difference below 1e-13 over four simulated days), so their saves replay unchanged.
9. **Validation lives in the engine and corrects nothing.**
   * Shares must sum to 100%, within 1e-6.
   * Names must be unique and non-blank; endpoints and regions must be known.
   * Rates must be within the bounds; retries must be within the limit.
   * Pattern timings must be observable at the five-minute tick.
   * The pattern is checked only when the source is `configured`.

   Every problem is returned at once, and the dashboard lists them.
10. **Principle 1 is amended.** The model never generates values from curves or random numbers. A player-declared load test is an input the player chose, labelled as a load test, and the system's response to it is still computed.

## Consequences

### Positive

* Learners can shape traffic (read-heavy, write-heavy, cacheable, storage-heavy, bursty) and see the bottleneck move, as Principle 2 requires.
* Load tests teach capacity planning directly: "what breaks at 5,000 RPS?" becomes a one-minute experiment.
* Retry storms and the value of retries for transient failures are both reproducible.
* The class vector and the `Traffic` breakdown give later components a contract to transform traffic, and give monitoring a data source.

### Negative / Trade-offs

* The solver does three times the per-node work of v3; it remains linear in nodes × edges.
* Load tests pause the business game. That is intentional, but a player can use a load test to wait out a bad day without losing users. Costs still run, and goals do not advance.
* Attempts are treated as independent within a request class. When only some requests of a class always fail (for example the storage-needing reads when no object storage is wired), retries appear to recover part of them. Failure rates are tracked per class, not per endpoint, to keep the solver small.

## Alternatives Considered

* **Market only** — keeps Principle 1 untouched, but drops the owner's requirement to set RPS and patterns directly.
* **Configured only** — replaces the economy's traffic with player-set volume, and breaks growth, churn, goals, and the learning path.
* **Separate canvas nodes for users, bots, regions** — rejected by the product rule that one canvas component is one infrastructure concept.
* **Per-request sampling with the seed** — adds noise without teaching anything that the expected-value model does not, and makes formulas harder to check by hand (Principle 3).
* **Keeping the scalar solver and scaling shares** — cannot express a CDN that answers only cacheable reads, or an application seeing fewer reads behind a CDN.

## References

* [ADR-0013](0013-production-sandbox-game.md), [ADR-0014](0014-sandbox-only-platform.md)
* [`docs/architecture.md`](../architecture.md#traffic-model), [`docs/principles.md`](../principles.md)
* `backend/internal/sandbox/traffic.go`, `flow.go`, `traffic_test.go`
