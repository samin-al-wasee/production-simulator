# Database Components

State and persistence: relational storage, caches, and in-memory stores.

**Status:** implemented — PostgreSQL (Phase 1) and Redis (Phase 3).

## Components

| Component | Provides | Status |
|---|---|---|
| PostgreSQL | Relational storage, transactions, replication | implemented — see [postgresql/](postgresql/) |
| Redis | Cache, in-memory store, pub/sub, rate limiting | implemented — see [redis/](redis/) |

Every component here is optional. An application that needs no persistence uses none.