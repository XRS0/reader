# ADR 0002: PostgreSQL-backed job queue

- **Status:** accepted
- **Date:** 2026-07-15

## Context

Uploads, stale sessions, aggregates, cleanup, translation and exports need durable asynchronous work. The MVP already requires PostgreSQL; Kafka/RabbitMQ/NATS would add infrastructure and a cross-system transaction problem.

## Decision

Store typed/versioned jobs in PostgreSQL. Claim runnable rows with a short transaction and `FOR UPDATE SKIP LOCKED`; process outside the claim transaction under a renewable lease. Handlers are idempotent. Track attempts, maximum attempts, timeout, priority, `run_after`, owner/lease, bounded safe error, completion and dead-letter state. Use exponential backoff with jitter.

## Consequences

Enqueue can be atomic with domain changes and operational burden stays low. PostgreSQL receives queue load, so polling/backoff/indexes/batches must be measured. Long work must not hold row transactions. A dedicated broker is reconsidered only when measured throughput/isolation/fan-out requirements exceed this design.
