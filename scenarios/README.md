# Scenarios

Reproducible drills — traffic, failures, security, scaling, and performance — that make production conditions inspectable. Each scenario folder follows the template in `docs/scenarios.md`. Implemented scenarios are marked in the catalog.

| Category | Purpose |
|---|---|
| `traffic/` | Load profiles and traffic spikes |
| `failures/` | Outages, slowdowns, and deployment drills |
| `security/` | Attack simulation and defense |
| `scaling/` | Capacity and autoscaling behavior |
| `performance/` | Load testing and benchmarks |

## Safety rule

Scenarios are declared, reproducible, and safe. Never run an irreversible or destructive drill on a shared environment without explicit approval.