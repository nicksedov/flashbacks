package auth

import (
	"testing"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBootstrapService(t *testing.T) (*BootstrapService, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	service := NewBootstrapService(db, "admin", "bootstrap-secret")
	return service, cleanup
}

func TestBootstrapService_IsBootstrapMode_EmptyDB(t *testing.T) {
	svc, _ := setupBootstrapService(t)

	result, err := svc.IsBootstrapMode()

	require.NoError(t, err)
	assert.True(t, result, "should be in bootstrap mode with empty DB")
}

func TestBootstrapService_IsBootstrapMode_WithUsers(t *testing.T) {
	svc, _ := setupBootstrapService(t)
	testutil.SeedUserWithHash(t, svc.db, "existing-user", "user", domain.RoleUser, true, "hashed-password")

	result, err := svc.IsBootstrapMode()

	require.NoError(t, err)
	assert.False(t, result, "should not be in bootstrap mode when users exist")
}

func TestBootstrapService_ValidateCredentials_Match(t *testing.T) {
	svc, _ := setupBootstrapService(t)

	result := svc.ValidateBootstrapCredentials("admin", "bootstrap-secret")

	assert.True(t, result, "credentials should match")
}

func TestBootstrapService_ValidateCredentials_Mismatch(t *testing.T) {
	svc, _ := setupBootstrapService(t)

	tests := []struct {
		name     string
		login    string
		password string
	}{
		{"wrong login", "wrong-admin", "bootstrap-secret"},
		{"wrong password", "admin", "wrong-password"},
		{"both wrong", "wrong-admin", "wrong-password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ValidateBootstrapCredentials(tt.login, tt.password)
			assert.False(t, result, "credentials should not match")
		})
	}
}

func TestBootstrapService_CreateBootstrapAdmin_Success(t *testing.T) {
	svc, _ := setupBootstrapService(t)

	user, err := svc.CreateBootstrapAdmin("new-admin-password", "Admin User")

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "admin", user.Login)
	assert.Equal(t, "Admin User", user.DisplayName)
	assert.Equal(t, "admin", string(user.Role))
	assert.True(t, user.IsActive)
	assert.False(t, user.MustChangePassword)
	assert.NotEmpty(t, user.PasswordHash)

	// Verify bootstrap mode is now disabled
	isBootstrap, err := svc.IsBootstrapMode()
	require.NoError(t, err)
	assert.False(t, isBootstrap, "bootstrap mode should be disabled after admin creation")
}

func TestBootstrapService_CreateBootstrapAdmin_NotInBootstrap(t *testing.T) {
	svc, _ := setupBootstrapService(t)
	// Seed a user to exit bootstrap mode
	testutil.SeedUserWithHash(t, svc.db, "existing-user", "user", domain.RoleUser, true, "hashed-password")

	_, err := svc.CreateBootstrapAdmin("new-password", "New Admin")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap mode is already disabled")
}
