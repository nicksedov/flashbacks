package config

import (
	"os"
	"testing"
)

// unsetEnv removes the given environment variables for the duration of the test
// and restores the original values on cleanup. t.Setenv cannot delete variables.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()

	type saved struct {
		value string
		set   bool
	}
	originals := make(map[string]saved, len(keys))

	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		originals[key] = saved{value: value, set: ok}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to unset %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for key, original := range originals {
			if original.set {
				_ = os.Setenv(key, original.value)
				continue
			}
			_ = os.Unsetenv(key)
		}
	})
}

func TestLoadDefaults(t *testing.T) {
	unsetEnv(t,
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"EXIF_HOST", "EXIF_PORT", "EXIFTOOL_POOL_SIZE", "EXIF_LOG_LEVEL", "LOG_LEVEL",
	)

	cfg := Load()

	if cfg.ServerHost != "0.0.0.0" {
		t.Errorf("ServerHost = %q, want %q", cfg.ServerHost, "0.0.0.0")
	}
	if cfg.ServerPort != "5172" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "5172")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.DBName != "image_toolkit" {
		t.Errorf("DBName = %q, want %q", cfg.DBName, "image_toolkit")
	}
}

func TestLoadServerOverrides(t *testing.T) {
	t.Setenv("EXIF_HOST", "127.0.0.1")
	t.Setenv("EXIF_PORT", "6002")

	cfg := Load()

	if cfg.ServerHost != "127.0.0.1" {
		t.Errorf("ServerHost = %q, want %q", cfg.ServerHost, "127.0.0.1")
	}
	if cfg.ServerPort != "6002" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "6002")
	}
}

func TestLoadLogLevel(t *testing.T) {
	tests := []struct {
		name         string
		exifLogLevel string
		legacyLevel  string
		want         string
	}{
		{name: "defaults to info", want: "info"},
		{name: "reads EXIF_LOG_LEVEL", exifLogLevel: "debug", want: "debug"},
		{
			name:         "EXIF_LOG_LEVEL wins over LOG_LEVEL",
			exifLogLevel: "debug",
			legacyLevel:  "warn",
			want:         "debug",
		},
		{name: "falls back to legacy LOG_LEVEL", legacyLevel: "warn", want: "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// An empty value means "not set" here, so unset instead of t.Setenv("").
			setOrUnset := func(key, value string) {
				if value == "" {
					unsetEnv(t, key)
					return
				}
				t.Setenv(key, value)
			}
			setOrUnset("EXIF_LOG_LEVEL", tt.exifLogLevel)
			setOrUnset("LOG_LEVEL", tt.legacyLevel)

			if got := Load().LogLevel; got != tt.want {
				t.Fatalf("LogLevel = %q, want %q", got, tt.want)
			}
		})
	}
}
