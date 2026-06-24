package auth

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"gorm.io/gorm"
)

// setupAuthService creates a test database and all auth components.
func setupAuthService(t *testing.T) (*gorm.DB, *AuthService, *BootstrapService, *UserService, func()) {
	t.Helper()
	db, cleanupDB := testutil.NewTestDB(t)

	sessionConfig := &SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour,
		AbsoluteTimeout: 90 * 24 * time.Hour,
		CookieMaxAge:    30 * 24 * 60 * 60,
		TokenLength:     64,
	}

	sessionRepo := NewSessionRepository(db, sessionConfig)
	bootstrap := NewBootstrapService(db, "bootstrap_admin", "bootstrap123")
	loginLimiter := NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	authService := NewAuthService(db, bootstrap, sessionRepo, loginLimiter)
	userService := NewUserService(db, sessionRepo)

	return db, authService, bootstrap, userService, cleanupDB
}

func TestAuthService_Login_Success(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	// Create a user using the real HashPassword
	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	result, err := authService.Login("testuser", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("expected successful login, got error: %v", err)
	}
	if result.IsBootstrap {
		t.Fatal("expected IsBootstrap=false, got true")
	}
	if result.User == nil {
		t.Fatal("expected user in result, got nil")
	}
	if result.User.Login != "testuser" {
		t.Fatalf("expected login 'testuser', got '%s'", result.User.Login)
	}
	if result.Token == "" {
		t.Fatal("expected token in result, got empty")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	_, err = authService.Login("testuser", "wrongpassword", "127.0.0.1", "test-agent")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	_, authService, _, _, _ := setupAuthService(t)

	_, err := authService.Login("nonexistent", "password123", "127.0.0.1", "test-agent")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthService_Login_DeactivatedUser(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "deactivated",
		DisplayName:  "Deactivated User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     false,
	}
	db.Select("login", "display_name", "role", "password_hash", "is_active").Create(&user)
	// GORM treats false as zero value, need explicit update
	db.Model(&domain.User{}).Where("id = ?", user.ID).Update("is_active", false)

	_, err = authService.Login("deactivated", "password123", "127.0.0.1", "test-agent")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for deactivated user, got: %v", err)
	}
}

func TestAuthService_Login_BootstrapMode(t *testing.T) {
	// Use a fresh DB with no users to trigger bootstrap mode
	db, cleanupDB := testutil.NewTestDB(t)
	defer cleanupDB()

	// Remove seeded LlmProviders to make it truly empty for bootstrap check
	db.Exec("DELETE FROM llm_providers")
	db.Exec("DELETE FROM llm_settings")
	db.Exec("DELETE FROM app_settings")

	sessionConfig := &SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour,
		AbsoluteTimeout: 90 * 24 * time.Hour,
		CookieMaxAge:    30 * 24 * 60 * 60,
		TokenLength:     64,
	}
	sessionRepo := NewSessionRepository(db, sessionConfig)
	bootstrap := NewBootstrapService(db, "bootstrap_admin", "bootstrap123")
	loginLimiter := NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	authService := NewAuthService(db, bootstrap, sessionRepo, loginLimiter)

	result, err := authService.Login("bootstrap_admin", "bootstrap123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("expected successful bootstrap login, got error: %v", err)
	}
	if !result.IsBootstrap {
		t.Fatal("expected IsBootstrap=true, got false")
	}
	if result.Token != "" {
		t.Fatal("expected empty token for bootstrap, got non-empty")
	}
}

func TestAuthService_Login_BootstrapWrongCreds(t *testing.T) {
	db, cleanupDB := testutil.NewTestDB(t)
	defer cleanupDB()

	// Remove seeded data to trigger bootstrap mode
	db.Exec("DELETE FROM llm_providers")
	db.Exec("DELETE FROM llm_settings")
	db.Exec("DELETE FROM app_settings")

	sessionConfig := &SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour,
		AbsoluteTimeout: 90 * 24 * time.Hour,
		CookieMaxAge:    30 * 24 * 60 * 60,
		TokenLength:     64,
	}
	sessionRepo := NewSessionRepository(db, sessionConfig)
	bootstrap := NewBootstrapService(db, "bootstrap_admin", "bootstrap123")
	loginLimiter := NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	authService := NewAuthService(db, bootstrap, sessionRepo, loginLimiter)

	_, err := authService.Login("bootstrap_admin", "wrongpassword", "127.0.0.1", "test-agent")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthService_Login_RateLimited(t *testing.T) {
	_, authService, _, _, _ := setupAuthService(t)

	// Simulate rate limiting by creating a limiter that's already banning
	// We'll use a custom setup with a pre-banned IP
	db, cleanupDB := testutil.NewTestDB(t)
	defer cleanupDB()

	sessionConfig := &SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour,
		AbsoluteTimeout: 90 * 24 * time.Hour,
		CookieMaxAge:    30 * 24 * 60 * 60,
		TokenLength:     64,
	}
	sessionRepo := NewSessionRepository(db, sessionConfig)
	bootstrap := NewBootstrapService(db, "bootstrap_admin", "bootstrap123")
	// Use maxAttempts=1 for easier testing
	loginLimiter := NewLoginRateLimiter(1, 15*time.Minute, 30*time.Minute)
	authService = NewAuthService(db, bootstrap, sessionRepo, loginLimiter)

	// Trigger the ban with one failure
	loginLimiter.RecordFailure("banned_ip")

	// Now try to login - should be rate limited
	_, err := authService.Login("anyuser", "anypass", "banned_ip", "test-agent")
	if err != domain.ErrRateLimited {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}
}

