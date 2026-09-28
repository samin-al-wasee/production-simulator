# network-latency

Category: failures · Status: implemented

## Goal

Measure the effect of added network latency on user-visible response time and confirm it clears when removed.

## Components involved

sample-web with a `tc netem` delay of 300 ms (Chaos Engine), reverse proxy, Load Generator. Setup: `make up`.

## Expected symptoms

- With the fault active, load-test latency rises by about 600 ms (the delay applies to the application's outgoing packets, and responses cross the proxy path) with no errors; after revert it returns to about a millisecond.

## Investigation

```sh
make loadtest URL=http://localhost:8080/api/work RPS=50 DURATION=5s      # baseline
make chaos FILE=scenarios/failures/network-latency/experiment.yaml &       # 25 s fault
sleep 12; make loadtest URL=http://localhost:8080/api/work RPS=50 DURATION=5s
```

## Success criteria

- Baseline p50 is around a millisecond; during the fault p50 is around 600 ms; after recovery it is back to baseline.
- You can explain why throughput dropped slightly at the same offered load (open-loop dispatch with in-flight limits).
