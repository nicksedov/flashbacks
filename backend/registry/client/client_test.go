package client_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flashbacks/registry/client"
	"github.com/flashbacks/registry/internal/testutil"
)

// quietLogger discards client logs so tests stay readable.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeJSONBody(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestConfigAndSecretRoundTrip(t *testing.T) {
	stack := testutil.Start(t)
	c := stack.NewClient(t, "core", "core-1", 15)
	ctx := context.Background()

	if err := c.SetConfig(ctx, "core", "db_host", "db.internal"); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	values, err := c.GetConfig(ctx, "core")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if values["db_host"] != "db.internal" {
		t.Fatalf("GetConfig() = %v", values)
	}

	if err := c.PutSecret(ctx, "core", "db_password", "s3cret"); err != nil {
		t.Fatalf("PutSecret() error = %v", err)
	}
	secret, err := c.GetSecret(ctx, "core", "db_password")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if secret != "s3cret" {
		t.Fatalf("GetSecret() = %q", secret)
	}
}

func TestRegisterAndDiscover(t *testing.T) {
	stack := testutil.Start(t)
	c := stack.NewClient(t, "exif", "exif-1", 5)
	ctx := context.Background()

	if err := c.Register(ctx, client.Instance{Host: "127.0.0.1", Port: 5172, BasePath: "/exif"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer func() { _ = c.Deregister(context.Background()) }()

	instances, err := c.Discover(ctx, "exif")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(instances) != 1 || instances[0].Port != 5172 || instances[0].Host != "127.0.0.1" {
		t.Fatalf("Discover() = %+v", instances)
	}
}

func TestResolvePeerFallsBack(t *testing.T) {
	c, err := client.New(client.Config{
		Endpoint:    "http://127.0.0.1:1",
		ServiceName: "core",
		InstanceID:  "core-1",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := c.ResolvePeer(context.Background(), "exif", "http://bootstrap:5172")
	if got != "http://bootstrap:5172" {
		t.Fatalf("ResolvePeer() = %q, want bootstrap URL", got)
	}
}

func TestResolveConfigLayering(t *testing.T) {
	stack := testutil.Start(t)
	c := stack.NewClient(t, "core", "core-1", 15)
	ctx := context.Background()

	if err := c.SetConfig(ctx, "core", "db_host", "registry-host"); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	if err := c.SetConfig(ctx, "core", "scan_workers", "4"); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	env := map[string]string{"db_host": "env-host"}
	defaults := map[string]string{
		"db_host":      "default-host",
		"scan_workers": "2",
		"server_port":  "5170",
	}
	resolved := c.ResolveConfig(ctx, "core", defaults, func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})

	if resolved["db_host"] != "env-host" {
		t.Fatalf("db_host = %q, want env override", resolved["db_host"])
	}
	if resolved["scan_workers"] != "4" {
		t.Fatalf("scan_workers = %q, want registry value", resolved["scan_workers"])
	}
	if resolved["server_port"] != "5170" {
		t.Fatalf("server_port = %q, want default", resolved["server_port"])
	}
}

func TestResolveConfigFallsBackWhenRegistryUnavailable(t *testing.T) {
	c, err := client.New(client.Config{
		Endpoint:    "http://127.0.0.1:1",
		ServiceName: "core",
		InstanceID:  "core-1",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resolved := c.ResolveConfig(context.Background(), "core", map[string]string{"db_host": "default-host"}, nil)
	if resolved["db_host"] != "default-host" {
		t.Fatalf("db_host = %q, want default", resolved["db_host"])
	}
}

func TestWatchConfigDeliversUpdates(t *testing.T) {
	stack := testutil.Start(t)
	c := stack.NewClient(t, "core", "core-1", 15)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ready := make(chan struct{})
	events := make(chan client.Event, 8)
	var once sync.Once

	go func() {
		_ = c.WatchConfig(ctx, "core", func(event client.Event) {
			if event.Type == "ready" {
				once.Do(func() { close(ready) })
				return
			}
			events <- event
		})
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not become ready")
	}

	if err := c.SetConfig(ctx, "core", "db_host", "watched"); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	select {
	case event := <-events:
		if event.Type != "put" || !strings.HasSuffix(event.Key, "/db_host") || event.Value != "watched" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no watch event received")
	}
}

func TestWatchInstancesDeliversUpdates(t *testing.T) {
	stack := testutil.Start(t)
	watcher := stack.NewClient(t, "core", "core-1", 15)
	registrar := stack.NewClient(t, "exif", "exif-1", 5)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ready := make(chan struct{})
	events := make(chan client.Event, 8)
	var once sync.Once

	go func() {
		_ = watcher.WatchInstances(ctx, "exif", func(event client.Event) {
			if event.Type == "ready" {
				once.Do(func() { close(ready) })
				return
			}
			events <- event
		})
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("instance watch did not become ready")
	}

	if err := registrar.Register(ctx, client.Instance{Host: "127.0.0.1", Port: 5172}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer func() { _ = registrar.Deregister(context.Background()) }()

	select {
	case event := <-events:
		if event.Type != "put" || !strings.Contains(event.Key, "/services/exif/") {
			t.Fatalf("unexpected instance event: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no instance event received")
	}
}

func TestRegisterHeartbeatKeepsRegistration(t *testing.T) {
	stack := testutil.Start(t)
	c := stack.NewClient(t, "exif", "exif-heartbeat", 2)
	ctx := context.Background()

	if err := c.Register(ctx, client.Instance{Host: "127.0.0.1", Port: 5172}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer func() { _ = c.Deregister(context.Background()) }()

	time.Sleep(3500 * time.Millisecond)

	instances, err := c.Discover(ctx, "exif")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("registration expired despite heartbeats: %+v", instances)
	}
}

// TestRegisterRetriesUntilFacadeIsReady covers services that start together
// with the registry (for example after a Docker daemon restart): a facade that
// is not ready yet must not leave the instance undiscoverable forever.
func TestRegisterRetriesUntilFacadeIsReady(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/registry/v1/services/") {
			if attempts.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			writeJSONBody(w, `{"leaseId":42,"ttl":15}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	c, err := client.New(client.Config{
		Endpoint:        server.URL,
		ServiceName:     "ocr",
		InstanceID:      "ocr",
		TTLSeconds:      15,
		RetryBackoff:    5 * time.Millisecond,
		MaxRetryBackoff: 20 * time.Millisecond,
		Logger:          quietLogger(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Register(ctx, client.Instance{Host: "flashbacks-ocr", Port: 5174}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer func() { _ = c.Deregister(context.Background()) }()

	if got, want := attempts.Load(), int32(3); got != want {
		t.Fatalf("registration attempts = %d, want %d", got, want)
	}
}

// TestRegisterContinuesInBackgroundAfterTimeout covers a facade that stays down
// past the synchronous window: Register reports the error but the instance still
// joins discovery later, without requiring a process restart.
func TestRegisterContinuesInBackgroundAfterTimeout(t *testing.T) {
	var ready atomic.Bool
	var succeeded atomic.Int32
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/registry/v1/services/"):
			attempts.Add(1)
			if !ready.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			succeeded.Add(1)
			writeJSONBody(w, `{"leaseId":11,"ttl":15}`)
		case r.Method == http.MethodDelete:
			writeJSONBody(w, `{"status":"ok"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, err := client.New(client.Config{
		Endpoint:        server.URL,
		ServiceName:     "ocr",
		InstanceID:      "ocr",
		TTLSeconds:      15,
		RetryBackoff:    5 * time.Millisecond,
		MaxRetryBackoff: 10 * time.Millisecond,
		RegisterTimeout: 20 * time.Millisecond,
		Logger:          quietLogger(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Register(ctx, client.Instance{Host: "flashbacks-ocr", Port: 5174}); err == nil {
		t.Fatal("Register() = nil, want the deferred registration error")
	}
	defer func() { _ = c.Deregister(context.Background()) }()

	// The facade becomes reachable only after the synchronous window expired.
	ready.Store(true)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if succeeded.Load() >= 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("successful registrations = %d after %d attempts, want a background retry to succeed",
		succeeded.Load(), attempts.Load())
}

// TestHeartbeatReRegistersAfterLeaseLoss verifies a lost lease is recovered
// instead of silently dropping the instance from discovery.
func TestHeartbeatReRegistersAfterLeaseLoss(t *testing.T) {
	var registers atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			writeJSONBody(w, fmt.Sprintf(`{"leaseId":%d,"ttl":1}`, registers.Add(1)))
		case strings.HasSuffix(r.URL.Path, "/heartbeat"):
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodDelete:
			writeJSONBody(w, `{"status":"ok"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, err := client.New(client.Config{
		Endpoint:    server.URL,
		ServiceName: "ocr",
		InstanceID:  "ocr",
		TTLSeconds:  1,
		Logger:      quietLogger(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := c.Register(context.Background(), client.Instance{Host: "flashbacks-ocr", Port: 5174}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer func() { _ = c.Deregister(context.Background()) }()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if registers.Load() >= 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("registrations = %d, want a re-registration after heartbeat failures", registers.Load())
}

// TestDeregisterSendsLeaseID guards against a stopping instance deleting the
// registration of a replacement that reused the same instance ID.
func TestDeregisterSendsLeaseID(t *testing.T) {
	var deleteQuery atomic.Value
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			writeJSONBody(w, `{"leaseId":7,"ttl":15}`)
		case r.Method == http.MethodDelete:
			deletes.Add(1)
			deleteQuery.Store(r.URL.RawQuery)
			writeJSONBody(w, `{"status":"ok"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, err := client.New(client.Config{
		Endpoint:    server.URL,
		ServiceName: "ocr",
		InstanceID:  "ocr",
		TTLSeconds:  15,
		Logger:      quietLogger(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := c.Register(context.Background(), client.Instance{Host: "flashbacks-ocr", Port: 5174}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := c.Deregister(context.Background()); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	if got, want := deleteQuery.Load(), "leaseId=7"; got != want {
		t.Fatalf("delete query = %v, want %q", got, want)
	}
	if got := deletes.Load(); got != 1 {
		t.Fatalf("delete requests = %d, want 1", got)
	}
}

// TestDeregisterWithoutRegistrationIsNoOp makes sure a process that never
// registered cannot remove somebody else's registration.
func TestDeregisterWithoutRegistrationIsNoOp(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer server.Close()

	c, err := client.New(client.Config{
		Endpoint:    server.URL,
		ServiceName: "ocr",
		InstanceID:  "ocr",
		Logger:      quietLogger(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := c.Deregister(context.Background()); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("registry requests = %d, want 0", got)
	}
}

func TestResolvePeerWarnsWhenNothingDiscovered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/services/ocr" {
			http.NotFound(w, r)
			return
		}
		writeJSONBody(w, `{"instances":[]}`)
	}))
	defer server.Close()

	var logs bytes.Buffer
	c, err := client.New(client.Config{
		Endpoint:    server.URL,
		ServiceName: "core",
		InstanceID:  "core",
		Logger:      slog.New(slog.NewTextHandler(&logs, nil)),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := c.ResolvePeer(context.Background(), "ocr", "http://localhost:5174")
	if want := "http://localhost:5174"; got != want {
		t.Fatalf("ResolvePeer() = %q, want %q", got, want)
	}
	if !strings.Contains(logs.String(), "no live instances discovered") {
		t.Fatalf("expected an undiscovered-peer warning, got %q", logs.String())
	}
}
