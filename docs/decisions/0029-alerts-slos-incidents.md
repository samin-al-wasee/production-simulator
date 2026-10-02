# ADR-0029: Alerts, SLOs, and an incident timeline

**Status:** accepted
**Date:** 2026-10-02

## Context

With telemetry configured ([ADR-0027](0027-configured-telemetry.md)) and failures explained ([ADR-0028](0028-explaining-failures.md)), the player can look at the system. Production teams do not watch dashboards all day: they decide in advance what must wake them (alerts), what reliability they promise (SLOs and error budgets), and afterwards review what happened in order (the incident timeline). The roadmap's last slice adds these.

## Decision

1. **A `monitor` command** replaces the game's monitoring, logged and replayed like every command, from `sandbox/v14`. It holds up to 20 alert rules and 20 SLOs, validated together.
2. **Alert rules:** a name, a component (or the system), a metric (rps, errors, error rate, latency, utilization, p95, health), `>` or `<` a threshold, held for 1 to 288 ticks. After every tick each rule is:
   * **no data** when the metric is not observed: a component whose metrics no store keeps, or the system while no front component is monitored;
   * **ok** when the condition does not hold;
   * **pending** while it has held for fewer ticks than required;
   * **firing** once it has held long enough.

   Firing and resolving are recorded (the last 50).
3. **SLOs:** a name, a component or the system, an availability target (50% to under 100%), and a window (1 hour to 7 days). Over the window's observed ticks, availability is `1 − failed ÷ requests`, the budget left is `1 − failed ÷ ((1 − target) × requests)`, and the burn rate is the last hour's failure share over `1 − target` (1 spends the budget exactly over the window). The share of the window observed is reported: ticks without data are not counted.
4. **The incident timeline** lines up events starting and ending, alerts firing and resolving, and the player's commands (moves aside), in tick order.
5. **Reported** by the API as `monitoring`, `alerts`, `alertLog`, `slos`, and `timeline`. Alerts and SLOs read what was observed; they change nothing in the simulation, so v14 keeps them, and a game without a `monitor` command replays as before.
6. **The dashboard** adds Alerts, SLOs, and Timeline to the Observe panel, with forms to add and remove rules and SLOs, and a *firing* badge on the Observe button.

## Consequences

### Positive

* Alerting on what is not monitored visibly does nothing, which is the lesson of every missed page.
* Error budgets turn reliability targets into numbers the player spends.
* The timeline supports a postmortem: what happened, when it was noticed, and what was done.

### Negative / Trade-offs

* Rules evaluate once per tick (five simulated minutes); faster alerting is not modelled.
* Notifications, routing, and silences are not modelled.

## Alternatives Considered

* **Alerts on the model's true values** — they would see what the player cannot, against ADR-0027.
* **A new ruleset** — monitoring changes no outcome; the new command only appears in saves that use it.

## References

* [ADR-0027](0027-configured-telemetry.md), [ADR-0028](0028-explaining-failures.md), [`docs/architecture.md`](../architecture.md#alerts-slos-and-incidents)
* `backend/internal/sandbox/alerts.go`, `alerts_test.go`; `templates/postmortem/`
