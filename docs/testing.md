# Testing strategy

> **Document type: required QA plan.** A listed scenario is not a passed test. Only recorded command output and acceptance evidence in [implementation-plan.md](implementation-plan.md) count as verification.

## Test layers

| Layer | Scope | Tools | Default command |
|---|---|---|---|
| Go unit | domain/use cases, injected clock/IDs/repos/providers | `testing`, `testify` | `make test-unit` |
| Go HTTP | validation, middleware, auth/error mapping | `httptest`, Gin test mode | included in unit |
| integration | Bun repositories, SQL constraints/transactions, migrations, RustFS adapter | `testcontainers-go`, real PostgreSQL/RustFS-compatible container | `make test-integration` |
| frontend unit/component | stores, hooks, preferences, menus/popovers/keyboard/a11y | Vitest, React Testing Library | `make test-unit` |
| end-to-end | real browser against composed app | Playwright | `make test-e2e` |
| contract/static | OpenAPI, Go lint, ESLint/Prettier/typecheck, Compose model | Redocly, golangci-lint, toolchain | `make lint` |

Unit tests must not sleep or use wall time: inject a clock and advance it deterministically. Integration tests must create isolated databases/buckets, wait on health rather than fixed delays, clean resources, and remain safe under parallel CI. Do not replace required database semantics (`SKIP LOCKED`, unique/partial indexes, time zones) with SQLite or in-memory mocks.

## Critical scenarios

### Sessions and progress

1. duplicate heartbeat credits zero on replay and returns stable state;
2. same idempotency key with a different payload conflicts;
3. repeated/lower/gapped sequence is handled without double credit;
4. hidden, blurred or idle interval is not active; late interval is capped/not credited;
5. stale finalizer uses the last trustworthy boundary and races safely with a new heartbeat;
6. duplicate finish is safe; heartbeat/finish race has one outcome;
7. an old progress revision cannot overwrite a new cross-device position;
8. DST/timezone aggregate boundaries and full aggregate rebuild are deterministic.

### Identity and ownership

Test Argon2id validation, login throttling, cookie/CSRF/CORS behavior, refresh rotation and replay-family revocation, one/all-device logout, expiry, malformed JWT, and every cross-user resource: books/chapters/download, progress/session, dictionary occurrence, bookmark, highlight and note. Ensure errors do not reveal existence.

### Books, storage and jobs

Test EPUB/FB2/TXT happy paths; MIME/signature mismatch; oversized body; ZIP path traversal, symlink, extreme ratio/count/inflated size; malformed XML/EPUB; sanitizer XSS vectors; object timeout/retry/multipart abort; original retention on failure; processing retry without duplicate chapters/assets; lease expiry and two-worker `SKIP LOCKED`; dead-letter/backoff; cleanup only of unreferenced trusted keys; persistence after restart.

### Translation and dictionary

Test canonical cache key dimensions, cache hit without a second provider call, TTL/invalidation/version change, timeout/retry/circuit/rate limits, response validation, max text/context and log redaction. Concurrent duplicate dictionary inserts must yield one entry; repeat encounter creates one occurrence and atomically increments the counter; restore respects uniqueness; user edits are not overwritten by provider data.

### Frontend and PWA

Test query loading/empty/error states, Zustand UI state only, warm theme persistence, global/per-book preferences, scroll/paginated mode, command palette shortcuts/focus/escape/groups, selection/context menu, translation popover and mobile bottom sheet, add word/bookmark/highlight/note, offline position queue and 409 resolution, another-device resume, localization, reduced motion, keyboard-only and focus return. Run Playwright at desktop/tablet/mobile viewports.

## Commands

```bash
make test-unit
make test-integration        # Docker required by testcontainers
make test-e2e                # app/test server per Playwright configuration
make lint
make build
make generate-openapi

# Focused examples
cd backend && go test -race ./internal/readingsessions/...
cd backend && go test -tags=integration -run TestProgressConflict ./...
cd frontend && npm test -- --run
cd frontend && npm run test:e2e -- --project=chromium
```

The exact available package paths/tests evolve with implementation; use `go list ./...`, `go test -list .`, and `npm run` rather than claiming a named test exists. CI records actual failures. Coverage is diagnostic, not a substitute for critical-scenario assertions; raise thresholds only after excluding generated code and defining ownership.

## CI gates

The workflow runs independent backend, lint, integration, migration, frontend, OpenAPI, Compose and Docker jobs. Images are built but not published. Migration CI applies up, status, one down, then up against PostgreSQL. A production release additionally needs an environment smoke test and backup/restore evidence; CI success alone does not satisfy the MVP acceptance list.

## Manual acceptance record

Before declaring MVP complete, record date/commit/environment/evidence for all 30 product acceptance steps: clean Compose start, registration/login, each format upload/process/read, warm/font/layout/navigation, close/resume/cross-device, exact start/end/active-vs-idle, daily/session stats, selection/translation/dictionary context/deep-link, bookmark/highlight/note, command palette, and restart persistence. Also record browser/accessibility and destructive cleanup results. Unrun scenarios remain **not verified**, never implicitly passed.
