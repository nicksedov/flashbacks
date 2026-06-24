package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/flashbacks/api-service/internal/application/auth"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthHandlers contains all authentication-related handlers
type AuthHandlers struct {
	authService *auth.AuthService
	bootstrap   *auth.BootstrapService
	userService *auth.UserService
	sessionRepo *auth.SessionRepository
	db          *gorm.DB
	i18n        *i18n.Service
}

// NewAuthHandlers creates a new auth handlers instance
func NewAuthHandlers(authService *auth.AuthService, bootstrap *auth.BootstrapService, userService *auth.UserService, sessionRepo *auth.SessionRepository, db *gorm.DB, i18nSvc *i18n.Service) *AuthHandlers {
	return &AuthHandlers{
		authService: authService,
		bootstrap:   bootstrap,
		userService: userService,
		sessionRepo: sessionRepo,
		db:          db,
		i18n:        i18nSvc,
	}
}

// respondSuccess sends a success response with the message translated to the user's language
func (h *AuthHandlers) respondSuccess(c *gin.Context, code int, msg i18n.MessageKey, data ...interface{}) {
	lang := middleware.GetLanguage(c)
	resp := i18n.SuccessResponseResolved(h.i18n, msg, lang, data...)
	c.JSON(code, resp)
}

// respondError sends an error response with the message translated to the user's language
func (h *AuthHandlers) respondError(c *gin.Context, code int, msg i18n.MessageKey) {
	lang := middleware.GetLanguage(c)
	c.JSON(code, i18n.ErrorResponseResolved(h.i18n, msg, lang))
}

// handleAuthStatus returns the current authentication status
func (h *AuthHandlers) handleAuthStatus(c *gin.Context) {
	isBootstrap, err := h.bootstrap.IsBootstrapMode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	// Try to get user from session
	user := middleware.GetCurrentUser(c)
	if user != nil {
		// Re-fetch with avatar to correctly set hasAvatar flag
		userWithAvatar, err := h.userService.GetUserWithAvatar(user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"isAuthenticated": true,
			"isBootstrapMode": false,
			"user":            dto.ToUserDTO(userWithAvatar),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isAuthenticated": false,
		"isBootstrapMode": isBootstrap,
	})
}

// handleLogin authenticates a user and creates a session
func (h *AuthHandlers) handleLogin(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidCredentials))
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	result, err := h.authService.Login(req.Login, req.Password, ipAddress, userAgent)
	if err != nil {
		if err == domain.ErrRateLimited {
			c.JSON(http.StatusTooManyRequests, i18n.ErrorResponse(i18n.MsgAuthRateLimited))
			return
		}
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthInvalidCredentials))
		return
	}

	if result.IsBootstrap {
		// Bootstrap login - return bootstrap session info
		config := h.sessionRepo.GetSessionConfig()
		c.SetCookie(middleware.SessionCookieName, "bootstrap", config.CookieMaxAge, "/", "", true, true)
		c.JSON(http.StatusOK, gin.H{
			"isBootstrap": true,
			"message":     i18n.MsgAuthBootstrapMode,
		})
		return
	}

	// Set session cookie
	config := h.sessionRepo.GetSessionConfig()
	c.SetCookie(
		middleware.SessionCookieName,
		result.Token,
		config.CookieMaxAge,
		"/",
		"",
		true, // secure - requires HTTPS (set false for dev, true in prod)
		true, // httpOnly - not accessible via JS
	)

	// Create audit log
	auth.CreateAuditLog(h.db, &result.User.ID, domain.ActionLogin, "user", &result.User.ID, fmt.Sprintf(`{"ip": "%s"}`, ipAddress))

	c.JSON(http.StatusOK, gin.H{
		"user": dto.ToUserDTO(result.User),
	})
}

// handleLogout revokes the current session
func (h *AuthHandlers) handleLogout(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user != nil {
		token, _ := c.Cookie(middleware.SessionCookieName)
		h.authService.Logout(token)
		auth.CreateAuditLog(h.db, &user.ID, domain.ActionLogout, "user", &user.ID, "")
	}

	// Clear cookie
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{"message": i18n.MsgAuthLogoutSuccess})
}

