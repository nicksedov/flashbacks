// Package config loads bootstrap configuration for the registry service.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the bootstrap settings required to start the registry service.
// Everything else (service configuration, registrations, and secrets) is stored
// in the embedded etcd instance.
type Config struct {
	// DataDir is the embedded etcd data directory.
	DataDir string
	// ClientURL is the etcd client endpoint used by the REST facade.
	ClientURL string
	// PeerURL is the etcd peer endpoint.
	PeerURL string
	// HTTPAddr is the listen address of the REST facade.
	HTTPAddr string
	// MasterKey is the base64-encoded 32-byte AES-256 master key.
	MasterKey string
	// AuthEnabled enables etcd authentication with a root user.
	AuthEnabled bool
	// RootPassword is the etcd root password when auth is enabled.
	RootPassword string
	// AuthToken protects the REST facade when it is not empty.
	AuthToken string
	// TLSCertFile and TLSKeyFile enable TLS when both are set.
	TLSCertFile string
	TLSKeyFile  string
	// TLSCAFile optionally pins the CA used to verify the TLS certificate.
	TLSCAFile string
	// ReadyTimeout bounds how long startup waits for etcd readiness.
	ReadyTimeout time.Duration
	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load reads the registry bootstrap configuration from the environment.
func Load() (*Config, error) {
	cfg := &Config{
		DataDir:         getEnv("REGISTRY_DATA_DIR", ".run/registry-data"),
		ClientURL:       getEnv("REGISTRY_CLIENT_URL", "http://127.0.0.1:2379"),
		PeerURL:         getEnv("REGISTRY_PEER_URL", "http://127.0.0.1:2380"),
		HTTPAddr:        getEnv("REGISTRY_HTTP_ADDR", ":5176"),
		MasterKey:       os.Getenv("REGISTRY_MASTER_KEY"),
		AuthEnabled:     getEnvBool("REGISTRY_AUTH_ENABLED", false),
		RootPassword:    os.Getenv("REGISTRY_ROOT_PASSWORD"),
		AuthToken:       os.Getenv("REGISTRY_AUTH_TOKEN"),
		TLSCertFile:     os.Getenv("REGISTRY_TLS_CERT_FILE"),
		TLSKeyFile:      os.Getenv("REGISTRY_TLS_KEY_FILE"),
		TLSCAFile:       os.Getenv("REGISTRY_TLS_CA_FILE"),
		ReadyTimeout:    getEnvDuration("REGISTRY_READY_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getEnvDuration("REGISTRY_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks invariants that would otherwise fail at runtime.
func (c *Config) Validate() error {
	if c.MasterKey == "" {
		return fmt.Errorf("REGISTRY_MASTER_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(c.MasterKey)
	if err != nil {
		return fmt.Errorf("REGISTRY_MASTER_KEY must be valid base64: %w", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("REGISTRY_MASTER_KEY must decode to 32 bytes, got %d", len(key))
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return fmt.Errorf("REGISTRY_TLS_CERT_FILE and REGISTRY_TLS_KEY_FILE must be set together")
	}
	if c.AuthEnabled && c.RootPassword == "" {
		return fmt.Errorf("REGISTRY_ROOT_PASSWORD is required when REGISTRY_AUTH_ENABLED=true")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
