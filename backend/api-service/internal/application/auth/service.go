package auth

import (
	"context"
	"time"

	"github.com/flashbacks/api-service/internal/domain"

	"gorm.io/gorm"
)

// AuthService handles authentication operations
type AuthService struct {
	db           *gorm.DB
	bootstrap    *BootstrapService
	sessionRepo  *SessionRepository
	loginLimiter *LoginRateLimiter
}

// NewAuthService creates a new auth service
func NewAuthService(db *gorm.DB, bootstrap *BootstrapService, sessionRepo *SessionRepository, limiter *LoginRateLimiter) *AuthService {
	return &AuthService{
		db:           db,
		bootstrap:    bootstrap,
		sessionRepo:  sessionRepo,
		loginLimiter: limiter,
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

	// Normal user authentication
	if err := s.db.Where("login = ?", login).First(&user).Error; err != nil {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrInvalidCredentials
	}

	// Verify password
	if !VerifyPassword(password, user.PasswordHash) {
		s.loginLimiter.RecordFailure(ipAddress)
		return nil, domain.ErrInvalidCredentials
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

	if !user.IsActive {
		// Deactivate session if user is disabled
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