func TestAuthService_Logout_Success(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	// Login to get a token
	loginResult, err := authService.Login("testuser", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Logout
	err = authService.Logout(loginResult.Token)
	if err != nil {
		t.Fatalf("expected successful logout, got error: %v", err)
	}

	// Verify session is revoked
	_, err = authService.GetCurrentUser(loginResult.Token)
	if err == nil {
		t.Fatal("expected error after logout, got nil")
	}
}

func TestAuthService_GetCurrentUser_Success(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	loginResult, err := authService.Login("testuser", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	currentUser, err := authService.GetCurrentUser(loginResult.Token)
	if err != nil {
		t.Fatalf("expected to get current user, got error: %v", err)
	}
	if currentUser.Login != "testuser" {
		t.Fatalf("expected login 'testuser', got '%s'", currentUser.Login)
	}
}

func TestAuthService_GetCurrentUser_DeactivatedUser(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "deactivated",
		DisplayName:  "Deactivated User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true, // Start active to allow login
	}
	db.Create(&user)

	loginResult, err := authService.Login("deactivated", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Deactivate the user
	db.Model(&user).Update("is_active", false)

	// Try to get current user
	_, err = authService.GetCurrentUser(loginResult.Token)
	if err != domain.ErrUserDeactivated {
		t.Fatalf("expected ErrUserDeactivated, got: %v", err)
	}
}

func TestAuthService_GetCurrentUser_ExpiredSession(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	// Login to get a session
	loginResult, err := authService.Login("testuser", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Expire the session
	db.Model(&domain.Session{}).Where("user_id = ?", user.ID).Update("expires_at", time.Now().Add(-1*time.Hour))

	// Try to get current user - should fail
	_, err = authService.GetCurrentUser(loginResult.Token)
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
}

func TestAuthService_ChangePassword_Success(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	// Login
	loginResult, err := authService.Login("testuser", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Change password
	err = authService.ChangePassword(user.ID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("expected successful password change, got error: %v", err)
	}

	// Verify old password no longer works
	_, err = authService.Login("testuser", "password123", "127.0.0.1", "test-agent")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected old password to fail, got: %v", err)
	}

	// Verify new password works
	_, err = authService.Login("testuser", "newpassword456", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("expected new password to work, got error: %v", err)
	}

	// Verify old session is revoked (ChangePassword revokes all sessions)
	_, err = authService.GetCurrentUser(loginResult.Token)
	if err == nil {
		t.Fatal("expected old session to be revoked")
	}
}

func TestAuthService_ChangePassword_WrongOldPassword(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	db.Create(&user)

	err = authService.ChangePassword(user.ID, "wrongpassword", "newpassword456")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for wrong old password, got: %v", err)
	}
}

func TestAuthService_AdminResetPassword_Success(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	// Create admin
	adminHash, _ := HashPassword("adminpass")
	admin := domain.User{
		Login:        "admin",
		DisplayName:  "Admin",
		Role:         domain.RoleAdmin,
		PasswordHash: adminHash,
		IsActive:     true,
	}
	db.Create(&admin)

	// Create regular user
	userHash, _ := HashPassword("userpass")
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: userHash,
		IsActive:     true,
	}
	db.Create(&user)

	// Login as admin
	adminLogin, _ := authService.Login("admin", "adminpass", "127.0.0.1", "test-agent")

	// Admin resets user password
	err := authService.AdminResetPassword(adminLogin.User.ID, user.ID, "resetpass123")
	if err != nil {
		t.Fatalf("expected successful admin reset, got error: %v", err)
	}

	// Verify new password works
	_, err = authService.Login("testuser", "resetpass123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("expected reset password to work, got error: %v", err)
	}
}

func TestAuthService_AdminResetPassword_NonAdmin(t *testing.T) {
	db, authService, _, _, _ := setupAuthService(t)

	// Create regular user
	userHash, _ := HashPassword("userpass")
	user := domain.User{
		Login:        "testuser",
		DisplayName:  "Test User",
		Role:         domain.RoleUser,
		PasswordHash: userHash,
		IsActive:     true,
	}
	db.Create(&user)

	// Create another regular user to be the target
	targetHash, _ := HashPassword("targetpass")
	target := domain.User{
		Login:        "target",
		DisplayName:  "Target User",
		Role:         domain.RoleUser,
		PasswordHash: targetHash,
		IsActive:     true,
	}
	db.Create(&target)

	// Regular user tries to reset another user's password
	err := authService.AdminResetPassword(user.ID, target.ID, "newpass123")
	if err != domain.ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-admin, got: %v", err)
	}
}
