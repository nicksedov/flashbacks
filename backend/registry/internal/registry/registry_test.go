package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/flashbacks/registry/internal/config"
)

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startTestRegistry(t *testing.T) *Embedded {
	t.Helper()
	clientPort := freePort(t)
	peerPort := freePort(t)
	cfg := config.Config{
		DataDir:      t.TempDir(),
		ClientURL:    fmt.Sprintf("http://127.0.0.1:%d", clientPort),
		PeerURL:      fmt.Sprintf("http://127.0.0.1:%d", peerPort),
		MasterKey:    base64.StdEncoding.EncodeToString(make([]byte, 32)),
		ReadyTimeout: 30 * time.Second,
	}
	embedded, err := Start(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = embedded.Close() })
	return embedded
}

func TestEmbeddedStartsAndServesKV(t *testing.T) {
	embedded := startTestRegistry(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := "/flashbacks/test/health"
	want := "ok"
	if _, err := embedded.Client().Put(ctx, key, want); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	resp, err := embedded.Client().Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(resp.Kvs) != 1 || string(resp.Kvs[0].Value) != want {
		t.Fatalf("Get() returned %v", resp.Kvs)
	}
}

func TestEmbeddedAuthEnabled(t *testing.T) {
	clientPort := freePort(t)
	peerPort := freePort(t)
	cfg := config.Config{
		DataDir:      t.TempDir(),
		ClientURL:    fmt.Sprintf("http://127.0.0.1:%d", clientPort),
		PeerURL:      fmt.Sprintf("http://127.0.0.1:%d", peerPort),
		MasterKey:    base64.StdEncoding.EncodeToString(make([]byte, 32)),
		AuthEnabled:  true,
		RootPassword: "root-password",
		ReadyTimeout: 30 * time.Second,
	}
	embedded, err := Start(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Start() with auth error = %v", err)
	}
	t.Cleanup(func() { _ = embedded.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := embedded.Client().Put(ctx, "/flashbacks/test/auth", "ok"); err != nil {
		t.Fatalf("authenticated Put() error = %v", err)
	}
}
