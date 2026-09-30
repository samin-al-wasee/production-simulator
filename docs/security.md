# ForgeLab Security

**Document status:** v2.0 (re-scoped by [ADR-0014](decisions/0014-sandbox-only-platform.md))

ForgeLab runs no real services, so its security concern is the repository and the local tools it ships. The Live-mode security controls (TLS overlay, Kubernetes hardening, compliance checks, attack drills) were retired by ADR-0014; they are on the `archive/live-lab` branch.

## Practice

* **No secrets in the repository.** Never commit, log, or store passwords, tokens, or keys (`AGENTS.md`).
* **Secret scan.** `make scan-secrets` (`forgelab security scan-secrets`) checks the repository for private keys, cloud and platform tokens, and credential assignments, with redacted output. It is part of `make check`. Known false positives are allowlisted in `security/secretscan.yaml`, each with a reason.
* **Local-only API.** `forgelab serve` listens on `127.0.0.1:8090` by default and sends no CORS headers unless `-allow-origin` is set. It holds games in memory and writes only to `.forgelab/` in the repository.
* **Dependencies.** New dependencies need the owner's approval (`AGENTS.md` §13).

## Limits

* The secret scan is pattern-based; it can miss secrets in unusual formats.
* There is no image, dependency, or CVE scanning.
