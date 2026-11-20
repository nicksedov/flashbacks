# In-memory TTL Cache — Design

## Cache primitive

`backend/core/internal/infrastructure/cache` holds a small, dependency-free
primitive (`TTLCache[K comparable, V any]`):

- Per-entry absolute expiry; `ttl <= 0` means "never expires" (the zero
  `expiresAt` is the sentinel, not a timestamp to compare against).
- Reads take only a read lock. The write lock is taken to store a value or to
  delete an entry observed as expired, with a re-check under the lock.
- No loader and no single-flight: a miss always goes to the caller's data
  source. The cache does not claim to collapse a thundering herd.
- `Invalidate`, `InvalidateAll`, `Len` (live entries only, expired ones are
  dropped) and `DeleteExpired`.
- Every 256 writes an opportunistic sweep drops expired entries, so a cache
  whose entries are never read again does not pin memory. There is no
  background goroutine and no size cap: entries are invalidated on write, so
  the live set is bounded by the data being cached.
- The clock is injectable for tests.

```go
package cache

func NewTTLCache[K comparable, V any](ttl time.Duration, clock func() time.Time) *TTLCache[K, V]

func (c *TTLCache[K, V]) Get(key K) (V, bool)
func (c *TTLCache[K, V]) Set(key K, value V)
func (c *TTLCache[K, V]) Invalidate(key K)
func (c *TTLCache[K, V]) InvalidateAll()
func (c *TTLCache[K, V]) Len() int
func (c *TTLCache[K, V]) DeleteExpired() int
```

## Invalidation

There is no shared invalidation handle. Each cache is a concrete, typed instance
at its call site, so the write paths evict it directly:

| Cache | Type | Key | Evicted by |
|---|---|---|---|
| Sessions | `TTLCache[string, *domain.Session]` | `"session:" + tokenHash` | `RevokeSession` (that key); `RevokeAllUserSessions`, `CleanupExpiredSessions` (`InvalidateAll`) |
| App settings | `TTLCache[int, *domain.AppSettings]` | `1` (singleton row) | `Save`, `Update` |
| User settings | `TTLCache[uint, *domain.UserSettings]` | `userID` | `Save` |
| Person names | request-scoped `map[uint]string` | `personID` | never global — the map dies with the request |

A user-wide revocation cannot enumerate token hashes (the key does not carry the
user ID), so it clears the whole session cache. Session entries are cheap to
rebuild and that path (logout everywhere, password change, admin deactivation)
is rare.

## Staleness and validation rules

- Cache hits do not refresh the TTL: staleness stays predictable.
- A cached session is served only when it is not revoked, not past
  `ExpiresAt`, and its `LastSeenAt` is within `IdleTimeout`.
- `UpdateLastSeen` writes the database only, so a snapshot's `LastSeenAt` can
  lag. An apparently idle snapshot is evicted and the read falls through to the
  database, which makes the final call against the freshest row instead of
  signing an active user out.
- Reads and writes hand out copies of the cached struct, so callers never share
  a mutable pointer.
- Correctness relies on eviction happening in the same process (single `core`
  instance). A database change made outside the repository is visible no later
  than the TTL.

## Wire DI impact (core)

- Providers live in `backend/core/internal/infrastructure/di/providers.go`, in
  `CoreSet`:
  - `ProvideSessionCache(cfg) *cache.TTLCache[string, *domain.Session]`
  - `ProvideAppSettingsCache(cfg) *cache.TTLCache[int, *domain.AppSettings]`
  - `ProvideUserSettingsCache(cfg) *cache.TTLCache[uint, *domain.UserSettings]`
- The settings and session repository providers take the matching cache.
- TTLs come from configuration, not from hard-coded constants:
  `SESSION_CACHE_TTL` (default 1m) and `SETTINGS_CACHE_TTL` (default 5m), both
  also exposed through the registry defaults.
- `AuthMiddleware` is unchanged: it goes through `AuthService` →
  `SessionRepository`, which owns the cache.
- After editing the graph, run `go generate ./...` and commit both `wire.go`
  and `wire_gen.go`.

## Behavioral notes

- A miss removes the expired entry before returning.
- Invalidation is synchronous: the write path returns only after the entry is
  gone, so a revoked session cannot be served afterwards.
- All caches live in process; there is no cross-instance coherency.
