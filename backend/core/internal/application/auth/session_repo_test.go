package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/infrastructure/cache"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSessionRepository(t *testing.T) (*SessionRepository, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	config := DefaultSessionConfig()
	config.IdleTimeout = 1 * time.Hour
	config.AbsoluteTimeout = 24 * time.Hour
	sessions := cache.NewTTLCache[string, *domain.Session](time.Minute, nil)
	repo := NewSessionRepository(db, config, sessions)
	return repo, cleanup
}

func TestSessionRepository_CreateSession_Success(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotEmpty(t, token)
}

func TestSessionRepository_GetSession_Success(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	session, err := repo.GetSession(context.Background(), token)

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, user.ID, session.UserID)
}

func TestSessionRepository_GetSession_Revoked(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	err = repo.RevokeSession(context.Background(), token)
	require.NoError(t, err)

	_, err = repo.GetSession(context.Background(), token)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSessionRepository_GetSession_Expired(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Change the row behind the repository's back and drop the cached snapshot,
	// so the read has to be decided by the database.
	repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).Update("expires_at", time.Now().Add(-1*time.Hour))
	repo.sessions.InvalidateAll()

	_, err = repo.GetSession(context.Background(), token)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSessionRepository_GetSession_IdleTimeout(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).Update("last_seen_at", time.Now().Add(-2*time.Hour))
	repo.sessions.InvalidateAll()

	_, err = repo.GetSession(context.Background(), token)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSessionRepository_RevokeSession(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	err = repo.RevokeSession(context.Background(), token)

	require.NoError(t, err)
}

func TestSessionRepository_RevokeAllUserSessions(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	for i := 0; i < 3; i++ {
		_, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
		require.NoError(t, err)
	}

	err := repo.RevokeAllUserSessions(context.Background(), user.ID)

	require.NoError(t, err)

	var activeCount int64
	repo.db.Model(&domain.Session{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Count(&activeCount)
	assert.Equal(t, int64(0), activeCount)
}

func TestSessionRepository_CleanupExpiredSessions(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	_, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Create an expired session
	repo.db.Create(&domain.Session{
		UserID:       user.ID,
		SessionToken: "expired-hash",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		IPAddress:    "127.0.0.1",
	})

	// Create a revoked session
	revokedAt := time.Now()
	repo.db.Create(&domain.Session{
		UserID:       user.ID,
		SessionToken: "revoked-hash",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		RevokedAt:    &revokedAt,
		IPAddress:    "127.0.0.1",
	})

	err = repo.CleanupExpiredSessions(context.Background())
	require.NoError(t, err)

	var count int64
	repo.db.Model(&domain.Session{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

// TestSessionRepository_GetSession_ServedFromCache proves the read path really
// answers from memory: the row is gone, yet the session is returned.
func TestSessionRepository_GetSession_ServedFromCache(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)
	require.NoError(t, repo.db.Where("user_id = ?", user.ID).Delete(&domain.Session{}).Error)

	session, err := repo.GetSession(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, session.UserID)
}

// TestSessionRepository_GetSession_PopulatesCache proves a miss stores the row.
func TestSessionRepository_GetSession_PopulatesCache(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)
	repo.sessions.InvalidateAll()

	_, err = repo.GetSession(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.sessions.Len(), "the miss must store the session")

	// The stored row can disappear; the second read still succeeds from cache.
	require.NoError(t, repo.db.Where("user_id = ?", user.ID).Delete(&domain.Session{}).Error)
	_, err = repo.GetSession(context.Background(), token)
	require.NoError(t, err)
}

// TestSessionRepository_GetSession_ReturnsCopy guards against handing the shared
// cached pointer to callers.
func TestSessionRepository_GetSession_ReturnsCopy(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	first, err := repo.GetSession(context.Background(), token)
	require.NoError(t, err)
	first.UserAgent = "mutated-by-caller"

	second, err := repo.GetSession(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "test-agent", second.UserAgent, "the cached snapshot must not be mutable through the result")
}

// TestSessionRepository_RevokeSession_EvictsCache guards the security property:
// a logged-out session must stop being served immediately.
func TestSessionRepository_RevokeSession_EvictsCache(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)
	_, err = repo.GetSession(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, 1, repo.sessions.Len())

	require.NoError(t, repo.RevokeSession(context.Background(), token))
	assert.Equal(t, 0, repo.sessions.Len(), "revocation must evict the session")

	_, err = repo.GetSession(context.Background(), token)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestSessionRepository_RevokeAllUserSessions_EvictsCache covers password change
// and admin deactivation, which revoke every session of a user.
func TestSessionRepository_RevokeAllUserSessions_EvictsCache(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	var tokens []string
	for i := 0; i < 3; i++ {
		token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
		require.NoError(t, err)
		tokens = append(tokens, token)
	}
	require.Equal(t, 3, repo.sessions.Len())

	require.NoError(t, repo.RevokeAllUserSessions(context.Background(), user.ID))
	assert.Equal(t, 0, repo.sessions.Len(), "user-wide revocation must clear the cache")

	for _, token := range tokens {
		_, err := repo.GetSession(context.Background(), token)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}
}

// TestSessionRepository_StaleIdleSnapshotFallsBackToDB covers an apparently idle
// cached snapshot whose database row was refreshed by UpdateLastSeen: the read
// must consult the database instead of signing the user out.
func TestSessionRepository_StaleIdleSnapshotFallsBackToDB(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Simulate a snapshot that lags behind the row (UpdateLastSeen writes the
	// database only): the cached copy claims the session has been idle for two
	// hours while the row was seen a moment ago.
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	repo.sessions.Set(sessionCacheKey(tokenHash), &domain.Session{
		UserID:       user.ID,
		SessionToken: tokenHash,
		CreatedAt:    time.Now(),
		LastSeenAt:   time.Now().Add(-2 * time.Hour),
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	require.NoError(t, repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).
		Update("last_seen_at", time.Now()).Error)

	session, err := repo.GetSession(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, session.UserID)
}

func TestSessionRepository_UpdateLastSeenIsThrottled(t *testing.T) {
	repo, cleanup := setupSessionRepository(t)
	defer cleanup()
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")
	token, err := repo.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// A recent timestamp is not rewritten: a page fires hundreds of requests
	// that would otherwise serialize on this row.
	recent := time.Now().Add(-10 * time.Second)
	require.NoError(t, repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).
		Update("last_seen_at", recent).Error)
	require.NoError(t, repo.UpdateLastSeen(context.Background(), token))
	var session domain.Session
	require.NoError(t, repo.db.Where("user_id = ?", user.ID).First(&session).Error)
	assert.WithinDuration(t, recent, session.LastSeenAt, time.Second)

	// After the throttle window the write is applied.
	stale := time.Now().Add(-2 * time.Minute)
	require.NoError(t, repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).
		Update("last_seen_at", stale).Error)
	require.NoError(t, repo.UpdateLastSeen(context.Background(), token))
	require.NoError(t, repo.db.Where("user_id = ?", user.ID).First(&session).Error)
	assert.True(t, session.LastSeenAt.After(stale.Add(time.Minute)))
}
