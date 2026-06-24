package auth

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserService(t *testing.T) (*UserService, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	config := DefaultSessionConfig()
	config.IdleTimeout = 24 * time.Hour
	sessionRepo := NewSessionRepository(db, config)
	service := NewUserService(db, sessionRepo)
	return service, cleanup
}

func TestUserService_CreateUser_Success(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")

	input := &CreateUserInput{
		Login:       "newuser",
		DisplayName: "New User",
		Role:        domain.RoleUser,
		Password:    "secure-password",
	}

	user, err := svc.CreateUser(admin.ID, input)

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "newuser", user.Login)
	assert.Equal(t, "New User", user.DisplayName)
	assert.Equal(t, "user", string(user.Role))
	assert.True(t, user.IsActive)
	assert.True(t, user.MustChangePassword)
}

func TestUserService_CreateUser_NonAdmin(t *testing.T) {
	svc, _ := setupUserService(t)
	regularUser := testutil.SeedUserWithHash(t, svc.db, "user", "user", domain.RoleUser, true, "hashed")

	input := &CreateUserInput{
		Login:       "newuser",
		DisplayName: "New User",
		Role:        domain.RoleUser,
		Password:    "secure-password",
	}

	_, err := svc.CreateUser(regularUser.ID, input)

	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestUserService_CreateUser_DuplicateLogin(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")
	testutil.SeedUserWithHash(t, svc.db, "existing-user", "user", domain.RoleUser, true, "hashed")

	input := &CreateUserInput{
		Login:       "existing-user",
		DisplayName: "Duplicate",
		Role:        domain.RoleUser,
		Password:    "secure-password",
	}

	_, err := svc.CreateUser(admin.ID, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user with this login already exists")
}

func TestUserService_CreateUser_ShortPassword(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")

	input := &CreateUserInput{
		Login:       "newuser",
		DisplayName: "New User",
		Role:        domain.RoleUser,
		Password:    "short",
	}

	_, err := svc.CreateUser(admin.ID, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "password must be between 8 and 128 characters")
}

func TestUserService_CreateUser_InvalidRole(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")

	input := &CreateUserInput{
		Login:       "newuser",
		DisplayName: "New User",
		Role:        "invalid-role",
		Password:    "secure-password",
	}

	_, err := svc.CreateUser(admin.ID, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestUserService_UpdateUser_DisplayName(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")
	user := testutil.SeedUserWithHash(t, svc.db, "user", "user", domain.RoleUser, true, "hashed")

	newName := "Updated Name"
	input := &UpdateUserInput{
		DisplayName: &newName,
	}

	updatedUser, err := svc.UpdateUser(admin.ID, user.ID, input)

	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updatedUser.DisplayName)
}

func TestUserService_UpdateUser_DeactivateLastAdmin(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")

	isActive := false
	input := &UpdateUserInput{
		IsActive: &isActive,
	}

	_, err := svc.UpdateUser(admin.ID, admin.ID, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot deactivate the last admin")
}

func TestUserService_UpdateUser_DemoteLastAdmin(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")

	newRole := domain.RoleUser
	input := &UpdateUserInput{
		Role: &newRole,
	}

	_, err := svc.UpdateUser(admin.ID, admin.ID, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot demote the last admin")
}

func TestUserService_DeleteUser_Success(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")
	user := testutil.SeedUserWithHash(t, svc.db, "user", "user", domain.RoleUser, true, "hashed")

	err := svc.DeleteUser(admin.ID, user.ID)

	require.NoError(t, err)

	// Verify user is deleted
	_, err = svc.GetUser(user.ID)
	require.Error(t, err)
}

func TestUserService_DeleteUser_LastAdmin(t *testing.T) {
	svc, _ := setupUserService(t)
	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "admin", domain.RoleAdmin, true, "hashed")

	err := svc.DeleteUser(admin.ID, admin.ID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete the last admin")
}

func TestUserService_UpdateProfile_Success(t *testing.T) {
	svc, _ := setupUserService(t)
	user := testutil.SeedUserWithHash(t, svc.db, "user", "user", domain.RoleUser, true, "hashed")

	updatedUser, err := svc.UpdateProfile(user.ID, "New Display Name")

	require.NoError(t, err)
	assert.Equal(t, "New Display Name", updatedUser.DisplayName)
}

func TestUserService_ListUsers_Empty(t *testing.T) {
	svc, _ := setupUserService(t)

	users, err := svc.ListUsers()

	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUserService_ListUsers_WithUsers(t *testing.T) {
	svc, _ := setupUserService(t)
	// Set explicit CreatedAt times to ensure ordering (SQLite has same-time precision)
	now := time.Now()
	u1 := testutil.SeedUserWithHash(t, svc.db, "user1", "user", domain.RoleUser, true, "hashed1")
	svc.db.Model(u1).Update("created_at", now.Add(-2*time.Second))
	u2 := testutil.SeedUserWithHash(t, svc.db, "user2", "user", domain.RoleUser, true, "hashed2")
	svc.db.Model(u2).Update("created_at", now.Add(-1*time.Second))
	u3 := testutil.SeedUserWithHash(t, svc.db, "user3", "user", domain.RoleUser, true, "hashed3")
	svc.db.Model(u3).Update("created_at", now)

	users, err := svc.ListUsers()

	require.NoError(t, err)
	assert.Len(t, users, 3)
	// Should be ordered by created_at desc, so user3 should be first
	assert.Equal(t, "user3", users[0].Login)
}

func TestListAuditLogs_Pagination(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Create 60 audit logs
	actorID := uint(1)
	for i := 0; i < 60; i++ {
		testutil.SeedAuditLog(t, db, &actorID, domain.ActionLogin, "session", nil)
	}

	logs, total, err := ListAuditLogs(db, 1, 50)

	require.NoError(t, err)
	assert.Equal(t, int64(60), total)
	assert.Len(t, logs, 50)
}
