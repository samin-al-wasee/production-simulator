# kafka-outage

Category: failures · Status: implemented

## Goal

Confirm the platform detects a broker outage and that the broker rejoins cleanly.

## Components involved

Kafka (deliberately stopped), kafka-exporter, Prometheus, Chaos Engine. Setup: `make up-all`.

## Expected symptoms

- `up{job="kafka"}` in Prometheus becomes 0 within a scrape interval or two; `kafka_brokers` disappears.
- After the broker restarts, the target returns to 1 and consumer-group metrics resume.

## Investigation

```sh
make chaos FILE=scenarios/failures/kafka-outage/experiment.yaml
curl -s 'localhost:9090/api/v1/query?query=up%7Bjob%3D%22kafka%22%7D'
```

## Success criteria

- The experiment reports PASSED.
- You can explain why the `optional` label keeps `TargetDown` quiet and `MessagingTargetDown` covers this case.
