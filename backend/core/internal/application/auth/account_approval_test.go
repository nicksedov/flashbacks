package auth

import (
	"context"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupAuthServiceWithMode creates an auth service with the given account creation mode.
func setupAuthServiceWithMode(t *testing.T, mode string) (*gorm.DB, *AuthService, *UserService, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)

	sessionConfig := &SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour,
		AbsoluteTimeout: 90 * 24 * time.Hour,
		CookieMaxAge:    30 * 24 * 60 * 60,
		TokenLength:     64,
	}

	sessionRepo := NewSessionRepository(db, sessionConfig)
	bootstrap := NewBootstrapService(db, "bootstrap_admin", "bootstrap123")
	loginLimiter := NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	registerLimiter := NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	authService := NewAuthService(db, bootstrap, sessionRepo, loginLimiter, registerLimiter, mode)
	userService := NewUserService(db, sessionRepo)

	return db, authService, userService, cleanup
}

func TestAuthService_RegisterUser_SelfService(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfService)
	defer cleanup()

	// Seed a user to leave bootstrap mode
	testutil.SeedUserWithHash(t, db, "existing", "Existing", domain.RoleUser, true, "hash")

	result, err := authService.RegisterUser(context.Background(), "  NewUser  ", "New User", "password123", "127.0.0.1", "test-agent")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Pending)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, domain.AccountStatusActive, result.User.AccountStatus)
	assert.Equal(t, "newuser", result.User.Login)
	assert.Equal(t, domain.RoleUser, result.User.Role)
}

func TestAuthService_RegisterUser_SelfServiceWithApproval(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfServiceWithApproval)
	defer cleanup()

	testutil.SeedUserWithHash(t, db, "existing", "Existing", domain.RoleUser, true, "hash")

	result, err := authService.RegisterUser(context.Background(), "pendinguser", "Pending User", "password123", "127.0.0.1", "test-agent")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Pending)
	assert.Empty(t, result.Token)
	assert.Equal(t, domain.AccountStatusPending, result.User.AccountStatus)
	assert.Equal(t, domain.RoleUser, result.User.Role)
}

func TestAuthService_RegisterUser_AdminOnly(t *testing.T) {
	_, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeAdminOnly)
	defer cleanup()

	_, err := authService.RegisterUser(context.Background(), "newuser", "New User", "password123", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrRegistrationDisabled)
}

func TestAuthService_RegisterUser_BootstrapMode(t *testing.T) {
	// NewTestDB seeds no users, so bootstrap mode is active
	_, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfService)
	defer cleanup()

	_, err := authService.RegisterUser(context.Background(), "newuser", "New User", "password123", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrBootstrapMode)
}

func TestAuthService_RegisterUser_DuplicateLoginCaseInsensitive(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfService)
	defer cleanup()

	testutil.SeedUserWithHash(t, db, "ExistingUser", "Existing", domain.RoleUser, true, "hash")

	_, err := authService.RegisterUser(context.Background(), "existinguser", "New User", "password123", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrUserExists)
}

func TestAuthService_RegisterUser_ShortPassword(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfService)
	defer cleanup()

	testutil.SeedUserWithHash(t, db, "existing", "Existing", domain.RoleUser, true, "hash")

	_, err := authService.RegisterUser(context.Background(), "newuser", "New User", "short", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrPasswordLength)
}

func TestAuthService_RegisterUser_EmptyFields(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfService)
	defer cleanup()

	testutil.SeedUserWithHash(t, db, "existing", "Existing", domain.RoleUser, true, "hash")

	_, err := authService.RegisterUser(context.Background(), "   ", "New User", "password123", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrInvalidRequestFormat)
}

func TestAuthService_Login_PendingUser(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfServiceWithApproval)
	defer cleanup()

	passwordHash, err := HashPassword("password123")
	require.NoError(t, err)

	user := domain.User{
		Login:         "pendinguser",
		DisplayName:   "Pending User",
		Role:          domain.RoleUser,
		AccountStatus: domain.AccountStatusPending,
		PasswordHash:  passwordHash,
		IsActive:      true,
	}
	require.NoError(t, db.Create(&user).Error)

	_, err = authService.Login(context.Background(), "pendinguser", "password123", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrAccountPending)
}

func TestAuthService_Login_RejectedUser(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeAdminOnly)
	defer cleanup()

	passwordHash, err := HashPassword("password123")
	require.NoError(t, err)

	user := domain.User{
		Login:           "rejecteduser",
		DisplayName:     "Rejected User",
		Role:            domain.RoleUser,
		AccountStatus:   domain.AccountStatusRejected,
		PasswordHash:    passwordHash,
		IsActive:        true,
		RejectionReason: "Duplicate account",
	}
	require.NoError(t, db.Create(&user).Error)

	_, err = authService.Login(context.Background(), "rejecteduser", "password123", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrAccountRejected)
}

