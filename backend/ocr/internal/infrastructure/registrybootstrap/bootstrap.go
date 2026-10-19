// Package registrybootstrap wires the ocr service to the registry facade.
package registrybootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	registryclient "github.com/flashbacks/registry/client"
)

// ShutdownTimeout bounds best-effort deregistration.
const ShutdownTimeout = 3 * time.Second

// Bootstrap carries the optional registry client for this process.
type Bootstrap struct {
	client *registryclient.Client
}

// FromEnv creates a bootstrap from REGISTRY_URL, SERVICE_NAME, and SERVICE_ID.
// An empty REGISTRY_URL disables registry integration and keeps env config.
func FromEnv(service string) *Bootstrap {
	endpoint := strings.TrimSpace(os.Getenv("REGISTRY_URL"))
	if endpoint == "" {
		slog.Info("registry integration disabled", "service", service)
		return &Bootstrap{}
	}
	instanceID := strings.TrimSpace(os.Getenv("SERVICE_ID"))
	if instanceID == "" {
		instanceID = defaultInstanceID(service)
	}
	client, err := registryclient.New(registryclient.Config{
		Endpoint:    endpoint,
		ServiceName: service,
		InstanceID:  instanceID,
		Token:       os.Getenv("REGISTRY_TOKEN"),
	})
	if err != nil {
		slog.Warn("registry client disabled", "service", service, "error", err)
		return &Bootstrap{}
	}
	return &Bootstrap{client: client}
}

// Enabled reports whether registry integration is active.
func (b *Bootstrap) Enabled() bool {
	return b != nil && b.client != nil
}

// ResolveConfig merges defaults, registry values, and explicit env overrides.
func (b *Bootstrap) ResolveConfig(ctx context.Context, service string, defaults map[string]string) map[string]string {
	if !b.Enabled() {
		return defaults
	}
	return b.client.ResolveConfig(ctx, service, defaults, os.LookupEnv)
}

// Register advertises this instance and starts its heartbeat.
func (b *Bootstrap) Register(ctx context.Context, service, host string, port int, basePath, healthPath, version string) {
	if !b.Enabled() {
		return
	}
	instance := registryclient.Instance{
		Host:       host,
		Port:       port,
		BasePath:   basePath,
		HealthPath: healthPath,
		Version:    version,
	}
	if err := b.client.Register(ctx, instance); err != nil {
		slog.Warn("registry registration failed", "service", service, "error", err)
	}
}

// Deregister removes the registration.
func (b *Bootstrap) Deregister(ctx context.Context) {
	if !b.Enabled() {
		return
	}
	_ = b.client.Deregister(ctx)
}

// AdvertiseHost returns the host peers should use to reach this service.
func AdvertiseHost() string {
	if host := strings.TrimSpace(os.Getenv("SERVICE_HOST")); host != "" {
		return host
	}
	return "localhost"
}

// PortInt parses a port string, returning fallback on failure.
func PortInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// Version returns the service version advertised to peers.
func Version() string {
	if version := strings.TrimSpace(os.Getenv("SERVICE_VERSION")); version != "" {
		return version
	}
	return "dev"
}

func defaultInstanceID(service string) string {
	host, _ := os.Hostname()
	if host == "" {
		host = "local"
	}
	return fmt.Sprintf("%s-%s-%d", service, host, os.Getpid())
}
