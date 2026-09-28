# secrets-exposure

Category: security · Status: implemented

**Defensive drill.** Run it only against your own local lab (localhost). Never point these commands at systems you do not own.

## Goal

Prove credentials are not exposed through the repository, container environments, or inspection commands, and see how a leak would be caught.

## Components involved

Secrets Management, Scanner. Setup: `sh scripts/gen-secrets.sh && make up-secure`.

## Expected symptoms

- `docker inspect forgelab-db` shows `POSTGRES_PASSWORD_FILE` but no password value.
- `forgelab security scan-secrets` reports no secrets on the repository.
- Adding a file with a fake credential makes the scan fail with a redacted excerpt.

## Investigation

```sh
docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' forgelab-db | grep -i postgres
make scan-secrets
echo 'password: not-a-real-secret-123' > /tmp/leak.yaml && cp /tmp/leak.yaml security/leak-test.yaml
make scan-secrets      # fails; then remove security/leak-test.yaml
```

## Success criteria

- You can show where the secret lives and who can read it (file mode 600).
- The scan catches the planted credential and never prints it in full.
- You can explain why a credential that reached git history must be rotated, not just deleted.
