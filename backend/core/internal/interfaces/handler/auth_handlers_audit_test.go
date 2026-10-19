package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/application/auth"
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/interfaces/i18n"
	"github.com/flashbacks/core/internal/interfaces/middleware"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newAuthHandlerTestEnv builds AuthHandlers backed by an in-memory database.
func newAuthHandlerTestEnv(t *testing.T, mode string) (*AuthHandlers, *gorm.DB, func()) {
	t.Helper()

	db, cleanup := testutil.NewTestDB(t)
	sessionConfig := &auth.SessionConfig{
		IdleTimeout:     30 * 24 * time.Hour,
		AbsoluteTimeout: 90 * 24 * time.Hour,
		CookieMaxAge:    30 * 24 * 60 * 60,
		TokenLength:     64,
	}
	sessionRepo := auth.NewSessionRepository(db, sessionConfig)
	bootstrap := auth.NewBootstrapService(db, "bootstrap_admin", "bootstrap123")
	loginLimiter := auth.NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	registerLimiter := auth.NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	authService := auth.NewAuthService(db, bootstrap, sessionRepo, loginLimiter, registerLimiter, mode)
	userService := auth.NewUserService(db, sessionRepo)

	i18nSvc, err := i18n.NewService()
	if err != nil {
		cleanup()
		t.Fatalf("failed to load i18n service: %v", err)
	}

	return NewAuthHandlers(authService, bootstrap, userService, sessionRepo, db, i18nSvc), db, cleanup
}

// serveAuthHandler runs an auth handler with an optional authenticated admin and route id.
func serveAuthHandler(
	h gin.HandlerFunc,
	admin *domain.User,
	method, target string,
	routeID uint,
	body any,
) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic("failed to encode request body: " + err.Error())
		}
	}

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(routeID), 10)}}
	if admin != nil {
		ctx.Set(middleware.ContextKeyUser, admin)
	}
	h(ctx)
	return w
}

func countAuditLogs(t *testing.T, db *gorm.DB, action domain.AuditAction) []domain.AuditLog {
	t.Helper()
	var logs []domain.AuditLog
	if err := db.Where("action = ?", action).Find(&logs).Error; err != nil {
		t.Fatalf("failed to query %s audit logs: %v", action, err)
	}
	return logs
}

func TestHandleRegister_RecordsAuditEntry(t *testing.T) {
	handlers, db, cleanup := newAuthHandlerTestEnv(t, domain.AccountCreationModeSelfService)
	defer cleanup()

	// Leave bootstrap mode so registration is allowed.
	testutil.SeedUserWithHash(t, db, "existing", "Existing", domain.RoleUser, true, "hash")

	w := serveAuthHandler(handlers.handleRegister, nil, http.MethodPost, "/api/auth/register", 0, map[string]any{
		"login":       "newuser",
		"displayName": "New User",
		"password":    "password123",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	logs := countAuditLogs(t, db, domain.ActionRegisterUser)
	if len(logs) != 1 {
		t.Fatalf("expected one register_user audit entry, got %d", len(logs))
	}
	if logs[0].TargetType != "user" || logs[0].TargetID == nil {
		t.Errorf("register_user target = %q/%v, want user with an id", logs[0].TargetType, logs[0].TargetID)
	}
	if logs[0].ActorUserID == nil || *logs[0].ActorUserID != *logs[0].TargetID {
		t.Errorf("register_user actor = %v, want the new user id %v", logs[0].ActorUserID, logs[0].TargetID)
	}
}

func TestHandleApproveUser_RecordsAuditEntry(t *testing.T) {
	handlers, db, cleanup := newAuthHandlerTestEnv(t, domain.AccountCreationModeSelfServiceWithApproval)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, db, "admin", "Admin", domain.RoleAdmin, true, "hash")
	pending := testutil.SeedUserWithHash(t, db, "pending", "Pending", domain.RoleUser, true, "hash")
	if err := db.Model(pending).Update("account_status", domain.AccountStatusPending).Error; err != nil {
		t.Fatalf("failed to mark the user pending: %v", err)
	}

	w := serveAuthHandler(handlers.handleApproveUser, admin, http.MethodPost, "/api/admin/users/1/approve", pending.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	logs := countAuditLogs(t, db, domain.ActionApproveUser)
	if len(logs) != 1 {
		t.Fatalf("expected one approve_user audit entry, got %d", len(logs))
	}
	if logs[0].ActorUserID == nil || *logs[0].ActorUserID != admin.ID {
		t.Errorf("approve_user actor = %v, want admin %d", logs[0].ActorUserID, admin.ID)
	}
	if logs[0].TargetID == nil || *logs[0].TargetID != pending.ID {
		t.Errorf("approve_user target = %v, want %d", logs[0].TargetID, pending.ID)
	}
}

func TestHandleRejectUser_RecordsAuditEntry(t *testing.T) {
	handlers, db, cleanup := newAuthHandlerTestEnv(t, domain.AccountCreationModeSelfServiceWithApproval)
	defer cleanup()

	admin := testutil.SeedUserWithHash(t, db, "admin", "Admin", domain.RoleAdmin, true, "hash")
	pending := testutil.SeedUserWithHash(t, db, "pending", "Pending", domain.RoleUser, true, "hash")
	if err := db.Model(pending).Update("account_status", domain.AccountStatusPending).Error; err != nil {
		t.Fatalf("failed to mark the user pending: %v", err)
	}

	w := serveAuthHandler(handlers.handleRejectUser, admin, http.MethodPost, "/api/admin/users/1/reject", pending.ID, map[string]any{
		"reason": "Duplicate account",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	logs := countAuditLogs(t, db, domain.ActionRejectUser)
	if len(logs) != 1 {
		t.Fatalf("expected one reject_user audit entry, got %d", len(logs))
	}
	if logs[0].ActorUserID == nil || *logs[0].ActorUserID != admin.ID {
		t.Errorf("reject_user actor = %v, want admin %d", logs[0].ActorUserID, admin.ID)
	}
	if logs[0].TargetID == nil || *logs[0].TargetID != pending.ID {
		t.Errorf("reject_user target = %v, want %d", logs[0].TargetID, pending.ID)
	}

	var reloaded domain.User
	if err := db.First(&reloaded, pending.ID).Error; err != nil {
		t.Fatalf("failed to reload the rejected user: %v", err)
	}
	if reloaded.AccountStatus != domain.AccountStatusRejected || reloaded.RejectionReason != "Duplicate account" {
		t.Errorf(
			"persisted status = %q reason = %q, want %q/%q",
			reloaded.AccountStatus,
			reloaded.RejectionReason,
			domain.AccountStatusRejected,
			"Duplicate account",
		)
	}
	if reloaded.StatusChangedAt == nil || reloaded.StatusChangedBy == nil {
		t.Errorf("expected statusChangedAt and statusChangedBy to be persisted, got %v/%v", reloaded.StatusChangedAt, reloaded.StatusChangedBy)
	}
}
