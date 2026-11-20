# In-memory TTL Cache — Delta Spec

## ADDED Requirements

### Requirement: Session lookup served from in-memory cache
Authenticated session reads consult an in-memory TTL cache before querying the database. On a cache miss, the session is fetched from Postgres and stored in the cache. On a hit, the cached session is returned if it is not revoked, not expired and not apparently idle.

<!-- focused-spec:id=in-memory-ttl-cache.session-hit-returns-cached-session-without-db-read -->
#### Scenario: Session hit returns cached session without DB read

- **WHEN** a valid session token is presented and the session exists in the in-memory cache
- **THEN** the read returns the session without issuing a database query, and the caller's copy is independent of the cached snapshot

<!-- focused-spec:id=in-memory-ttl-cache.session-miss-falls-back-to-db-and-populates-cache -->
#### Scenario: Session miss falls back to DB and populates cache

- **WHEN** a valid session token is presented and the session is not in the cache
- **THEN** the session is fetched from the database and stored in the cache for subsequent requests

<!-- focused-spec:id=in-memory-ttl-cache.logout-revoke-password-change-invalidates-the-session-cache -->
#### Scenario: Logout/revoke/password change invalidates the session cache

- **WHEN** a session is revoked via logout, API revocation or password change
- **THEN** the session is removed from the cache and subsequent requests fetch fresh state from the database

<!-- focused-spec:id=in-memory-ttl-cache.stale-idle-snapshot-judged-by-database -->
#### Scenario: An apparently idle cached session is judged by the database

- **WHEN** a cached session snapshot has not been seen for longer than the idle timeout while its database row was refreshed by `UpdateLastSeen`
- **THEN** the snapshot is evicted and the read falls through to the database, which keeps the active session instead of signing the user out

### Requirement: App and user settings reads served from cache
AppSettings and UserSettings reads are served from an in-memory TTL cache. Writes update the database and invalidate the corresponding cache entry.

<!-- focused-spec:id=in-memory-ttl-cache.settings-read-hits-cache -->
#### Scenario: Settings read hits cache

- **WHEN** AppSettings or UserSettings are read and the cache is populated and fresh
- **THEN** the read is served from cache without a database query

<!-- focused-spec:id=in-memory-ttl-cache.settings-update-invalidates-cache -->
#### Scenario: Settings update invalidates cache

- **WHEN** AppSettings or UserSettings are updated
- **THEN** the cache entry is invalidated so the next read fetches the new value from the database

### Requirement: Per-request person name resolution cached within the request
Person name resolution uses a request-scoped cache: every request starts with an empty map, so a name is resolved once per request and a rename is visible to the following request.

<!-- focused-spec:id=in-memory-ttl-cache.person-name-cached-within-a-request -->
#### Scenario: Person name cached within a request

- **WHEN** multiple faces in a request reference the same person
- **THEN** the name is resolved once and reused from the request-scoped cache

<!-- focused-spec:id=in-memory-ttl-cache.person-rename-observed-on-the-next-request -->
#### Scenario: Person rename observed on the next request

- **WHEN** a person is renamed between two requests
- **THEN** the next request resolves the new name because its cache starts empty
