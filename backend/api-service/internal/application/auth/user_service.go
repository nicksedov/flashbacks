package auth

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"time"

	"github.com/flashbacks/api-service/internal/domain"

	"github.com/deepteams/webp"
	"github.com/disintegration/imaging"
	"gorm.io/gorm"
)

// UserService handles user management operations
type UserService struct {
	db          *gorm.DB
	sessionRepo *SessionRepository
}

// NewUserService creates a new user service
func NewUserService(db *gorm.DB, sessionRepo *SessionRepository) *UserService {
	return &UserService{
		db:          db,
		sessionRepo: sessionRepo,
	}
}

// CreateUserInput contains the data needed to create a user
type CreateUserInput struct {
	Login       string          `json:"login" binding:"required"`
	DisplayName string          `json:"displayName" binding:"required"`
	Role        domain.UserRole `json:"role" binding:"required"`
	Password    string          `json:"password" binding:"required"`
}

// CreateUser creates a new user (admin action)
func (s *UserService) CreateUser(adminID uint, input *CreateUserInput) (*domain.User, error) {
	// Verify admin exists
	var admin domain.User
	if err := s.db.First(&admin, adminID).Error; err != nil {
		return nil, err
	}
	if admin.Role != domain.RoleAdmin {
		return nil, domain.ErrForbidden
	}

	// Validate role
	if input.Role != domain.RoleAdmin && input.Role != domain.RoleUser {
		return nil, errors.New("invalid role")
	}

	// Validate password length (min 8, max 128)
	if len(input.Password) < 8 || len(input.Password) > 128 {
		return nil, errors.New("password must be between 8 and 128 characters")
	}

	// Check if login already exists
	var existing domain.User
	if err := s.db.Where("login = ?", input.Login).First(&existing).Error; err == nil {
		return nil, errors.New("user with this login already exists")
	}

	// Hash password
	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := domain.User{
		Login:              input.Login,
		DisplayName:        input.DisplayName,
		Role:               input.Role,
		PasswordHash:       passwordHash,
		IsActive:           true,
		MustChangePassword: true,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id uint) (*domain.User, error) {
	var user domain.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers returns all users (avatar excluded for performance)
func (s *UserService) ListUsers() ([]domain.User, error) {
	var users []domain.User
	if err := s.db.Omit("avatar").Order("created_at desc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUserInput contains updatable user fields
type UpdateUserInput struct {
	DisplayName *string          `json:"displayName"`
	Role        *domain.UserRole `json:"role"`
	IsActive    *bool            `json:"isActive"`
}

// UpdateUser updates a user (admin action)
func (s *UserService) UpdateUser(adminID, userID uint, input *UpdateUserInput) (*domain.User, error) {
	// Verify admin exists
	var admin domain.User
	if err := s.db.First(&admin, adminID).Error; err != nil {
		return nil, err
	}
	if admin.Role != domain.RoleAdmin {
		return nil, domain.ErrForbidden
	}

	var user domain.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	// Prevent modifying the last admin
	if input.Role != nil && *input.Role != domain.RoleAdmin && user.Role == domain.RoleAdmin {
		adminCount, err := s.countAdmins()
		if err != nil {
			return nil, err
		}
		if adminCount <= 1 {
			return nil, errors.New("cannot demote the last admin")
		}
	}

	// Prevent deactivating the last admin
	if input.IsActive != nil && !*input.IsActive && user.Role == domain.RoleAdmin {
		adminCount, err := s.countAdmins()
		if err != nil {
			return nil, err
		}
		if adminCount <= 1 {
			return nil, errors.New("cannot deactivate the last admin")
		}
	}

	updates := make(map[string]interface{})
	if input.DisplayName != nil {
		updates["display_name"] = *input.DisplayName
	}
	if input.Role != nil {
		updates["role"] = *input.Role
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if len(updates) > 0 {
		if err := s.db.Model(&user).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// If deactivated, revoke all sessions
	if input.IsActive != nil && !*input.IsActive {
		s.sessionRepo.RevokeAllUserSessions(user.ID)
	}

	return &user, nil
}

// DeleteUser deletes a user (admin action)
func (s *UserService) DeleteUser(adminID, userID uint) error {
	// Verify admin exists
	var admin domain.User
	if err := s.db.First(&admin, adminID).Error; err != nil {
		return err
	}
	if admin.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	var user domain.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return err
	}

	// Prevent deleting the last admin
	if user.Role == domain.RoleAdmin {
		adminCount, err := s.countAdmins()
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return errors.New("cannot delete the last admin")
		}
	}

	// Revoke all sessions
	s.sessionRepo.RevokeAllUserSessions(user.ID)

	// Delete user
	return s.db.Delete(&user).Error
}

// UpdateProfile updates the current user's own profile
func (s *UserService) UpdateProfile(userID uint, displayName string) (*domain.User, error) {
	var user domain.User
	if err := s.db.Omit("avatar").First(&user, userID).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&user).Update("display_name", displayName).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserWithAvatar retrieves a user including the avatar field
func (s *UserService) GetUserWithAvatar(id uint) (*domain.User, error) {
	var user domain.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateAvatar processes and stores a user's avatar image
func (s *UserService) UpdateAvatar(userID uint, imageData []byte) error {
	if len(imageData) == 0 {
		return errors.New("empty image data")
	}
	if len(imageData) > 10<<20 {
		return errors.New("image too large, maximum 10MB")
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize to 128x128
	resized := imaging.Fill(img, 128, 128, imaging.Center, imaging.Lanczos)

	// Encode as WebP
	var buf bytes.Buffer
	if err := webp.Encode(&buf, resized, &webp.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to encode avatar as WebP: %w", err)
	}

	return s.db.Model(&domain.User{}).Where("id = ?", userID).Update("avatar", buf.Bytes()).Error
}

// DeleteAvatar removes a user's avatar
func (s *UserService) DeleteAvatar(userID uint) error {
	return s.db.Model(&domain.User{}).Where("id = ?", userID).Update("avatar", nil).Error
}

// GetAvatar retrieves the avatar bytes for a user
func (s *UserService) GetAvatar(userID uint) ([]byte, error) {
	var user domain.User
	if err := s.db.Select("avatar").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if len(user.Avatar) == 0 {
		return nil, errors.New("avatar not found")
	}
	return user.Avatar, nil
}

// countAdmins returns the number of active admin users
func (s *UserService) countAdmins() (int64, error) {
	var count int64
	err := s.db.Model(&domain.User{}).Where("role = ? AND is_active = ?", domain.RoleAdmin, true).Count(&count).Error
	return count, err
}

// CreateAuditLog creates an audit log entry
func CreateAuditLog(db *gorm.DB, actorUserID *uint, action domain.AuditAction, targetType string, targetID *uint, meta string) error {
	log := domain.AuditLog{
		ActorUserID: actorUserID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Meta:        meta,
		CreatedAt:   time.Now(),
	}
	return db.Create(&log).Error
}

// ListAuditLogs returns audit logs with pagination
func ListAuditLogs(db *gorm.DB, page, pageSize int) ([]domain.AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	var total int64
	if err := db.Model(&domain.AuditLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []domain.AuditLog
	offset := (page - 1) * pageSize
	if err := db.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
