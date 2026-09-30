# Scanner

Committed-secret detection for the repository.

**Status:** implemented (Phase 8, re-scoped by ADR-0014)

## Purpose

Keep passwords, tokens, and keys out of the repository.

## Provided

- `forgelab security scan-secrets [dir]` (`core/internal/secretscan`): private keys, cloud and platform tokens, and credential assignments, with redacted output.
- `make scan-secrets`, part of `make check`.

## Dependencies

- None (pure Go).

## Configuration

Allowlist in `security/secretscan.yaml`; every entry must give a reason.
