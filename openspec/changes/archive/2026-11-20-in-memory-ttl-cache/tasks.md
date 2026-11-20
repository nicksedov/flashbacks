# In-memory TTL Cache — Tasks

## 1) Add cache primitive
- [x] Create `backend/core/internal/infrastructure/cache/ttl.go` with `TTLCache` (enforced TTL, read-lock fast path, `Invalidate`/`InvalidateAll`/`Len`/`DeleteExpired`, non-expiring mode for `ttl <= 0`, opportunistic sweep).
- [x] Add `backend/core/internal/infrastructure/cache/ttl_test.go`: hit/miss/expiry/invalidation, non-expiring mode, live-only `Len`, `DeleteExpired`, concurrent access (`-race`).
- [x] Run `cd backend/core && go test ./internal/infrastructure/cache/... -race` and `go vet ./...`.

## 2) Wire DI wiring (core)
- [x] Add providers to `backend/core/internal/infrastructure/di/providers.go` (in `CoreSet`): `ProvideSessionCache`, `ProvideAppSettingsCache`, `ProvideUserSettingsCache`; TTLs come from `SESSION_CACHE_TTL` / `SETTINGS_CACHE_TTL`.
- [x] Update repository providers to accept their caches; write paths evict directly (no shared invalidation handle).
- [x] Run `go generate ./...`; commit both `wire.go` and `wire_gen.go`.

## 3) Session cache integration
- [x] `SessionRepository.GetSession` serves usable snapshots from cache, populates on a miss, returns copies, and falls through to the database for revoked/expired/apparently-idle snapshots.
- [x] `RevokeSession` evicts its key; `RevokeAllUserSessions` and `CleanupExpiredSessions` clear the cache.
- [x] Add `backend/core/internal/application/auth/session_repo_test.go` cases: served-from-cache, populates, copy isolation, revocation eviction (single and user-wide), stale idle snapshot.
- [x] Run `cd backend/core && go test ./internal/application/auth/... -count=1`.

## 4) Settings cache integration
- [x] Wire `AppSettingsRepository.Get`/`Save`/`Update` with the app-settings cache; evict on `Save`/`Update`.
- [x] Wire `UserSettingsRepository.FindOrCreateByUserID`/`Save` with the user-settings cache; evict on `Save`.
- [x] Add `backend/core/internal/domain/repository/settings_cache_test.go` asserting cache hits, copy isolation and invalidation on writes.
- [x] Run `cd backend/core && go test ./internal/domain/repository/... -count=1`.

## 5) Person name resolution (faces)
- [x] Keep the per-request `map[uint]string` in handlers; no global cache needed.
- [x] Add handler tests asserting one repository lookup for repeated faces of one person, and a fresh request seeing a rename.

## 6) Cross-cutting
- [x] Verify no new user-facing strings; i18n unchanged.
- [x] Verify no OpenAPI/MCP contract changes.
- [x] Run `go build ./...`, `go vet ./...`, `go test ./... -count=1` for `backend/core`.
