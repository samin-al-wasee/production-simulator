# brute-force-and-probing

Category: security · Status: implemented

**Defensive drill.** Run it only against your own local lab (localhost). Never point these commands at systems you do not own.

## Goal

See how the hardened edge responds to request floods and to scanners probing for sensitive paths.

## Components involved

Reverse Proxy (nginx with rate limiting and blocked paths), TLS / PKI. Setup: `sh scripts/gen-dev-certs.sh && sh scripts/gen-secrets.sh && make up-secure`.

## Expected symptoms

- A burst of rapid requests receives `429` after the burst allowance (10 requests/second per client, burst 20).
- `/.env`, `/.git/config`, and `/metrics` return `404`, revealing nothing about what exists.
- Legitimate traffic at a normal rate is unaffected.

## Investigation

```sh
for i in $(seq 1 80); do curl -s --cacert environments/local/security/certs/ca.crt -o /dev/null -w '%{http_code}\n' https://localhost:8443/version; done | sort | uniq -c
curl -s --cacert environments/local/security/certs/ca.crt -o /dev/null -w '%{http_code}\n' https://localhost:8443/.env
docker logs forgelab-proxy --tail 20        # rejected requests are logged by nginx
make smoke-security                          # scripted checks
```

## Success criteria

- You can show the proportion of 429s and explain the rate and burst settings.
- You can explain why `404` (not `403`) is used for probing paths.
