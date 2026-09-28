# TLS / PKI

Encrypted transport for the local and Kubernetes environments.

**Status:** implemented (Phase 8)

## Purpose

Encrypted transport for the local and Kubernetes environments.

## Provided

- `scripts/gen-dev-certs.sh`: local CA and a `localhost` server certificate (SAN localhost, 127.0.0.1), 30-day validity, private keys mode 600, git-ignored.
- Compose: nginx TLS 1.2/1.3 on `:8443`, HTTP redirect, HSTS and security headers (`compose.security.yaml`, `make up-secure`).
- Kubernetes: ingress TLS on host port 8444 using the `forgelab-tls` Secret as the controller default certificate.

## Dependencies

- Reverse Proxy or ingress controller

## Configuration

Trust `environments/local/security/certs/ca.crt` for local testing only. Verify with `make smoke-security` and `scripts/k8s-security-smoke.sh`.