// handleMe returns the current user's profile
func (h *AuthHandlers) handleMe(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	// Re-fetch with avatar to correctly set hasAvatar flag
	userWithAvatar, err := h.userService.GetUserWithAvatar(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": dto.ToUserDTO(userWithAvatar),
	})
}

// handleChangePassword changes the current user's password
func (h *AuthHandlers) handleChangePassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	// Validate new password length
	if len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthPasswordLength))
		return
	}

	if err := h.authService.ChangePassword(user.ID, req.OldPassword, req.NewPassword); err != nil {
		if err == domain.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthInvalidCurrentPassword))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthPasswordChangeFailed))
		return
	}

	auth.CreateAuditLog(h.db, &user.ID, domain.ActionChangePassword, "user", &user.ID, "")

	c.JSON(http.StatusOK, gin.H{
		"message":   i18n.Success,
		"mustLogin": true,
	})
}

// handleBootstrapSetup completes the bootstrap initialization
func (h *AuthHandlers) handleBootstrapSetup(c *gin.Context) {
	var req dto.BootstrapSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	// Validate password
	if len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthPasswordLength))
		return
	}

	// Create admin user
	user, err := h.bootstrap.CreateBootstrapAdmin(req.NewPassword, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthBootstrapFailed))
		return
	}

	// Revoke bootstrap cookie and create real session
	token, err := h.sessionRepo.CreateSession(user.ID, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthSessionCreationFailed))
		return
	}

	// Set proper session cookie
	config := h.sessionRepo.GetSessionConfig()
	c.SetCookie(
		middleware.SessionCookieName,
		token,
		config.CookieMaxAge,
		"/",
		"",
		true,
		true,
	)

	// Audit log
	auth.CreateAuditLog(h.db, &user.ID, domain.ActionBootstrapComplete, "system", nil, `{"admin_login": "`+user.Login+`"}`)

	c.JSON(http.StatusOK, gin.H{
		"user":    dto.ToUserDTO(user),
		"message": i18n.MsgAuthBootstrapComplete,
	})
}

// --- Admin Handlers ---

// handleListUsers returns all users (admin only)
func (h *AuthHandlers) handleListUsers(c *gin.Context) {
	users, err := h.userService.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthUsersListFailed))
		return
	}

	userDTOs := make([]dto.UserDTO, len(users))
	for i, u := range users {
		userDTOs[i] = dto.ToUserDTO(&u)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": userDTOs,
		"total": len(userDTOs),
	})
}

// handleCreateUser creates a new user (admin only)
func (h *AuthHandlers) handleCreateUser(c *gin.Context) {
	admin := middleware.GetCurrentUser(c)

	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	// Validate password length
	if len(req.Password) < 8 || len(req.Password) > 128 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthPasswordLength))
		return
	}

	// Validate role
	if req.Role != domain.RoleAdmin && req.Role != domain.RoleUser {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRole))
		return
	}

	input := &auth.CreateUserInput{
		Login:       req.Login,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		Password:    req.Password,
	}

	user, err := h.userService.CreateUser(admin.ID, input)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgUserServiceUserExists))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthUserCreated))
		return
	}

	auth.CreateAuditLog(h.db, &admin.ID, domain.ActionCreateUser, "user", &user.ID, fmt.Sprintf(`{"login": "%s", "role": "%s"}`, user.Login, user.Role))

	c.JSON(http.StatusCreated, gin.H{
		"user":    dto.ToUserDTO(user),
		"message": i18n.MsgAuthUserCreated,
	})
}

// handleUpdateUser updates a user (admin only)
func (h *AuthHandlers) handleUpdateUser(c *gin.Context) {
	admin := middleware.GetCurrentUser(c)

	id := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(id, "%d", &userID); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthUserNotFound))
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	input := &auth.UpdateUserInput{
		DisplayName: req.DisplayName,
		Role:        req.Role,
		IsActive:    req.IsActive,
	}

	user, err := h.userService.UpdateUser(admin.ID, userID, input)
	if err != nil {
		if strings.Contains(err.Error(), "last admin") {
			if strings.Contains(err.Error(), "demote") {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgUserServiceLastAdminDemote))
			} else if strings.Contains(err.Error(), "deactivate") {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgUserServiceLastAdminDeactivate))
			} else {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgUserServiceLastAdminDelete))
			}
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthUserUpdateFailed))
		return
	}

	// Audit
	action := domain.ActionUpdateUser
	if input.IsActive != nil && !*input.IsActive {
		action = domain.ActionDeactivateUser
	} else if input.IsActive != nil && *input.IsActive {
		action = domain.ActionActivateUser
	}
	auth.CreateAuditLog(h.db, &admin.ID, action, "user", &user.ID, "")

	c.JSON(http.StatusOK, gin.H{
		"user":    dto.ToUserDTO(user),
		"message": i18n.MsgAuthUserUpdated,
	})
}