func TestAuthService_Login_PendingUser_WrongPassword(t *testing.T) {
	db, authService, _, cleanup := setupAuthServiceWithMode(t, domain.AccountCreationModeSelfServiceWithApproval)
	defer cleanup()

	passwordHash, err := HashPassword("password123")
	require.NoError(t, err)

	user := domain.User{
		Login:         "pendinguser",
		DisplayName:   "Pending User",
		Role:          domain.RoleUser,
		AccountStatus: domain.AccountStatusPending,
		PasswordHash:  passwordHash,
		IsActive:      true,
	}
	require.NoError(t, db.Create(&user).Error)

	// Non-disclosure rule: wrong password must not reveal the pending status
	_, err = authService.Login(context.Background(), "pendinguser", "wrongpassword", "127.0.0.1", "test-agent")
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestUserService_ApproveUser_Success(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "Admin", domain.RoleAdmin, true, "hash")
	pendingUser := testutil.SeedUserWithHash(t, svc.db, "pending", "Pending", domain.RoleUser, true, "hash")
	svc.db.Model(pendingUser).Update("account_status", domain.AccountStatusPending)

	user, err := svc.ApproveUser(context.Background(), admin.ID, pendingUser.ID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, domain.AccountStatusActive, user.AccountStatus)
	assert.NotNil(t, user.StatusChangedAt)
	assert.NotNil(t, user.StatusChangedBy)
}

func TestUserService_ApproveUser_NotPending(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "Admin", domain.RoleAdmin, true, "hash")
	activeUser := testutil.SeedUserWithHash(t, svc.db, "active", "Active", domain.RoleUser, true, "hash")

	_, err := svc.ApproveUser(context.Background(), admin.ID, activeUser.ID)
	require.ErrorIs(t, err, domain.ErrUserNotPending)
}

func TestUserService_ApproveUser_NonAdmin(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	regular := testutil.SeedUserWithHash(t, svc.db, "user", "User", domain.RoleUser, true, "hash")
	pendingUser := testutil.SeedUserWithHash(t, svc.db, "pending", "Pending", domain.RoleUser, true, "hash")
	svc.db.Model(pendingUser).Update("account_status", domain.AccountStatusPending)

	_, err := svc.ApproveUser(context.Background(), regular.ID, pendingUser.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestUserService_RejectUser_Success(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "Admin", domain.RoleAdmin, true, "hash")
	pendingUser := testutil.SeedUserWithHash(t, svc.db, "pending", "Pending", domain.RoleUser, true, "hash")
	svc.db.Model(pendingUser).Update("account_status", domain.AccountStatusPending)

	user, err := svc.RejectUser(context.Background(), admin.ID, pendingUser.ID, "Duplicate account")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, domain.AccountStatusRejected, user.AccountStatus)
	assert.Equal(t, "Duplicate account", user.RejectionReason)
	assert.NotNil(t, user.StatusChangedAt)
}

func TestUserService_RejectUser_NotPending(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "Admin", domain.RoleAdmin, true, "hash")
	activeUser := testutil.SeedUserWithHash(t, svc.db, "active", "Active", domain.RoleUser, true, "hash")

	_, err := svc.RejectUser(context.Background(), admin.ID, activeUser.ID, "No reason")
	require.ErrorIs(t, err, domain.ErrUserNotPending)
}

func TestUserService_ListUsers_ByStatus(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	pending1 := testutil.SeedUserWithHash(t, svc.db, "pending1", "Pending One", domain.RoleUser, true, "hash")
	svc.db.Model(pending1).Update("account_status", domain.AccountStatusPending)
	pending2 := testutil.SeedUserWithHash(t, svc.db, "pending2", "Pending Two", domain.RoleUser, true, "hash")
	svc.db.Model(pending2).Update("account_status", domain.AccountStatusPending)
	testutil.SeedUserWithHash(t, svc.db, "active", "Active", domain.RoleUser, true, "hash")
	rejected := testutil.SeedUserWithHash(t, svc.db, "rejected", "Rejected", domain.RoleUser, true, "hash")
	svc.db.Model(rejected).Update("account_status", domain.AccountStatusRejected)

	pendingUsers, err := svc.ListUsers(context.Background(), string(domain.AccountStatusPending))
	require.NoError(t, err)
	assert.Len(t, pendingUsers, 2)
	for _, u := range pendingUsers {
		assert.Equal(t, domain.AccountStatusPending, u.AccountStatus)
	}
}

func TestUserService_CreateUser_AdminActiveStatus(t *testing.T) {
	svc, cleanup := setupUserService(t)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, svc.db, "admin", "Admin", domain.RoleAdmin, true, "hash")

	input := &CreateUserInput{
		Login:       "NewUser",
		DisplayName: "New User",
		Role:        domain.RoleUser,
		Password:    "secure-password",
	}

	user, err := svc.CreateUser(context.Background(), admin.ID, input)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, domain.AccountStatusActive, user.AccountStatus)
	assert.Equal(t, "newuser", user.Login)
}
