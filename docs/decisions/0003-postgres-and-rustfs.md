# ADR 0003: PostgreSQL metadata and RustFS objects

- **Status:** accepted
- **Date:** 2026-07-15

## Context

Book originals, covers, images and potentially large chapter HTML do not belong in relational rows, while ownership, constraints, search and transactions do.

## Decision

Keep relational/source-of-truth metadata in PostgreSQL and binary/large immutable content in S3-compatible RustFS via AWS SDK for Go v2. Use purpose buckets and server-generated opaque/versioned keys. PostgreSQL records trusted key, size, checksum, type, version and availability state. Use explicit staging/activation/reconciliation because no distributed transaction spans PostgreSQL and S3.

## Consequences

Both systems need coordinated backups and restore verification. Object writes/deletes must be idempotent and repairable. Browser access uses ownership-checked, short-lived presigned URLs and a separate public endpoint. Local single-node RustFS persistence is not production redundancy.
