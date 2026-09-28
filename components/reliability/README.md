# Reliability Components

Failure as a feature: chaos injection, load testing, resiliency patterns, and backup/restore.

**Status:** partially implemented — chaos, load generation, and backup/restore ship in Phase 5 (ADR-0008); the Resiliency Toolkit is planned (its retry/dead-letter model lives in `core/internal/retry`).

## Components

| Component | Provides | Status |
|---|---|---|
| Chaos Engine | Fault injection (kill, latency, packet loss, stress) | implemented — see [chaos-engine/](chaos-engine/) |
| Load Generator | Repeatable traffic profiles and benchmarks | implemented — see [load-generator/](load-generator/) |
| Resiliency Toolkit | Retries, timeouts, circuit breakers, backpressure | planned |
| Backup / Restore | Scheduled snapshots and DR restore drills | implemented — see [backup-restore/](backup-restore/) |

This is where ForgeLab earns its name: incidents are study material, not defects to hide.