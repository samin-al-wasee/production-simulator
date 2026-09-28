# ADR-0011: Security controls, verification, and defensive attack drills (Phase 8)

**Status:** accepted
**Date:** 2026-09-28

## Context

Phase 8 adds attack simulation scenarios, secrets management, TLS, RBAC, and compliance-oriented checks. The lab must demonstrate controls that actually work, keep credentials out of git and out of container environments, and keep every drill defensive: attack simulations only target the lab's own localhost containers and clusters, and never scan or attack third-party systems.

## Decision

1. **TLS and hardened edge (Compose):** `compose.security.yaml` (`make up-secure`) terminates TLS 1.2/1.3 on `:8443` with a locally generated certificate authority (`scripts/gen-dev-certs.sh`, 30-day validity, git-ignored, private keys mode 600), redirects HTTP, sets HSTS and other security headers, rate-limits per client (429), hides the server version, and returns 404 for dotfiles and `/metrics`.
2. **Secrets management:** the database password is generated locally (`scripts/gen-secrets.sh`, git-ignored, mode 600) and supplied as a Docker secret file (`POSTGRES_PASSWORD_FILE`), so it never appears in the container environment or `docker inspect`. On Kubernetes, the database Secret and the TLS Secret are created by `scripts/k8s-up.sh` outside the manifests. On cloud presets (ADR-0009) RDS manages its password in Secrets Manager and Cloud SQL uses IAM authentication.
3. **Kubernetes hardening:** the `forgelab` namespace enforces the `restricted` Pod Security Standard; workloads run as non-root with all capabilities dropped, read-only root filesystems, seccomp `RuntimeDefault`, and no service account token; the application image itself now runs as an unprivileged user. Default-deny ingress and egress NetworkPolicies plus explicit allows (ingress controller → web → postgres, DNS) segment east-west traffic. The ingress serves TLS on host port 8444 with the local certificate as the controller's default certificate; plain HTTP stays on 8081 so the existing drills keep working.
4. **RBAC:** two namespaced roles for groups, `forgelab-viewer` (read-only) and `forgelab-operator` (act on deployments and delete pods); neither can read Secrets, exec into pods, or change RBAC, and the application service account has no API access.
5. **Compliance-oriented checks:** `core/internal/compliance` and `forgelab security compliance` evaluate Kubernetes manifests (rendered with `kubectl kustomize`) and Compose files against a fixed control catalog (`K8S-001`…`K8S-013`, `COMP-001`…`COMP-005`). Findings carry a severity and informational references to public frameworks (CIS, NIST 800-53); these are guidance, not a certification. Exceptions are declared next to the object (a `forgelab.io/compliance-exempt` annotation or `x-forgelab-exempt` in Compose) with a mandatory reason and are reported, never hidden.
6. **Secret scanning:** `core/internal/secretscan` and `forgelab security scan-secrets` detect private keys, cloud and platform tokens, and credential assignments, print only redacted excerpts, and honor an allowlist (`security/secretscan.yaml`) in which every entry states why it is safe.
7. **Verification, not assertion:** `scripts/security-smoke.sh` (Compose) and `scripts/k8s-security-smoke.sh` (Kubernetes) test the controls, including a positive control for network policy (an allow-listed pod must connect, otherwise the block test proves nothing). `make compliance` runs the compliance and secret checks together.
8. **Attack simulation scenarios** are defensive drills under `scenarios/security/`, each mapping to a smoke-test section: brute force and probing, TLS downgrade, secrets exposure, RBAC privilege escalation, network segmentation breach, and privileged workload admission.

## Consequences

### Positive

- Each control is verified by an executable check, including one against real Kubernetes network policy enforcement.
- Credentials live in git-ignored files or Secrets, never in manifests or container environments.
- Exceptions are explicit and auditable.

### Negative / Trade-offs

- The local CA is self-signed and only suitable for this machine; browsers will warn unless the CA is trusted manually.
- The security overlay and the observability overlay should not be combined where the postgres-exporter DSN matters, because the exporter still receives credentials through the environment.
- The base Kubernetes ingress keeps plain HTTP available on 8081 (no forced redirect) so drills can use curl without a CA.
- Pattern-based secret scanning has false positives and false negatives; it complements, and does not replace, provider-side secret scanning.
- The compliance controls are a small, curated subset; passing them does not imply conformance with any framework.
- Image and vulnerability scanning are not included.

## Alternatives Considered

- **Vault / SOPS / external secret operators** — appropriate for shared environments; the local lab needs no additional service, and cloud presets already use provider secret stores.
- **cert-manager and a private CA** — heavier than needed for a local lab.
- **Calico/Cilium for policies** — kind's default CNI enforced the NetworkPolicies (verified), so no extra CNI is needed.
- **OPA/Gatekeeper or Kyverno for admission and compliance** — good in clusters; here the same controls are checked offline in Go and Pod Security admission covers the runtime side.
- **Third-party vulnerability scanners (Trivy, Grype)** — deferred to avoid external tool dependencies (AGENTS.md §13).

## References

- `ROADMAP.md` Phase 8 — Security
- `docs/security.md`
- `docs/decisions/0007-kubernetes-environment.md`
- `docs/decisions/0009-cloud-presets-and-cost-guard.md`
