package client_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flashbacks/registry/client"
	"github.com/flashbacks/registry/internal/testutil"
)

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
