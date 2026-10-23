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

// AuthError is a sentinel authentication error.
//
// It is declared as a struct whose Error method has a pointer receiver, so only
// *AuthError implements the error interface. A value-receiver string type would
// have made both AuthError and *AuthError implement error, leaving it ambiguous
// whether callers compare by value or by pointer (and hiding mismatches between
// the two forms). The concrete sentinels are the package-level variables below
// and must be matched by identity (errors.Is or ==).
type AuthError struct {
	// msg is the developer/log-facing description of the error. User-facing
	// text is selected by the interfaces layer from i18n.Msg* keys, never from
	// this field.
	msg string
}

// Error implements the error interface. The pointer receiver is deliberate: it
// guarantees that only *AuthError (never a copy) satisfies error.
func (e *AuthError) Error() string {
	return e.msg
}

// Authentication error sentinels. Each one is a distinct *AuthError value, so
// errors can be matched with errors.Is or a direct == comparison.
//
// Messages are English, log-facing descriptions. Translated, user-facing text is
// resolved in the interfaces layer from the matching i18n.Msg* keys, never here.
var (
	// ErrInvalidCredentials is returned for a wrong login/password pair or a
	// wrong current password.
	ErrInvalidCredentials = &AuthError{msg: "invalid login or password"}
	// ErrUserDeactivated is returned when the account is deactivated.
	ErrUserDeactivated = &AuthError{msg: "account is deactivated"}
	// ErrRateLimited is returned when the caller exceeded the auth rate limit.
	ErrRateLimited = &AuthError{msg: "too many login attempts, please try again later"}
	// ErrForbidden is returned when the actor lacks the required role.
	ErrForbidden = &AuthError{msg: "insufficient permissions"}
	// ErrAccountPending is returned while the account awaits admin approval.
	ErrAccountPending = &AuthError{msg: "account is pending administrator approval"}
	// ErrAccountRejected is returned when the account was rejected by an admin.
	ErrAccountRejected = &AuthError{msg: "account has been rejected by an administrator"}
	// ErrRegistrationDisabled is returned when self-service sign-up is off.
	ErrRegistrationDisabled = &AuthError{msg: "self-service registration is disabled"}
	// ErrUserNotPending is returned when approving/rejecting a user that is not
	// pending.
	ErrUserNotPending = &AuthError{msg: "account is not awaiting approval"}
	// ErrBootstrapMode is returned when the action is unavailable while the
	// system is in bootstrap mode.
	ErrBootstrapMode = &AuthError{msg: "bootstrap mode is active"}
	// ErrInvalidRequestFormat is returned when required request fields are
	// missing or malformed.
	ErrInvalidRequestFormat = &AuthError{msg: "invalid request format"}
	// ErrPasswordLength is returned when the password violates the length rules.
	ErrPasswordLength = &AuthError{msg: "password must be between 8 and 128 characters"}
	// ErrUserExists is returned when the login is already taken.
	ErrUserExists = &AuthError{msg: "user with this login already exists"}
)

// compile-time assertion: only the pointer form implements error.
var _ error = (*AuthError)(nil)
