package auth

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

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
	repo := NewSessionRepository(db, config)
	return repo, cleanup
}

func TestSessionRepository_CreateSession_Success(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotEmpty(t, token)
}

func TestSessionRepository_GetSession_Success(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	session, err := repo.GetSession(token)

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, user.ID, session.UserID)
}

func TestSessionRepository_GetSession_Revoked(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	err = repo.RevokeSession(token)
	require.NoError(t, err)

	_, err = repo.GetSession(token)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSessionRepository_GetSession_Expired(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).Update("expires_at", time.Now().Add(-1*time.Hour))

	_, err = repo.GetSession(token)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSessionRepository_GetSession_IdleTimeout(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	repo.db.Model(&domain.Session{}).Where("user_id = ?", user.ID).Update("last_seen_at", time.Now().Add(-2*time.Hour))

	_, err = repo.GetSession(token)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSessionRepository_RevokeSession(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	token, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	err = repo.RevokeSession(token)

	require.NoError(t, err)
}

func TestSessionRepository_RevokeAllUserSessions(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	for i := 0; i < 3; i++ {
		_, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
		require.NoError(t, err)
	}

	err := repo.RevokeAllUserSessions(user.ID)

	require.NoError(t, err)

	var activeCount int64
	repo.db.Model(&domain.Session{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Count(&activeCount)
	assert.Equal(t, int64(0), activeCount)
}

func TestSessionRepository_CleanupExpiredSessions(t *testing.T) {
	repo, _ := setupSessionRepository(t)
	user := testutil.SeedUserWithHash(t, repo.db, "user", "user", domain.RoleUser, true, "hashed")

	_, err := repo.CreateSession(user.ID, "127.0.0.1", "test-agent")
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

	err = repo.CleanupExpiredSessions()
	require.NoError(t, err)

	var count int64
	repo.db.Model(&domain.Session{}).Count(&count)
	assert.Equal(t, int64(1), count)
}
