package config

import (
	"encoding/base64"
	"testing"
)

func validKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestLoadRequiresMasterKey(t *testing.T) {
	t.Setenv("REGISTRY_MASTER_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error when REGISTRY_MASTER_KEY is missing")
	}
}

func TestLoadRejectsWrongKeyLength(t *testing.T) {
	t.Setenv("REGISTRY_MASTER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a 16-byte master key")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("REGISTRY_MASTER_KEY", validKey())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":5176" {
		t.Fatalf("HTTPAddr = %q, want :5176", cfg.HTTPAddr)
	}
	if cfg.ClientURL != "http://127.0.0.1:2379" {
		t.Fatalf("ClientURL = %q", cfg.ClientURL)
	}
	if cfg.PeerURL != "http://127.0.0.1:2380" {
		t.Fatalf("PeerURL = %q", cfg.PeerURL)
	}
}

func TestValidateRejectsPartialTLS(t *testing.T) {
	cfg := &Config{MasterKey: validKey(), TLSCertFile: "cert.pem"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected an error when only the TLS cert is set")
	}
}

func TestValidateRequiresRootPasswordWithAuth(t *testing.T) {
	cfg := &Config{MasterKey: validKey(), AuthEnabled: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected an error when auth is enabled without a root password")
	}
}
