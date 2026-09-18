# Database Components

State and persistence: relational storage, caches, and in-memory stores.

**Status:** cataloged in `docs/component-catalog.md` — nothing implemented yet.

## Planned components

| Component | Provides | Status |
|---|---|---|
| PostgreSQL | Relational storage, transactions, replication | planned |
| Redis | Cache, in-memory store, pub/sub, rate limiting | planned |

Every component here is optional. An application that needs no persistence uses none.