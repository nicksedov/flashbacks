package domain

import (
	"time"
)

// UserRole represents the role of a user
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// AccountStatus represents the approval lifecycle of a user account
type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"   // access is granted
	AccountStatusPending  AccountStatus = "pending"  // awaiting admin decision
	AccountStatusRejected AccountStatus = "rejected" // rejected, access denied
)

// Account creation mode constants (ACCOUNT_CREATION_MODE env variable)
const (
	AccountCreationModeSelfService             = "self_service"
	AccountCreationModeSelfServiceWithApproval = "self_service_with_approval"
	AccountCreationModeAdminOnly               = "admin_only"
)

// User represents a user account in the system
type User struct {
	ID                 uint          `gorm:"primaryKey" json:"id"`
	Login              string        `gorm:"uniqueIndex;size:255;not null" json:"login"`
	DisplayName        string        `gorm:"size:255;not null" json:"displayName"`
	Role               UserRole      `gorm:"size:50;not null;default:user" json:"role"`
	AccountStatus      AccountStatus `gorm:"size:50;not null;default:active" json:"accountStatus"`
	PasswordHash       string        `gorm:"not null" json:"-"`
	Avatar             []byte        `gorm:"type:bytea" json:"-"`
	IsActive           bool          `gorm:"default:true" json:"isActive"`
	MustChangePassword bool          `gorm:"default:false" json:"mustChangePassword"`
	StatusChangedAt    *time.Time    `json:"statusChangedAt"`
	StatusChangedBy    *uint         `json:"statusChangedBy"`
	RejectionReason    string        `gorm:"size:500" json:"rejectionReason"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
	LastLoginAt        *time.Time    `json:"lastLoginAt"`
}

// UserSettings represents user-specific application settings
type UserSettings struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex;not null" json:"userId"`
	Theme     string    `gorm:"default:light-purple;not null" json:"theme"`
	Language  string    `gorm:"default:en;not null" json:"language"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Session represents an active user session
type Session struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"index;not null" json:"userId"`
	SessionToken string     `gorm:"uniqueIndex;size:255;not null" json:"-"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastSeenAt   time.Time  `json:"lastSeenAt"`
	ExpiresAt    time.Time  `gorm:"index" json:"expiresAt"`
	IPAddress    string     `gorm:"size:45" json:"-"`
	UserAgent    string     `gorm:"size:500" json:"-"`
	RevokedAt    *time.Time `json:"-"`
}

// AuditAction represents the type of audit action
type AuditAction string

const (
	ActionLogin             AuditAction = "login"
	ActionLogout            AuditAction = "logout"
	ActionLoginFailed       AuditAction = "login_failed"
	ActionCreateUser        AuditAction = "create_user"
	ActionRegisterUser      AuditAction = "register_user"
	ActionApproveUser       AuditAction = "approve_user"
	ActionRejectUser        AuditAction = "reject_user"
	ActionUpdateUser        AuditAction = "update_user"
	ActionDeleteUser        AuditAction = "delete_user"
	ActionResetPassword     AuditAction = "reset_password"
	ActionChangePassword    AuditAction = "change_password"
	ActionDeactivateUser    AuditAction = "deactivate_user"
	ActionActivateUser      AuditAction = "activate_user"
	ActionBootstrapComplete AuditAction = "bootstrap_complete"
)

// AuditLog records security and administrative events
type AuditLog struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	ActorUserID *uint       `gorm:"index" json:"actorUserId"`
	Action      AuditAction `gorm:"size:50;not null" json:"action"`
	TargetType  string      `gorm:"size:100" json:"targetType"`
	TargetID    *uint       `json:"targetId"`
	Meta        string      `gorm:"type:jsonb" json:"meta"`
	CreatedAt   time.Time   `json:"createdAt"`
}

// AuthError represents authentication error types
type AuthError string

func (e AuthError) Error() string {
	return string(e)
}

const (
	ErrInvalidCredentials   AuthError = "Неверный логин или пароль"
	ErrUserDeactivated      AuthError = "Учётная запись деактивирована"
	ErrRateLimited          AuthError = "Слишком много попыток входа. Попробуйте позже"
	ErrForbidden            AuthError = "Недостаточно прав"
	ErrAccountPending       AuthError = "Учётная запись ожидает одобрения администратора"
	ErrAccountRejected      AuthError = "Учётная запись отклонена администратором"
	ErrRegistrationDisabled AuthError = "Самостоятельная регистрация отключена"
	ErrUserNotPending       AuthError = "Учётная запись не ожидает одобрения"
	ErrBootstrapMode        AuthError = "Режим первичной настройки"
	ErrInvalidRequestFormat AuthError = "Неверный формат запроса"
	ErrPasswordLength       AuthError = "Пароль должен содержать от 8 до 128 символов"
	ErrUserExists           AuthError = "Пользователь с таким логином уже существует"
)
