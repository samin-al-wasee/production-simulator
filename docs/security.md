# ForgeLab Security

**Document status:** Baseline · v1.0

How ForgeLab's security controls are built, verified, and where their limits are (ADR-0011). Everything here is defensive and aimed at the lab's own localhost containers and clusters.

## Controls at a glance

| Area | Local (Compose) | Kubernetes | Cloud presets |
|---|---|---|---|
| Transport | nginx TLS 1.2/1.3 on :8443, redirect, HSTS | Ingress TLS on :8444 | Managed endpoints |
| Secrets | Docker secret file, mode 600, git-ignored | Secrets created outside manifests | RDS Secrets Manager, Cloud SQL IAM auth |
| Identity and access | — | RBAC roles for viewer and operator groups, no SA token | Provider IAM |
| Segmentation | Private bridge network | Default-deny NetworkPolicies | Private database subnets, no public DB |
| Workload hardening | Compliance checks | Pod Security `restricted`, non-root, no capabilities, read-only root, seccomp | Workload Identity |
| Edge protection | Rate limiting (429), blocked dotfiles and `/metrics`, security headers | — | — |

## Verification

| Command | Verifies |
|---|---|
| `make smoke-security` | TLS, redirect, TLS 1.1 rejection, headers, rate limiting, blocked paths, secret handling (needs `make up-secure`) |
| `sh scripts/k8s-security-smoke.sh` | Ingress TLS, Pod Security admission, network policy (with positive control), RBAC (needs `make k8s-up`) |
| `make compliance` | Compliance controls and secret scan over the repository and rendered manifests |
| `make scan-secrets` | Secret scan only |

## Compliance controls

Findings are informational guidance with references to public frameworks; they are not a certification. Exceptions are declared beside the object with a reason (`forgelab.io/compliance-exempt: "ID=reason"` on Kubernetes objects, `x-forgelab-exempt` on Compose services) and always appear in the report.

| ID | Control | Severity | Reference |
|---|---|---|---|
| K8S-001 | Containers must not be privileged | high | CIS Kubernetes 5.2.2; NIST 800-53 AC-6 |
| K8S-002 | Containers must run as non-root | high | CIS Kubernetes 5.2.6 |
| K8S-003 | Privilege escalation disabled | medium | CIS Kubernetes 5.2.5 |
| K8S-004 | Drop all capabilities | medium | CIS Kubernetes 5.2.7-5.2.9 |
| K8S-005 | Memory limit set | medium | NIST 800-53 SC-6 |
| K8S-006 | Read-only root filesystem | low | CIS Docker 5.12 |
| K8S-007 | Pinned, non-latest image tag | medium | CIS Kubernetes 5.5.1 |
| K8S-008 | No host namespaces or hostPath | high | CIS Kubernetes 5.2.3-5.2.4 |
| K8S-009 | Seccomp profile set | medium | CIS Kubernetes 5.7.2 |
| K8S-010 | Namespace enforces a Pod Security Standard | medium | Pod Security Admission |
| K8S-011 | Default-deny NetworkPolicy per namespace with workloads | high | CIS Kubernetes 5.3.2; NIST 800-53 SC-7 |
| K8S-012 | Service account token not automounted | medium | CIS Kubernetes 5.1.6 |
| K8S-013 | Ingress terminates TLS | high | NIST 800-53 SC-8 |
| COMP-001 | Compose service not privileged | high | CIS Docker 5.4 |
| COMP-002 | No Docker socket mount | high | CIS Docker 5.31 |
| COMP-003 | Pinned, non-latest image tag | medium | CIS Docker 4.7 |
| COMP-004 | No host network | high | CIS Docker 5.9 |
| COMP-005 | No literal credentials in environment | high | NIST 800-53 IA-5 |

## Known limits

- The local certificate authority is self-signed and only for this machine.
- The database password is read only when a data volume is first initialised.
- The security overlay and the observability overlay should not be combined where the postgres-exporter DSN matters.
- Secret scanning is pattern-based; compliance controls are a curated subset; no image or CVE scanning.
- The dashboard has no authentication and is meant for loopback use (ADR-0010).
