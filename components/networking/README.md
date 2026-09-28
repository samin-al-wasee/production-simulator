# Networking Components

Traffic entry and internal topology: load balancing, proxying, gateway policies, and service discovery.

**Status:** partially implemented — Reverse Proxy (Phase 1) and Load Balancer (Phase 4) ship; the rest are cataloged.

## Components

| Component | Provides | Status |
|---|---|---|
| Load Balancer | Traffic distribution, health-based routing | implemented — see [load-balancer/](load-balancer/) |
| Reverse Proxy | TLS, routing, buffering | implemented — see [reverse-proxy/](reverse-proxy/) |
| API Gateway | Authn/z, rate limiting, shaping | planned |
| Service Discovery | Automatic peer resolution | planned |

Any component here is optional; a manifest that omits networking routes nothing.