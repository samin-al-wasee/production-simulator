# db-outage

Category: failures · Status: implemented

## Goal

Observe how the application behaves when its database disappears, and confirm it recovers by itself.

## Components involved

PostgreSQL (deliberately stopped), sample-web, reverse proxy, Chaos Engine. Setup: `make up`.

## Expected symptoms

- `/healthz` returns 503 with `"db":"unreachable"`; the application and proxy stay up.
- After the database restarts, `/healthz` returns 200 again without restarting the application.
- With `make up-obs`: the `DatabaseDown` alert fires and `pg_up` drops to 0.

## Investigation

```sh
make chaos FILE=scenarios/failures/db-outage/experiment.yaml   # scripted: probes before, during, after
make chaos FILE=scenarios/failures/db-outage/experiment.yaml DRY=1  # only print the plan
docker logs forgelab-app --tail 20                         # access logs show 503s
```

## Success criteria

- The experiment reports PASSED (hypothesis held and the system recovered).
- You can explain why the app stayed reachable while reporting degraded, and where a real application would need timeouts and pool recovery.
