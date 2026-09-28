# Scanner

Compliance-oriented and secret scanning.

**Status:** implemented (Phase 8)

## Purpose

Compliance-oriented and secret scanning.

## Provided

- `forgelab security compliance <file|dir|->`: 18 controls over Kubernetes manifests and Compose files, severities, informational CIS/NIST references, and declared exemptions with reasons.
- `forgelab security scan-secrets [dir]`: private keys, cloud/platform tokens, credential assignments; redacted output; allowlist in `security/secretscan.yaml`.
- `make compliance` runs both against the repository and the rendered Kubernetes manifests.
- Not included: image and CVE scanning.

## Dependencies

- kubectl (only for rendering Kubernetes manifests)

## Configuration

`-fail-on low|medium|high` selects the failing severity (default medium).
