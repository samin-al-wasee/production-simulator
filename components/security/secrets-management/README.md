# Secrets Management

Keep credentials out of git and out of container environments.

**Status:** implemented (Phase 8)

## Purpose

Keep credentials out of git and out of container environments.

## Provided

- Local: `scripts/gen-secrets.sh` writes a random database password to a git-ignored, mode 600 file; `compose.security.yaml` mounts it as a Docker secret (`POSTGRES_PASSWORD_FILE`).
- Kubernetes: `scripts/k8s-up.sh` creates the database and TLS Secrets outside the manifests.
- Cloud: RDS-managed master password in Secrets Manager; Cloud SQL IAM authentication (no password).
- Detection: `forgelab security scan-secrets` (see Scanner).

## Dependencies

- Compute (Docker or Kubernetes)

## Configuration

Rotation: delete the generated file and recreate the stack (the database password is read only when the data volume is first initialised).
