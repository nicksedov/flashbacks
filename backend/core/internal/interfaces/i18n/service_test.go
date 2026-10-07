package i18n

import (
	"testing"
)

func TestNewService(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test English messages
	tests := []struct {
		key      MessageKey
		lang     string
		expected string
	}{
		{MsgAuthInvalidCredentials, "en", "Invalid login or password"},
		{MsgAuthInvalidCredentials, "ru", "Неверный логин или пароль"},
		{MsgScanStarted, "en", "Scan started"},
		{MsgScanStarted, "ru", "Сканирование начато"},
		{MsgFolderAdded, "en", "Folder added to gallery"},
		{MsgFolderAdded, "ru", "Папка добавлена в галерею"},
		{Success, "en", "Success"},
		{Success, "ru", "Успех"},
	}

	for _, tt := range tests {
		t.Run(string(tt.key)+"_"+tt.lang, func(t *testing.T) {
			got := svc.GetMessage(tt.key, tt.lang)
			if got != tt.expected {
				t.Errorf("GetMessage(%q, %q) = %q, want %q", tt.key, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestGetMessageFallback(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test fallback to English for unsupported language
	msg := svc.GetMessage(MsgAuthInvalidCredentials, "fr")
	expected := "Invalid login or password"
	if msg != expected {
		t.Errorf("GetMessage fallback = %q, want %q", msg, expected)
	}

	// Test fallback to key itself for missing key
	msg = svc.GetMessage(MessageKey("nonexistent.key"), "en")
	expected = "nonexistent.key"
	if msg != expected {
		t.Errorf("GetMessage missing key = %q, want %q", msg, expected)
	}
}
