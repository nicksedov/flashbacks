package registrybootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolverUsesDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/services/exif" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"instances": []map[string]any{
				{"id": "exif-1", "host": "127.0.0.1", "port": 5172},
			},
		})
	}))
	defer server.Close()

	t.Setenv("REGISTRY_URL", server.URL)
	t.Setenv("SERVICE_NAME", "core")
	t.Setenv("SERVICE_ID", "core-test")

	bootstrap := FromEnv("core")
	defer bootstrap.Close()

	resolver := bootstrap.NewResolver("exif", "http://bootstrap:5172")
	if got, want := resolver.URL(), "http://127.0.0.1:5172"; got != want {
		t.Fatalf("resolver URL = %q, want %q", got, want)
	}
}

// TestResolverFallsBackWhenNothingDiscovered guards the incident where a peer
// missing from discovery left core talking to its unreachable bootstrap URL.
func TestResolverFallsBackWhenNothingDiscovered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/services/ocr" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"instances": []map[string]any{}})
	}))
	defer server.Close()

	t.Setenv("REGISTRY_URL", server.URL)
	t.Setenv("SERVICE_ID", "core-test")

	bootstrap := FromEnv("core")
	defer bootstrap.Close()

	resolver := bootstrap.NewResolver("ocr", "http://localhost:5174")
	if got, want := resolver.URL(), "http://localhost:5174"; got != want {
		t.Fatalf("resolver URL = %q, want bootstrap fallback %q", got, want)
	}
}

// TestResolverResyncsAfterWatchReconnect verifies a peer that registers while
// the watch stream is down is still picked up once the stream is reopened.
func TestResolverResyncsAfterWatchReconnect(t *testing.T) {
	var hosted atomic.Bool
	var watchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/v1/services/ocr":
			instances := []map[string]any{}
			if hosted.Load() {
				instances = append(instances, map[string]any{"id": "ocr", "host": "flashbacks-ocr", "port": 5174})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"instances": instances})
		case "/registry/v1/services/ocr/watch":
			watchCalls.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"type\":\"ready\"}\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			// Closing the stream forces the resolver to reconnect.
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("REGISTRY_URL", server.URL)
	t.Setenv("SERVICE_ID", "core-test")

	bootstrap := FromEnv("core")
	defer bootstrap.Close()

	resolver := bootstrap.NewResolver("ocr", "http://localhost:5174")
	if got, want := resolver.URL(), "http://localhost:5174"; got != want {
		t.Fatalf("initial resolver URL = %q, want bootstrap fallback %q", got, want)
	}

	// The peer registers while no watch stream is established.
	hosted.Store(true)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if resolver.URL() == "http://flashbacks-ocr:5174" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("resolver URL = %q after %d watch streams, want discovered peer", resolver.URL(), watchCalls.Load())
}
