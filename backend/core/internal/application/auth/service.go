package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/domain"

	"gorm.io/gorm"
)

// AuthService handles authentication operations
type AuthService struct {
	db                  *gorm.DB
	bootstrap           *BootstrapService
	sessionRepo         *SessionRepository
	loginLimiter        *LoginRateLimiter
	registerLimiter     *LoginRateLimiter
	accountCreationMode string
}

// NewAuthService creates a new auth service
func NewAuthService(
	db *gorm.DB,
	bootstrap *BootstrapService,
	sessionRepo *SessionRepository,
	loginLimiter *LoginRateLimiter,
	registerLimiter *LoginRateLimiter,
	accountCreationMode string,
) *AuthService {
	return &AuthService{
		db:                  db,
		bootstrap:           bootstrap,
		sessionRepo:         sessionRepo,
		loginLimiter:        loginLimiter,
		registerLimiter:     registerLimiter,
		accountCreationMode: accountCreationMode,
	}
}

// LoginResult contains the result of a login attempt
type LoginResult struct {
	User        *domain.User
	Token       string
	IsBootstrap bool
}

// Login authenticates a user and creates a session
func (s *AuthService) Login(ctx context.Context, login, password, ipAddress, userAgent string) (*LoginResult, error) {
	// Check rate limiting
	if !s.loginLimiter.Allow(ipAddress) {
		return nil, domain.ErrRateLimited
	}

	// Check if in bootstrap mode
	isBootstrap, err := s.bootstrap.IsBootstrapMode(ctx)
	if err != nil {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, err
	}

	var user *domain.User

	if isBootstrap {
		// Validate against bootstrap credentials
		if !s.bootstrap.ValidateBootstrapCredentials(login, password) {
			s.loginLimiter.RecordFailure(ipAddress)
			return nil, domain.ErrInvalidCredentials
		}
		// Bootstrap login successful - user will create permanent account after
		result := &LoginResult{
			IsBootstrap: true,
		}
		return result, nil
	}

	// Normal user authentication (case-insensitive login lookup)
	normalizedLogin := strings.ToLower(strings.TrimSpace(login))
	if err := s.db.Where("lower(login) = ?", normalizedLogin).First(&user).Error; err != nil {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrInvalidCredentials
	}

	// Verify password BEFORE revealing account status to avoid login enumeration
	if !VerifyPassword(password, user.PasswordHash) {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrInvalidCredentials
	}

	// Check administrative deactivation
	if !user.IsActive {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrUserDeactivated
	}

	// Check account approval status
	switch user.AccountStatus {
	case domain.AccountStatusPending:
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrAccountPending
	case domain.AccountStatusRejected:
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrAccountRejected
	}

	// Rate limit success - reset counter
	s.loginLimiter.RecordSuccess(ipAddress)

	// Create session
	token, err := s.sessionRepo.CreateSession(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	// Update last login time
	now := time.Now()
	s.db.Model(&user).Update("last_login_at", now)

	return &LoginResult{
		User:        user,
		Token:       token,
		IsBootstrap: false,
	}, nil
}

// AccountCreationMode returns the configured account creation mode
func (s *AuthService) AccountCreationMode() string {
	return s.accountCreationMode
}

// RegisterResult contains the result of a self-service registration
type RegisterResult struct {
	User    *domain.User
	Token   string // session token; empty when the account awaits approval
	Pending bool   // true when the account was created with pending status
}

// RegisterUser creates a new user account via self-service registration.
// The resulting account status depends on the configured account creation mode:
//   - self_service: account is created as active and a session is issued
//   - self_service_with_approval: account is created as pending, no session
//   - admin_only: registration is disabled
//
// The role is always "user"; self-registration can never create an admin.
func (s *AuthService) RegisterUser(ctx context.Context, login, displayName, password, ipAddress, userAgent string) (*RegisterResult, error) {
	// Separate rate limiter for registration attempts
	if !s.registerLimiter.Allow(ipAddress) {
		return nil, domain.ErrRateLimited
	}

	// Registration is disabled in admin_only mode
	if s.accountCreationMode == domain.AccountCreationModeAdminOnly {
		return nil, domain.ErrRegistrationDisabled
	}

	// Registration is unavailable during bootstrap initialization
	isBootstrap, err := s.bootstrap.IsBootstrapMode(ctx)
	if err != nil {
		return nil, err
	}
	if isBootstrap {
		return nil, domain.ErrBootstrapMode
	}

	// Normalize login (trim + lowercase) for case-insensitive uniqueness
	login = strings.ToLower(strings.TrimSpace(login))
	displayName = strings.TrimSpace(displayName)

	// Validate required fields
	if login == "" || displayName == "" {
		return nil, domain.ErrInvalidRequestFormat
	}

	// Validate password length
	if len(password) < 8 || len(password) > 128 {
		return nil, domain.ErrPasswordLength
	}

	// Check login uniqueness (case-insensitive)
	var existing domain.User
	if err := s.db.Where("lower(login) = ?", login).First(&existing).Error; err == nil {
		s.registerLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrUserExists
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Determine the initial account status from the creation mode
	accountStatus := domain.AccountStatusActive
	if s.accountCreationMode == domain.AccountCreationModeSelfServiceWithApproval {
		accountStatus = domain.AccountStatusPending
	}

	now := time.Now()
	user := domain.User{
		Login:           login,
		DisplayName:     displayName,
		Role:            domain.RoleUser,
		AccountStatus:   accountStatus,
		PasswordHash:    passwordHash,
		IsActive:        true,
		StatusChangedAt: &now,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	targetID := user.ID
	if err := CreateAuditLog(s.db, &user.ID, domain.ActionRegisterUser, "user", &targetID, fmt.Sprintf(`{"ip": "%s"}`, ipAddress)); err != nil {
		return nil, fmt.Errorf("failed to write audit log: %w", err)
	}

	result := &RegisterResult{
		User:    &user,
		Pending: accountStatus == domain.AccountStatusPending,
	}

	// For self_service mode, auto-login: create a session for the new user
	if !result.Pending {
		token, err := s.sessionRepo.CreateSession(ctx, user.ID, ipAddress, userAgent)
		if err != nil {
			return nil, err
		}
		result.Token = token
		s.db.Model(&user).Update("last_login_at", time.Now())
	}

	s.registerLimiter.RecordSuccess(ipAddress)
	return result, nil
}

// Logout revokes a session
func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessionRepo.RevokeSession(ctx, token)
}

// GetCurrentUser retrieves the user associated with a session token
func (s *AuthService) GetCurrentUser(ctx context.Context, token string) (*domain.User, error) {
	session, err := s.sessionRepo.GetSession(ctx, token)
	if err != nil {
		return nil, err
	}

	// Update last seen
	s.sessionRepo.UpdateLastSeen(ctx, token)

	var user domain.User
	// Exclude avatar bytes to avoid loading ~10KB on every authenticated request
	if err := s.db.Omit("avatar").First(&user, session.UserID).Error; err != nil {
		return nil, err
	}

	if !user.IsActive || user.AccountStatus != domain.AccountStatusActive {
		// Revoke session if user is disabled or not yet approved
		s.sessionRepo.RevokeSession(ctx, token)
		return nil, domain.ErrUserDeactivated
	}

	return &user, nil
}

// ChangePassword changes a user's password and revokes all their sessions
func (s *AuthService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	var user domain.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return err
	}

	// Verify old password
	if !VerifyPassword(oldPassword, user.PasswordHash) {
		return domain.ErrInvalidCredentials
	}

	// Hash new password
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password
	if err := s.db.Model(&user).Updates(map[string]interface{}{
		"password_hash":        newHash,
		"must_change_password": false,
	}).Error; err != nil {
		return err
	}

	// Revoke all sessions
	return s.sessionRepo.RevokeAllUserSessions(ctx, userID)
}

// AdminResetPassword resets a user's password (admin action)
func (s *AuthService) AdminResetPassword(ctx context.Context, adminID, targetUserID uint, newPassword string) error {
	// Verify admin exists and has admin role
	var admin domain.User
	if err := s.db.First(&admin, adminID).Error; err != nil {
		return err
	}
	if admin.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	var user domain.User
	if err := s.db.First(&user, targetUserID).Error; err != nil {
		return err
	}

	// Hash new password
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password and set must_change_password
	if err := s.db.Model(&user).Updates(map[string]interface{}{
		"password_hash":        newHash,
		"must_change_password": true,
	}).Error; err != nil {
		return err
	}

	// Revoke all user sessions
	return s.sessionRepo.RevokeAllUserSessions(ctx, targetUserID)
}
