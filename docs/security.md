# ForgeLab Security

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

ForgeLab runs no real services, so its security concern is the repository and the local tools it ships. The Live-mode security controls (TLS overlay, Kubernetes hardening, compliance checks, attack drills) were retired by ADR-0014; they are on the `archive/live-lab` branch.

## Practice

* **No secrets in the repository.** Never commit, log, or store passwords, tokens, or keys (`AGENTS.md`).
* **Secret scan.** `make scan-secrets` (`forgelab security scan-secrets`) checks the repository for private keys, cloud and platform tokens, and credential assignments, with redacted output. It is part of `make check`. Known false positives are allowlisted in `security/secretscan.yaml`, each with a reason.
* **Local-only API.** `forgelab serve` listens on `127.0.0.1:8090` by default and sends no CORS headers unless `-allow-origin` is set. It holds games in memory and writes only to `.forgelab/` in the repository.
* **OAuth credentials.** Sign-in is OAuth-only, so no password is stored, hashed, or accepted. Client ids and secrets (`GITHUB_CLIENT_ID`/`GITHUB_CLIENT_SECRET`, `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`) come from the environment and are never committed; a provider without credentials is not offered.
* **Sessions.** Session tokens are 256 random bits; only their SHA-256 hash is stored, so a database leak does not yield a usable token. The raw token travels only in an `HttpOnly`, `Secure`, `SameSite=Lax` cookie, and logout deletes the row so a session is revocable server-side. Handlers read the user id from the session, never from the request.
* **Dependencies.** New dependencies need the owner's approval (`AGENTS.md` §13).

## Limits

* The secret scan is pattern-based; it can miss secrets in unusual formats.
* There is no image, dependency, or CVE scanning.
