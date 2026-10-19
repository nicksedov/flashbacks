package registrybootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
