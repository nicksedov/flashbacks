package domain

import (
	"errors"
	"fmt"
	"testing"
	"unicode"
)

// authErrorSentinels lists every authentication sentinel so the tests below can
// guarantee they stay distinct, non-empty and identity-matchable.
func authErrorSentinels() []*AuthError {
	return []*AuthError{
		ErrInvalidCredentials,
		ErrUserDeactivated,
		ErrRateLimited,
		ErrForbidden,
		ErrAccountPending,
		ErrAccountRejected,
		ErrRegistrationDisabled,
		ErrUserNotPending,
		ErrBootstrapMode,
		ErrInvalidRequestFormat,
		ErrPasswordLength,
		ErrUserExists,
	}
}

func TestAuthErrorSentinelsAreDistinctAndNonEmpty(t *testing.T) {
	seen := make(map[*AuthError]bool, len(authErrorSentinels()))

	for i, sentinel := range authErrorSentinels() {
		if sentinel == nil {
			t.Fatalf("sentinel #%d is nil", i)
		}
		if sentinel.Error() == "" {
			t.Errorf("sentinel #%d has an empty message", i)
		}
		if seen[sentinel] {
			t.Errorf("sentinel #%d (%q) is registered twice", i, sentinel.Error())
		}
		seen[sentinel] = true
	}
}

// TestAuthErrorValueDoesNotImplementError guards the linter fix: Error has a
// pointer receiver, so only *AuthError satisfies error and comparison is never
// ambiguous between the value and the pointer form.
func TestAuthErrorValueDoesNotImplementError(t *testing.T) {
	var err error = ErrForbidden

	if _, ok := err.(*AuthError); !ok {
		t.Fatalf("sentinel type = %T, want *AuthError", err)
	}
}

func TestAuthErrorMatchingByIdentity(t *testing.T) {
	wrapped := fmt.Errorf("login failed: %w", ErrUserDeactivated)

	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{name: "direct match", err: ErrForbidden, target: ErrForbidden, want: true},
		{name: "wrapped match", err: wrapped, target: ErrUserDeactivated, want: true},
		{name: "different sentinel", err: ErrForbidden, target: ErrInvalidCredentials, want: false},
		{name: "unwrapped pointer from another sentinel", err: ErrAccountPending, target: ErrAccountRejected, want: false},
		{name: "nil error", err: nil, target: ErrForbidden, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, tt.target); got != tt.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		})
	}
}

// TestAuthErrorMessagesAreEnglish enforces the repository-wide "English only"
// rule for code-level strings: translated user-facing text belongs to the i18n
// locale files, not to domain error messages.
func TestAuthErrorMessagesAreEnglish(t *testing.T) {
	for i, sentinel := range authErrorSentinels() {
		for _, r := range sentinel.Error() {
			if r > unicode.MaxASCII {
				t.Errorf("sentinel #%d (%q) contains the non-ASCII character %q", i, sentinel.Error(), r)
				break
			}
		}
	}
}

// TestAuthErrorCopyIsNotSentinel documents that matching happens by pointer
// identity: a dereferenced copy carries the same message but is not the sentinel,
// so sentinels must always be returned as-is (never copied or recreated).
func TestAuthErrorCopyIsNotSentinel(t *testing.T) {
	copied := *ErrForbidden

	if errors.Is(&copied, ErrForbidden) {
		t.Error("expected a copy to be a different error identity than ErrForbidden")
	}
	if copied.Error() != ErrForbidden.Error() {
		t.Errorf("copy message = %q, want %q", copied.Error(), ErrForbidden.Error())
	}
}
