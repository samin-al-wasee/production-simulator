# Security Components

Hardening and defense: secrets, TLS, network policy, and scanning.

**Status:** implemented (Phase 8, ADR-0011). Image/CVE scanning is not included.

## Components

| Component | Provides | Status |
|---|---|---|
| Secrets Management | Store/rotate credentials without leaking | implemented — see [secrets-management/](secrets-management/) |
| TLS / PKI | Certificates for internal and external traffic | implemented — see [tls-pki/](tls-pki/) |
| Network Policy | East-west segmentation, RBAC | implemented — see [network-policy/](network-policy/) |
| Scanner | Vulnerability/image/compliance scanning | implemented — see [scanner/](scanner/) |

Every component here is optional and opt-in, matching the platform's no-implicit-component rule.