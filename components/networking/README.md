# Networking Components

Traffic entry and internal topology: load balancing, proxying, gateway policies, and service discovery.

**Status:** partially implemented — Reverse Proxy ships in Phase 1; the rest are cataloged.

## Components

| Component | Provides | Status |
|---|---|---|
| Load Balancer | Traffic distribution, health-based routing | planned |
| Reverse Proxy | TLS, routing, buffering | implemented — see [reverse-proxy/](reverse-proxy/) |
| API Gateway | Authn/z, rate limiting, shaping | planned |
| Service Discovery | Automatic peer resolution | planned |

Any component here is optional; a manifest that omits networking routes nothing.