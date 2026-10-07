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
	ctx := c.Request.Context()
	isBootstrap, err := h.bootstrap.IsBootstrapMode(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	// Try to get user from session
	user := middleware.GetCurrentUser(c)
	if user != nil {
		// Re-fetch with avatar to correctly set hasAvatar flag
		userWithAvatar, err := h.userService.GetUserWithAvatar(ctx, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"isAuthenticated":     true,
			"isBootstrapMode":     false,
			"accountCreationMode": h.authService.AccountCreationMode(),
			"user":                dto.ToUserDTO(userWithAvatar),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isAuthenticated":     false,
		"isBootstrapMode":     isBootstrap,
		"accountCreationMode": h.authService.AccountCreationMode(),
	})
}

// handleLogin authenticates a user and creates a session
func (h *AuthHandlers) handleLogin(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidCredentials))
		return
	}

	ctx := c.Request.Context()
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	result, err := h.authService.Login(ctx, req.Login, req.Password, ipAddress, userAgent)
	if err != nil {
		switch err {
		case domain.ErrRateLimited:
			h.respondError(c, http.StatusTooManyRequests, i18n.MsgAuthRateLimited)
		case domain.ErrAccountPending:
			h.respondError(c, http.StatusForbidden, i18n.MsgAuthAccountPendingApproval)
		case domain.ErrAccountRejected:
			h.respondError(c, http.StatusForbidden, i18n.MsgAuthAccountRejected)
		case domain.ErrUserDeactivated:
			h.respondError(c, http.StatusUnauthorized, i18n.MsgAuthAccountDeactivated)
		default:
			h.respondError(c, http.StatusUnauthorized, i18n.MsgAuthInvalidCredentials)
		}
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

// handleRegister handles self-service registration (public endpoint)
func (h *AuthHandlers) handleRegister(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthInvalidRequestFormat)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Login) == "" || strings.TrimSpace(req.DisplayName) == "" {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthInvalidRequestFormat)
		return
	}

	// Validate password length
	if len(req.Password) < 8 || len(req.Password) > 128 {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthPasswordLength)
		return
	}

	ctx := c.Request.Context()
	result, err := h.authService.RegisterUser(ctx, req.Login, req.DisplayName, req.Password, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		switch err {
		case domain.ErrRegistrationDisabled:
			h.respondError(c, http.StatusForbidden, i18n.MsgAuthRegistrationDisabled)
		case domain.ErrBootstrapMode:
			h.respondError(c, http.StatusConflict, i18n.MsgAuthBootstrapMode)
		case domain.ErrRateLimited:
			h.respondError(c, http.StatusTooManyRequests, i18n.MsgAuthRateLimited)
		case domain.ErrUserExists:
			h.respondError(c, http.StatusBadRequest, i18n.MsgUserServiceUserExists)
		case domain.ErrPasswordLength:
			h.respondError(c, http.StatusBadRequest, i18n.MsgAuthPasswordLength)
		case domain.ErrInvalidRequestFormat:
			h.respondError(c, http.StatusBadRequest, i18n.MsgAuthInvalidRequestFormat)
		default:
			h.respondError(c, http.StatusInternalServerError, i18n.MsgAuthInternalError)
		}
		return
	}

	lang := middleware.GetLanguage(c)

	if result.Pending {
		// Account created but awaiting admin approval - no session is issued
		resp := i18n.SuccessResponseResolved(h.i18n, i18n.MsgAuthRegistrationPending, lang)
		resp["pending"] = true
		c.JSON(http.StatusCreated, resp)
		return
	}

	// self_service mode: issue a session cookie (auto-login)
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

	resp := i18n.SuccessResponseResolved(h.i18n, i18n.MsgAuthRegistrationSuccess, lang)
	resp["user"] = dto.ToUserDTO(result.User)
	c.JSON(http.StatusCreated, resp)
}

