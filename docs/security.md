# ForgeLab Security

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

ForgeLab runs no real services, so its security concern is the repository and the local tools it ships. The Live-mode security controls (TLS overlay, Kubernetes hardening, compliance checks, attack drills) were retired by ADR-0014; they are on the `archive/live-lab` branch.

## Practice

* **No secrets in the repository.** Never commit, log, or store passwords, tokens, or keys (`AGENTS.md`).
* **Secret scan.** `make scan-secrets` (`forgelab security scan-secrets`) checks the repository for private keys, cloud and platform tokens, and credential assignments, with redacted output. It is part of `make check`. Known false positives are allowlisted in `security/secretscan.yaml`, each with a reason.
* **Local-only API.** `forgelab serve` listens on `127.0.0.1:8090` by default and sends no CORS headers unless `-allow-origin` is set. It holds games in memory and writes only to `.forgelab/` in the repository. With a store it writes saved games and learning progress to Postgres, and the dashboard reaches the API same-origin through its own server, so the session cookie stays first-party (ADR-0033).
* **OAuth credentials.** Sign-in is OAuth-only, so no password is stored, hashed, or accepted. Client ids and secrets (`GITHUB_CLIENT_ID`/`GITHUB_CLIENT_SECRET`, `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`) come from the environment and are never committed; a provider without credentials is not offered.
* **Sessions.** Session tokens are 256 random bits; only their SHA-256 hash is stored, so a database leak does not yield a usable token. The raw token travels only in an `HttpOnly`, `Secure`, `SameSite=Lax` cookie, and logout deletes the row so a session is revocable server-side. Handlers read the user id from the session, never from the request.
* **Per-user isolation.** Saved sandboxes and learning progress are keyed by the session user id; every owned query filters by that id, and another user's id reads as a 404 rather than confirming it exists. A request without a session has no persistence in store mode, so nothing is written on behalf of no one.
* **Sign-in surface.** The dashboard proxies the OAuth endpoints at `/api/v1/auth/*`, forwarding the session cookie and every `Set-Cookie`, so the `HttpOnly`, `SameSite=Lax`, host-only session cookie is first-party on the dashboard origin; no `SameSite=None` and no CSRF token are needed for this surface (ADR-0033).
* **Test session seeder.** `backend/cmd/seed-session` creates a user and a session for the browser tests. It is a development tool: it refuses to run without `FORGELAB_TEST_SESSION=1`, and the deployment image builds only `./cmd/forgelab`, so it is never shipped.
* **Dependencies.** New dependencies need the owner's approval (`AGENTS.md` §13).

## Limits

* The secret scan is pattern-based; it can miss secrets in unusual formats.
* There is no image, dependency, or CVE scanning.
