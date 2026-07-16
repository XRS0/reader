# ADR 0001: modular monolith

- **Status:** accepted
- **Date:** 2026-07-15

## Context

BookFlow has strongly related transactional domains and a small initial operating team. Microservices would add distributed consistency, deployment and observability cost before independent scale boundaries are known.

## Decision

Use one Go module and PostgreSQL schema organized into explicit auth/users/devices, library/books/processing, reader/progress/sessions/statistics, translation/dictionary, annotation/preferences, storage/jobs/observability modules. Deploy stateless API and worker binaries from the same codebase. Keep Gin in transport and infrastructure behind practical ports.

## Consequences

Cross-domain transactions and local development stay simple. Module discipline must be enforced in code review because the compiler cannot prevent every boundary violation. API/worker can scale independently; extraction is considered only from measured load/ownership/deployment needs, with processing or translation the likely first candidates.
