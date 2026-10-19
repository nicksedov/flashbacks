// Package testutil starts an isolated registry stack for tests.
package testutil

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flashbacks/registry/client"
	"github.com/flashbacks/registry/internal/config"
	"github.com/flashbacks/registry/internal/httpapi"
	"github.com/flashbacks/registry/internal/registry"
	"github.com/flashbacks/registry/internal/secrets"
	"github.com/flashbacks/registry/internal/store"
)

// Stack is a running registry stack backed by an embedded etcd instance.
type Stack struct {
	Embedded *registry.Embedded
	Server   *httptest.Server
	Store    store.Store
	Codec    *secrets.Codec
}

// Start launches embedded etcd and the facade on random local ports.
func Start(tb testing.TB) *Stack {
	tb.Helper()

	clientPort := freePort(tb)
	peerPort := freePort(tb)
	masterKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg := config.Config{
		DataDir:      tb.TempDir(),
		ClientURL:    fmt.Sprintf("http://127.0.0.1:%d", clientPort),
		PeerURL:      fmt.Sprintf("http://127.0.0.1:%d", peerPort),
		MasterKey:    masterKey,
		ReadyTimeout: 30 * time.Second,
	}

	embedded, err := registry.Start(context.Background(), cfg)
	if err != nil {
		tb.Fatalf("start embedded etcd: %v", err)
	}
	tb.Cleanup(func() { _ = embedded.Close() })

	codec, err := secrets.NewFromBase64(masterKey)
	if err != nil {
		tb.Fatalf("create secrets codec: %v", err)
	}

	st := store.NewEtcdStore(embedded.Client())
	api := httpapi.New(st, codec, "")
	server := httptest.NewServer(api.Handler())
	tb.Cleanup(server.Close)

	return &Stack{Embedded: embedded, Server: server, Store: st, Codec: codec}
}

// NewClient creates a registry client bound to this stack.
func (s *Stack) NewClient(tb testing.TB, service, instanceID string, ttl int64) *client.Client {
	tb.Helper()
	created, err := client.New(client.Config{
		Endpoint:    s.Server.URL,
		ServiceName: service,
		InstanceID:  instanceID,
		TTLSeconds:  ttl,
	})
	if err != nil {
		tb.Fatalf("create registry client: %v", err)
	}
	return created
}

func freePort(tb testing.TB) int {
	tb.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
