# load-test-baseline

Category: performance · Status: implemented

## Goal

Establish a repeatable latency and throughput baseline for the sample application before any change or fault.

## Components involved

sample-web behind the reverse proxy, Load Generator. Setup: `make up` (add `make up-obs` to watch Grafana while it runs).

## Expected symptoms

- Constant 100 RPS for 30 s shows a stable p50/p95/p99 with a 0% error rate and no dropped requests.
- A ramp to a higher rate shows where latency starts to climb or requests are dropped.

## Investigation

```sh
make loadtest URL=http://localhost:8080/api/work RPS=100 DURATION=30s
make loadtest URL=http://localhost:8080/api/work RPS=50 RAMP_TO=500 PROFILE=ramp DURATION=60s SCALE=128
```
Compare the physical and virtual RPS columns: virtual values are simulated capacity at the given scale factor, not measurements.

## Success criteria

- You recorded p50/p95/p99 and error rate for the baseline and can reproduce them within noise.
- You identified the offered rate at which latency or drops first degrade.
