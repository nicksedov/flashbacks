package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/infrastructure/cache"

	"gorm.io/gorm"
)

// SessionConfig holds session lifetime configuration
type SessionConfig struct {
	IdleTimeout     time.Duration // Session expires after this period of inactivity
	AbsoluteTimeout time.Duration // Session expires after this time regardless of activity
	CookieMaxAge    int           // Max-Age attribute for persistent cookie (seconds)
	TokenLength     int           // Length of the random token in bytes
}

// DefaultSessionConfig returns the default session configuration
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour, // 30 days
		AbsoluteTimeout: 90 * 24 * time.Hour, // 90 days
		CookieMaxAge:    30 * 24 * 60 * 60,   // 30 days in seconds
		TokenLength:     64,                  // 64 bytes = 512 bits
	}
}

// SessionRepository handles database operations for sessions.
//
// Reads go through an optional in-memory cache keyed by the token hash. The
// write paths (logout, revoke, password change, cleanup) evict it, so a revoked
// session stops being served as soon as the write returns. The cache is a
// process-local optimization: correctness still comes from the database, which
// every miss consults.
type SessionRepository struct {
	db     *gorm.DB
	config *SessionConfig
	// sessions may be nil, which disables caching (useful in tests).
	sessions *cache.TTLCache[string, *domain.Session]
}

// NewSessionRepository creates a new session repository. A nil cache disables
// caching.
func NewSessionRepository(db *gorm.DB, config *SessionConfig, sessions *cache.TTLCache[string, *domain.Session]) *SessionRepository {
	return &SessionRepository{
		db:       db,
		config:   config,
		sessions: sessions,
	}
}

// sessionCacheKey is the cache key for a stored token hash.
func sessionCacheKey(tokenHash string) string {
	return "session:" + tokenHash
}

// sessionUsable reports whether a cached snapshot may still be served. The
// snapshot's LastSeenAt can lag behind the database (UpdateLastSeen writes the
// row only), so an apparently idle session is treated as unused: the caller
// then falls through to the database, which makes the final call against the
// freshest timestamp instead of signing an active user out.
func (r *SessionRepository) sessionUsable(session *domain.Session, now time.Time) bool {
	if session == nil || session.RevokedAt != nil {
		return false
	}
	if now.After(session.ExpiresAt) {
		return false
	}
	return now.Sub(session.LastSeenAt) <= r.config.IdleTimeout
}

// CreateSession creates a new session for a user and returns the session token
func (r *SessionRepository) CreateSession(ctx context.Context, userID uint, ipAddress, userAgent string) (string, error) {
	token, err := GenerateSecureToken(r.config.TokenLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	// Hash the token for storage
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))

	now := time.Now()
	session := domain.Session{
		UserID:       userID,
		SessionToken: tokenHash,
		CreatedAt:    now,
		LastSeenAt:   now,
		ExpiresAt:    now.Add(r.config.AbsoluteTimeout),
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	if err := r.db.Create(&session).Error; err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	if r.sessions != nil {
		r.sessions.Set(sessionCacheKey(tokenHash), &session)
	}

	return token, nil
}

// GetSession retrieves a session by token and validates it
func (r *SessionRepository) GetSession(ctx context.Context, token string) (*domain.Session, error) {
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	key := sessionCacheKey(tokenHash)

	// Fast path: a cached snapshot that is still usable is served without a
	// database round trip. A snapshot that is revoked, expired or apparently
	// idle is evicted and the read falls through, so the database gets to
	// revoke rows and to judge idle time against the freshest last_seen_at.
	if r.sessions != nil {
		if cached, ok := r.sessions.Get(key); ok {
			if r.sessionUsable(cached, time.Now()) {
				snapshot := *cached
				return &snapshot, nil
			}
			r.sessions.Invalidate(key)
		}
	}

	var session domain.Session
	if err := r.db.Where("session_token = ? AND revoked_at IS NULL", tokenHash).First(&session).Error; err != nil {
		return nil, err
	}

	// Check if session has expired
	now := time.Now()
	if now.After(session.ExpiresAt) {
		// Mark as revoked
		r.db.Model(&session).Update("revoked_at", now)
		return nil, gorm.ErrRecordNotFound
	}

	// Check idle timeout
	if now.Sub(session.LastSeenAt) > r.config.IdleTimeout {
		r.db.Model(&session).Update("revoked_at", now)
		return nil, gorm.ErrRecordNotFound
	}

	if r.sessions != nil {
		// Store a copy: the returned pointer is the caller's, and a mutable
		// shared snapshot would race between concurrent requests.
		stored := session
		r.sessions.Set(key, &stored)
	}

	return &session, nil
}

// UpdateLastSeen updates the last_seen_at timestamp for a session.
//
// The write is throttled to once a minute per session: every authenticated
// request updates this row, and a page that fires hundreds of requests at once
// (the People grid loads a thumbnail per tile) would otherwise serialize all of
// them on one row and exhaust the connection pool.
//
// The cached snapshot keeps its older last_seen_at; that is deliberate, because
// refreshing it on every request would keep the entry alive past its TTL and
// hide revocations. GetSession resolves an apparently idle snapshot against the
// database.
func (r *SessionRepository) UpdateLastSeen(ctx context.Context, token string) error {
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	now := time.Now()
	return r.db.Model(&domain.Session{}).
		Where("session_token = ? AND revoked_at IS NULL AND last_seen_at < ?", tokenHash, now.Add(-time.Minute)).
		Update("last_seen_at", now).Error
}

// RevokeSession marks a session as revoked and evicts it from the cache.
func (r *SessionRepository) RevokeSession(ctx context.Context, token string) error {
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	now := time.Now()
	err := r.db.Model(&domain.Session{}).
		Where("session_token = ?", tokenHash).
		Updates(map[string]interface{}{
			"revoked_at": now,
		}).Error
	if err != nil {
		return err
	}
	if r.sessions != nil {
		r.sessions.Invalidate(sessionCacheKey(tokenHash))
	}
	return nil
}

// RevokeAllUserSessions revokes all sessions for a user.
//
// The cache is keyed by token hash, which does not carry the user ID, so a
// user-wide revocation clears the whole cache. Session entries are cheap to
// rebuild and this path (password change, admin deactivation) is rare.
func (r *SessionRepository) RevokeAllUserSessions(ctx context.Context, userID uint) error {
	now := time.Now()
	err := r.db.Model(&domain.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
	if err != nil {
		return err
	}
	if r.sessions != nil {
		r.sessions.InvalidateAll()
	}
	return nil
}

// CleanupExpiredSessions removes expired and revoked sessions from the database.
// The cached snapshots of deleted rows are dropped as well.
func (r *SessionRepository) CleanupExpiredSessions(ctx context.Context) error {
	now := time.Now()
	err := r.db.Where("expires_at < ? OR revoked_at IS NOT NULL", now).Delete(&domain.Session{}).Error
	if err != nil {
		return err
	}
	if r.sessions != nil {
		r.sessions.InvalidateAll()
	}
	return nil
}

// GetSessionConfig returns the session configuration
func (r *SessionRepository) GetSessionConfig() *SessionConfig {
	return r.config
}
