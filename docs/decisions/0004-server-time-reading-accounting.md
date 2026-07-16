# ADR 0004: server-time reading accounting

- **Status:** accepted
- **Date:** 2026-07-15

## Context

Counting open-to-close wall time inflates reading when a tab is hidden or the user is idle. Client clocks and delivery can be skewed, replayed or interrupted. Multiple devices and API replicas create races.

## Decision

Use periodic configurable heartbeats with visibility, focus, recent-interaction, locator, sequence and idempotency key. Credit only a bounded interval derived from server receive times when eligibility and freshness rules pass. Lock/update session, event and idempotency state transactionally. Duplicate/out-of-order input earns no additional time. A worker finalizes stale sessions without credit after the last accepted heartbeat. Progress uses a separate monotonically increasing revision.

## Consequences

Active time is conservative and explainable, though short activity between lost heartbeats may be undercounted. The client must manage heartbeat lifecycle and offline progress separately. Exact rules and clock-driven concurrency tests are critical; see the algorithm document.
