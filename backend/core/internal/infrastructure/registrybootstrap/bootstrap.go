// Package registrybootstrap wires the core service to the registry facade.
package registrybootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	registryclient "github.com/flashbacks/registry/client"
)

// ShutdownTimeout bounds best-effort deregistration.
const ShutdownTimeout = 3 * time.Second

// Bootstrap carries the optional registry client for this process.
type Bootstrap struct {
	client *registryclient.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// FromEnv creates a bootstrap from REGISTRY_URL, SERVICE_NAME, and SERVICE_ID.
// An empty REGISTRY_URL disables registry integration and keeps env config.
func FromEnv(service string) *Bootstrap {
	ctx, cancel := context.WithCancel(context.Background())
	endpoint := strings.TrimSpace(os.Getenv("REGISTRY_URL"))
	if endpoint == "" {
		slog.Info("registry integration disabled", "service", service)
		cancel()
		return &Bootstrap{ctx: ctx, cancel: cancel}
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
		cancel()
		return &Bootstrap{ctx: ctx, cancel: cancel}
	}
	return &Bootstrap{client: client, ctx: ctx, cancel: cancel}
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

// Secret reads a secret value, returning ok=false when unavailable.
func (b *Bootstrap) Secret(ctx context.Context, service, key string) (string, bool) {
	if !b.Enabled() {
		return "", false
	}
	value, err := b.client.GetSecret(ctx, service, key)
	if err != nil {
		slog.Warn("registry secret unavailable", "service", service, "key", key, "error", err)
		return "", false
	}
	return value, true
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

// Close stops background discovery watchers.
func (b *Bootstrap) Close() {
	if b == nil || b.cancel == nil {
		return
	}
	b.cancel()
}

// Resolver exposes a peer base URL that follows registry changes.
type Resolver struct {
	mu  sync.RWMutex
	url string
}

// URL returns the current peer base URL.
func (r *Resolver) URL() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.url
}

func (r *Resolver) set(url string) {
	r.mu.Lock()
	r.url = url
	r.mu.Unlock()
}

// NewResolver resolves a peer once and keeps it updated while the service runs.
func (b *Bootstrap) NewResolver(service, bootstrapURL string) *Resolver {
	resolver := &Resolver{url: bootstrapURL}
	if !b.Enabled() {
		return resolver
	}

	resolveCtx, cancel := context.WithTimeout(b.ctx, 3*time.Second)
	resolver.set(b.client.ResolvePeer(resolveCtx, service, bootstrapURL))
	cancel()

	go func() {
		err := b.client.WatchInstances(b.ctx, service, func(event registryclient.Event) {
			if event.Type != "put" && event.Type != "delete" {
				return
			}
			refreshCtx, refreshCancel := context.WithTimeout(b.ctx, 3*time.Second)
			resolver.set(b.client.ResolvePeer(refreshCtx, service, bootstrapURL))
			refreshCancel()
		})
		if err != nil && b.ctx.Err() == nil {
			slog.Warn("registry instance watch stopped", "service", service, "error", err)
		}
	}()

	return resolver
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
