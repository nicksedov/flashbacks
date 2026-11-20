# Proposal

## Why

Authenticated requests, app/user settings reads, and per-request person name resolution are hot read paths that currently hit the database on every call. This adds latency and connection-pool pressure, especially under gallery/people loads. An in-memory TTL cache with explicit invalidation on writes will reduce DB load and improve response times without changing API contracts.

## What Changes

- Add a small internal TTL-cache primitive in `backend/core/internal/infrastructure/cache` (enforced per-entry TTL, read-lock fast path, explicit invalidation, testable clock).
- Wire it into:
  - Session lookup in `SessionRepository`; evict on logout, API revocation, password change and cleanup.
  - App and user settings reads; evict on `Save`/`Update`.
  - Person name resolution in the faces handlers: keep the request-scoped `map[uint]string` and cover it with a test (a rename is seen by the next request).
- Make the TTLs configurable (`SESSION_CACHE_TTL`, default 1m; `SETTINGS_CACHE_TTL`, default 5m) through `config.AppConfig` and the registry defaults.
- Add unit tests for cache hits/misses, TTL expiry, invalidation and concurrent access; keep `go vet` clean.
- No new user-facing strings; i18n unchanged.
- No OpenAPI/MCP contract changes.

## Capabilities

### New Capabilities

- `in-memory-ttl-cache`: internal cache primitive with TTL and explicit invalidation for write-path coordination.

### Modified Capabilities

- `user-auth`: session reads are served from the in-memory cache before the database and evicted on logout/revoke/password change.
- `user-settings`: app/user settings reads are served from cache and invalidated on update.
- `face-recognition`: per-request person name resolution is cached within the request; a rename is visible on the next request.

## Impact

- Affected code: `backend/core/internal/infrastructure/cache/`, `backend/core/internal/application/auth/session_repo.go`, `backend/core/internal/domain/repository/app_settings_repo.go`, `backend/core/internal/domain/repository/user_settings_repo.go`, `backend/core/internal/interfaces/handler/handlers_faces.go` (tests only), `backend/core/internal/infrastructure/config/config.go`, `backend/core/internal/infrastructure/di/providers.go`.
- Wire graph change in `core`: run `go generate ./...` and commit both `wire.go` and `wire_gen.go`.
- No API contract changes; no new locales.
