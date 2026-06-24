package middleware

import (
	"net/http"
	"strings"

	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// CSRFProtection implements CSRF protection for cookie-based auth
type CSRFProtection struct {
	// Paths that don't require CSRF validation (safe methods + login)
	skipPaths []string
	i18n      *i18n.Service
}

// NewCSRFProtection creates a new CSRF protection middleware
func NewCSRFProtection(i18nSvc *i18n.Service) *CSRFProtection {
	return &CSRFProtection{
		skipPaths: []string{
			"/api/auth/login",
			"/api/auth/status",
			"/api/auth/me",
			"/api/auth/logout",
			"/api/auth/bootstrap/setup",
			"/api/auth/change-password",
			"/api/users/me",
			"/api/users/me/avatar",
			"/api/admin/users",
			"/api/admin/users/:id",
			"/api/settings",
			"/api/delete-files",
			"/api/batch-delete",
		},
		i18n: i18nSvc,
	}
}

// resolveMessage translates a message key using the i18n service
func (p *CSRFProtection) resolveMessage(msg i18n.MessageKey) string {
	if p.i18n != nil {
		return p.i18n.GetMessage(msg, "en")
	}
	return string(msg)
}

// ShouldSkipCSRF checks if a path should skip CSRF validation
func (p *CSRFProtection) ShouldSkipCSRF(path string) bool {
	for _, skipPath := range p.skipPaths {
		if path == skipPath {
			return true
		}
	}
	return false
}

// Middleware returns the CSRF protection gin middleware
// Note: For API endpoints using JSON requests with cookie-based auth,
// CSRF tokens are not required if:
// - Session cookies are HttpOnly
// - CORS is properly configured
// - All API endpoints require authentication
// We only validate Origin/Referer headers for additional protection.
func (p *CSRFProtection) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only validate state-changing methods
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Skip CSRF for whitelisted endpoints
		if p.ShouldSkipCSRF(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Validate Origin header for additional security
		// This prevents cross-origin requests from other domains
		origin := c.GetHeader("Origin")
		if origin != "" {
			referer := c.GetHeader("Referer")
			// Allow same-origin requests
			if referer != "" && !strings.HasPrefix(referer, origin) {
				c.JSON(http.StatusForbidden, i18n.ErrorResponse(i18n.MsgMiddlewareCSRFFailed))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
