package ocr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestServiceURLFollowsProvider(t *testing.T) {
	var mu sync.Mutex
	current := "http://flashbacks-ocr:5174"
	client := NewClientWithProvider(func() string {
		mu.Lock()
		defer mu.Unlock()
		return current
	})

	if got, want := client.ServiceURL(), "http://flashbacks-ocr:5174"; got != want {
		t.Fatalf("ServiceURL() = %q, want %q", got, want)
	}

	mu.Lock()
	current = "http://127.0.0.1:5174/"
	mu.Unlock()

	if got, want := client.ServiceURL(), "http://127.0.0.1:5174"; got != want {
		t.Fatalf("ServiceURL() after discovery update = %q, want %q", got, want)
	}
}

func TestCheckHealthUsesProviderURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ocr/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
}
