# Architecture decision records

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-modular-monolith.md) | Modular monolith with API and worker processes | accepted |
| [0002](0002-postgres-job-queue.md) | PostgreSQL-backed job queue for MVP | accepted |
| [0003](0003-postgres-and-rustfs.md) | Relational metadata in PostgreSQL, binaries in RustFS | accepted |
| [0004](0004-server-time-reading-accounting.md) | Server-time, idempotent reading accounting | accepted |

ADRs document target decisions, not implementation completion. A later incompatible decision adds a new ADR and marks the old one superseded; do not silently rewrite historical context beyond typo/clarity fixes.