// handleDeleteUser deletes a user (admin only)
func (h *AuthHandlers) handleDeleteUser(c *gin.Context) {
	admin := middleware.GetCurrentUser(c)

	id := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(id, "%d", &userID); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthUserNotFound))
		return
	}

	if err := h.userService.DeleteUser(admin.ID, userID); err != nil {
		if strings.Contains(err.Error(), "last admin") {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgUserServiceLastAdminDelete))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthUserDeleteFailed))
		return
	}

	auth.CreateAuditLog(h.db, &admin.ID, domain.ActionDeleteUser, "user", &userID, "")

	c.JSON(http.StatusOK, gin.H{"message": i18n.MsgAuthUserDeleted})
}

// handleResetPassword resets a user's password (admin only)
func (h *AuthHandlers) handleResetPassword(c *gin.Context) {
	admin := middleware.GetCurrentUser(c)

	id := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(id, "%d", &userID); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthUserNotFound))
		return
	}

	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	if len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthPasswordLength))
		return
	}

	if err := h.authService.AdminResetPassword(admin.ID, userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthPasswordResetFailed))
		return
	}

	auth.CreateAuditLog(h.db, &admin.ID, domain.ActionResetPassword, "user", &userID, "")

	c.JSON(http.StatusOK, gin.H{"message": i18n.MsgAuthPasswordResetSuccess})
}

// handleUpdateProfile updates the current user's profile
func (h *AuthHandlers) handleUpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	_, err := h.userService.UpdateProfile(user.ID, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthProfileUpdateFailed))
		return
	}

	// Re-fetch with avatar to correctly set hasAvatar flag
	updatedUser, err := h.userService.GetUserWithAvatar(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthProfileUpdateFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": dto.ToUserDTO(updatedUser),
	})
}

// handleUploadAvatar uploads and processes the current user's avatar
func (h *AuthHandlers) handleUploadAvatar(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	// Parse multipart form (max 10MB)
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAvatarTooLarge))
		return
	}

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAvatarInvalidType))
		return
	}
	defer file.Close()

	// Validate content type
	contentType := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !allowedTypes[contentType] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAvatarInvalidType))
		return
	}

	// Read file bytes
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarUploadFailed))
		return
	}

	if err := h.userService.UpdateAvatar(user.ID, fileBytes); err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAvatarTooLarge))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarUploadFailed))
		return
	}

	// Return updated user with hasAvatar=true
	updatedUser, err := h.userService.GetUserWithAvatar(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarUploadFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": dto.ToUserDTO(updatedUser),
	})
}

// handleDeleteAvatar removes the current user's avatar
func (h *AuthHandlers) handleDeleteAvatar(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	if err := h.userService.DeleteAvatar(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarDeleteFailed))
		return
	}

	// Return updated user with hasAvatar=false
	updatedUser, err := h.userService.GetUserWithAvatar(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarDeleteFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": dto.ToUserDTO(updatedUser),
	})
}

// handleGetAvatar serves a user's avatar image
func (h *AuthHandlers) handleGetAvatar(c *gin.Context) {
	id := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(id, "%d", &userID); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAvatarNotFound))
		return
	}

	avatar, err := h.userService.GetAvatar(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgAvatarNotFound))
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "image/webp", avatar)
}

// handleAuditLogs returns audit logs (admin only)
func (h *AuthHandlers) handleAuditLogs(c *gin.Context) {
	page := 1
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}

	logs, total, err := auth.ListAuditLogs(h.db, page, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthAuditLogsFailed))
		return
	}

	dtoLogs := make([]dto.AuditLogDTO, len(logs))
	for i, log := range logs {
		dtoLogs[i] = dto.ToAuditLogDTO(&log)
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  dtoLogs,
		"total": total,
		"page":  page,
	})
}
