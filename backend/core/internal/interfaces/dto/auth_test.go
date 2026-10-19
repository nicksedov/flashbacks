package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
)

func TestUserDTOAccountStatusFields(t *testing.T) {
	changedAt := time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)
	user := &domain.User{
		ID:              7,
		Login:           "alice",
		DisplayName:     "Alice Liddell",
		Role:            domain.RoleUser,
		AccountStatus:   domain.AccountStatusRejected,
		IsActive:        true,
		StatusChangedAt: &changedAt,
		RejectionReason: "Duplicate account",
		CreatedAt:       time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC),
	}

	payload, err := json.Marshal(ToUserDTO(user))
	if err != nil {
		t.Fatalf("failed to marshal UserDTO: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("failed to unmarshal UserDTO: %v", err)
	}

	if raw["accountStatus"] != string(domain.AccountStatusRejected) {
		t.Errorf("accountStatus = %v, want %q", raw["accountStatus"], domain.AccountStatusRejected)
	}
	if raw["statusChangedAt"] != "2026-10-01 12:30:00" {
		t.Errorf("statusChangedAt = %v, want the formatted timestamp", raw["statusChangedAt"])
	}
	if raw["rejectionReason"] != "Duplicate account" {
		t.Errorf("rejectionReason = %v, want %q", raw["rejectionReason"], "Duplicate account")
	}

	// The DTO must never leak credential material.
	for _, forbidden := range []string{"passwordHash", "password_hash", "PasswordHash"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("UserDTO unexpectedly exposes %q", forbidden)
		}
	}
}

func TestUserDTOAccountStatusFieldsEmpty(t *testing.T) {
	user := &domain.User{
		ID:            8,
		Login:         "bob",
		DisplayName:   "Bob",
		Role:          domain.RoleUser,
		AccountStatus: domain.AccountStatusActive,
		IsActive:      true,
	}

	payload, err := json.Marshal(ToUserDTO(user))
	if err != nil {
		t.Fatalf("failed to marshal UserDTO: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("failed to unmarshal UserDTO: %v", err)
	}

	if raw["accountStatus"] != string(domain.AccountStatusActive) {
		t.Errorf("accountStatus = %v, want %q", raw["accountStatus"], domain.AccountStatusActive)
	}
	if raw["statusChangedAt"] != nil {
		t.Errorf("statusChangedAt = %v, want null", raw["statusChangedAt"])
	}
	if raw["rejectionReason"] != "" {
		t.Errorf("rejectionReason = %v, want an empty string", raw["rejectionReason"])
	}
}

func TestAuditLogDTOFields(t *testing.T) {
	actor := uint(3)
	target := uint(9)
	log := &domain.AuditLog{
		ID:          11,
		ActorUserID: &actor,
		Action:      domain.ActionApproveUser,
		TargetType:  "user",
		TargetID:    &target,
		Meta:        `{"source":"admin"}`,
		CreatedAt:   time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	}

	payload, err := json.Marshal(ToAuditLogDTO(log))
	if err != nil {
		t.Fatalf("failed to marshal AuditLogDTO: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("failed to unmarshal AuditLogDTO: %v", err)
	}

	if raw["action"] != string(domain.ActionApproveUser) {
		t.Errorf("action = %v, want %q", raw["action"], domain.ActionApproveUser)
	}
	if raw["actorUserId"] != float64(3) {
		t.Errorf("actorUserId = %v, want 3", raw["actorUserId"])
	}
	if raw["targetId"] != float64(9) {
		t.Errorf("targetId = %v, want 9", raw["targetId"])
	}
	if raw["createdAt"] != "2026-10-01 09:00:00" {
		t.Errorf("createdAt = %v, want the formatted timestamp", raw["createdAt"])
	}
}
