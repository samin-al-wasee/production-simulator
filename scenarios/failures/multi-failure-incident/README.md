# multi-failure-incident

Category: failures · Status: implemented

## Goal

Practice incident response when two things fail at once under load, then turn the result into a reviewable postmortem. This is the stage 10 exercise of the learning path.

## Components involved

Reverse Proxy, sample-web, Redis (deliberately stopped), the application's network (300 ms latency injected), Load Generator, Chaos Engine, Prometheus and Grafana for observation, and the postmortem template. Setup: `make up-all`.

## Expected symptoms

- Under steady load, cache-backed requests (`/api/cache`) fail fast with 503 while Redis is down, and every response is about 600 ms slower while the latency fault is active.
- The 5xx ratio and p95 panels in Grafana move, and `HighErrorRate` and `HighLatencyP95` become pending or firing; `MessagingTargetDown` fires for `redis`.
- After both faults are reverted, error rate and latency return to baseline; the first cache reads after recovery are cold (`source: origin`).

## Investigation

```sh
# 1. Baseline: a benchmark before anything breaks
forgelab benchmark -url http://localhost:8080/api/work -rps 50 -step-duration 10s

# 2. Steady traffic in one terminal (cache-backed endpoint)
make loadtest URL="http://localhost:8080/api/cache?key=incident" RPS=50 DURATION=90s

# 3. In other terminals, inject the two faults while the load runs
make chaos FILE=scenarios/failures/redis-outage/experiment.yaml
make chaos FILE=scenarios/failures/network-latency/experiment.yaml

# 4. Diagnose from signals only: Grafana "ForgeLab Overview", Prometheus alerts,
#    `docker logs forgelab-app`, and the load test output.

# 5. After recovery, benchmark again and compare with step 1.
forgelab benchmark -url http://localhost:8080/api/work -rps 50 -step-duration 10s
```

Work out which symptom belongs to which fault before reading the experiment files.

## Success criteria

- You can name both faults and attribute each symptom to the right one using only signals.
- The second benchmark is back within its SLOs and matches the first within noise.
- You copied `templates/postmortem/postmortem.md`, filled in every section (impact with numbers, timeline, root cause, detection gap, action items with owners), and had it reviewed.
- `forgelab learn complete s10-multi-failure` and `s10-postmortem` once the above is true.