// handleLogout revokes the current session
func (h *AuthHandlers) handleLogout(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user != nil {
		token, _ := c.Cookie(middleware.SessionCookieName)
		h.authService.Logout(c.Request.Context(), token)
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
	userWithAvatar, err := h.userService.GetUserWithAvatar(c.Request.Context(), user.ID)
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

	if err := h.authService.ChangePassword(c.Request.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
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

	ctx := c.Request.Context()
	// Create admin user
	user, err := h.bootstrap.CreateBootstrapAdmin(ctx, req.NewPassword, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthBootstrapFailed))
		return
	}

	// Revoke bootstrap cookie and create real session
	token, err := h.sessionRepo.CreateSession(ctx, user.ID, c.ClientIP(), c.GetHeader("User-Agent"))
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

// handleListUsers returns all users (admin only), optionally filtered by
// account status via the "status" query parameter (e.g. status=pending).
func (h *AuthHandlers) handleListUsers(c *gin.Context) {
	status := c.Query("status")
	users, err := h.userService.ListUsers(c.Request.Context(), status)
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

	user, err := h.userService.CreateUser(c.Request.Context(), admin.ID, input)
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

	user, err := h.userService.UpdateUser(c.Request.Context(), admin.ID, userID, input)
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

	if err := h.userService.DeleteUser(c.Request.Context(), admin.ID, userID); err != nil {
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

	if err := h.authService.AdminResetPassword(c.Request.Context(), admin.ID, userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthPasswordResetFailed))
		return
	}

	auth.CreateAuditLog(h.db, &admin.ID, domain.ActionResetPassword, "user", &userID, "")

	c.JSON(http.StatusOK, gin.H{"message": i18n.MsgAuthPasswordResetSuccess})
}

// handleApproveUser approves a pending user account (admin only)
func (h *AuthHandlers) handleApproveUser(c *gin.Context) {
	admin := middleware.GetCurrentUser(c)

	userID, ok := parseUserID(c)
	if !ok {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthUserNotFound)
		return
	}

	user, err := h.userService.ApproveUser(c.Request.Context(), admin.ID, userID)
	if err != nil {
		if err == domain.ErrUserNotPending {
			h.respondError(c, http.StatusConflict, i18n.MsgAuthUserNotPending)
			return
		}
		h.respondError(c, http.StatusInternalServerError, i18n.MsgAuthUserUpdateFailed)
		return
	}

	auth.CreateAuditLog(h.db, &admin.ID, domain.ActionApproveUser, "user", &user.ID, "")

	lang := middleware.GetLanguage(c)
	resp := i18n.SuccessResponseResolved(h.i18n, i18n.MsgAuthUserApproved, lang)
	resp["user"] = dto.ToUserDTO(user)
	c.JSON(http.StatusOK, resp)
}

// handleRejectUser rejects a pending user account (admin only)
func (h *AuthHandlers) handleRejectUser(c *gin.Context) {
	admin := middleware.GetCurrentUser(c)

	userID, ok := parseUserID(c)
	if !ok {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthUserNotFound)
		return
	}

	var req dto.RejectUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthInvalidRequestFormat)
		return
	}

	// Rejection reason is required
	if strings.TrimSpace(req.Reason) == "" {
		h.respondError(c, http.StatusBadRequest, i18n.MsgAuthInvalidRequestFormat)
		return
	}

	user, err := h.userService.RejectUser(c.Request.Context(), admin.ID, userID, req.Reason)
	if err != nil {
		if err == domain.ErrUserNotPending {
			h.respondError(c, http.StatusConflict, i18n.MsgAuthUserNotPending)
			return
		}
		h.respondError(c, http.StatusInternalServerError, i18n.MsgAuthUserUpdateFailed)
		return
	}

	auth.CreateAuditLog(h.db, &admin.ID, domain.ActionRejectUser, "user", &user.ID, "")

	lang := middleware.GetLanguage(c)
	resp := i18n.SuccessResponseResolved(h.i18n, i18n.MsgAuthUserRejected, lang)
	resp["user"] = dto.ToUserDTO(user)
	c.JSON(http.StatusOK, resp)
}

// handleUpdateProfile updates the current user's profile
func (h *AuthHandlers) handleUpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
		return
	}

	_, err := h.userService.UpdateProfile(c.Request.Context(), user.ID, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthProfileUpdateFailed))
		return
	}

	// Re-fetch with avatar to correctly set hasAvatar flag
	updatedUser, err := h.userService.GetUserWithAvatar(c.Request.Context(), user.ID)
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

	if err := h.userService.UpdateAvatar(c.Request.Context(), user.ID, fileBytes); err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAvatarTooLarge))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarUploadFailed))
		return
	}

	// Return updated user with hasAvatar=true
	updatedUser, err := h.userService.GetUserWithAvatar(c.Request.Context(), user.ID)
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

	if err := h.userService.DeleteAvatar(c.Request.Context(), user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAvatarDeleteFailed))
		return
	}

	// Return updated user with hasAvatar=false
	updatedUser, err := h.userService.GetUserWithAvatar(c.Request.Context(), user.ID)
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

	avatar, err := h.userService.GetAvatar(c.Request.Context(), userID)
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

// parseUserID parses the ":id" route parameter into a uint.
func parseUserID(c *gin.Context) (uint, bool) {
	var userID uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &userID); err != nil {
		return 0, false
	}
	return userID, true
}
