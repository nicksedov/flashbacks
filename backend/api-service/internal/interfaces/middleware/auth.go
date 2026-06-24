package middleware

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/application/auth"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

const (
	// SessionCookieName is the name of the session cookie
	SessionCookieName = "session_id"

	// Context keys for storing user in gin context
	ContextKeyUser   = "user"
	ContextKeyUserID = "user_id"
)

// AuthMiddleware extracts and validates the session from cookie
type AuthMiddleware struct {
	sessionRepo *auth.SessionRepository
	authService *auth.AuthService
	i18n        *i18n.Service
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(sessionRepo *auth.SessionRepository, authService *auth.AuthService, i18nSvc *i18n.Service) *AuthMiddleware {
	return &AuthMiddleware{
		sessionRepo: sessionRepo,
		authService: authService,
		i18n:        i18nSvc,
	}
}

// resolveMessage translates a message key using the i18n service, falling back to English if not set
func (m *AuthMiddleware) resolveMessage(msg i18n.MessageKey) string {
	if m.i18n != nil {
		return m.i18n.GetMessage(msg, "en") // Default to English for auth errors
	}
	return string(msg)
}

// RequireAuth validates the session and loads the user into context
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": m.resolveMessage(i18n.MsgMiddlewareUnauthorized)})
			c.Abort()
			return
		}

		user, err := m.authService.GetCurrentUser(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": m.resolveMessage(i18n.MsgMiddlewareUnauthorized)})
			c.Abort()
			return
		}

		// Set user in context
		c.Set(ContextKeyUser, user)
		c.Set(ContextKeyUserID, user.ID)

		c.Next()
	}
}

// RequireAdmin ensures the current user has admin role
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get(ContextKeyUser)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		user, ok := userVal.(*domain.User)
		if !ok || user.Role != domain.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCurrentUser retrieves the current user from gin context
func GetCurrentUser(c *gin.Context) *domain.User {
	userVal, exists := c.Get(ContextKeyUser)
	if !exists {
		return nil
	}
	user, ok := userVal.(*domain.User)
	if !ok {
		return nil
	}
	return user
}

// GetUserID retrieves the current user ID from gin context
func GetUserID(c *gin.Context) uint {
	userVal, exists := c.Get(ContextKeyUserID)
	if !exists {
		return 0
	}
	userID, ok := userVal.(uint)
	if !ok {
		return 0
	}
	return userID
}
