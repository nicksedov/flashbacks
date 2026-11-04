package config

import (
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
)

// TestLlmModelsCacheTTLDefault guards task 4.4: the documented default of the
// catalog TTL is 24h.
func TestLlmModelsCacheTTLDefault(t *testing.T) {
	t.Setenv("LLM_MODELS_CACHE_TTL", "")

	cfg := LoadConfig()
	if cfg.LlmModelsCacheTTL != 24*time.Hour {
		t.Fatalf("LLM_MODELS_CACHE_TTL default = %v, want 24h", cfg.LlmModelsCacheTTL)
	}
}

// TestLlmModelsCacheTTLFromEnv verifies the documented value parses.
func TestLlmModelsCacheTTLFromEnv(t *testing.T) {
	t.Setenv("LLM_MODELS_CACHE_TTL", "90m")

	cfg := LoadConfig()
	if cfg.LlmModelsCacheTTL != 90*time.Minute {
		t.Fatalf("LLM_MODELS_CACHE_TTL = %v, want 90m", cfg.LlmModelsCacheTTL)
	}
}

// TestLlmModelsCacheTTLInvalidFallsBack verifies an invalid value keeps the
// documented default instead of disabling the cache.
func TestLlmModelsCacheTTLInvalidFallsBack(t *testing.T) {
	t.Setenv("LLM_MODELS_CACHE_TTL", "not-a-duration")

	cfg := LoadConfig()
	if cfg.LlmModelsCacheTTL != 24*time.Hour {
		t.Fatalf("LLM_MODELS_CACHE_TTL = %v, want the 24h default", cfg.LlmModelsCacheTTL)
	}
}

// TestLlmModelsCacheTTLRegistryRoundTrip verifies the setting survives the
// registry overlay.
func TestLlmModelsCacheTTLRegistryRoundTrip(t *testing.T) {
	cfg := LoadConfig()
	cfg.LlmModelsCacheTTL = 6 * time.Hour

	defaults := cfg.RegistryDefaults()
	if _, ok := defaults["LLM_MODELS_CACHE_TTL"]; !ok {
		t.Fatal("LLM_MODELS_CACHE_TTL is missing from the registry defaults")
	}

	overlay := LoadConfig()
	overlay.ApplyRegistry(defaults)
	if overlay.LlmModelsCacheTTL != 6*time.Hour {
		t.Fatalf("applied LLM_MODELS_CACHE_TTL = %v, want 6h", overlay.LlmModelsCacheTTL)
	}
}

// TestFacesBatchDefaults guards the documented defaults: eight images per
// analysis request and a five-minute budget for it.
func TestFacesBatchDefaults(t *testing.T) {
	t.Setenv("FACES_BATCH_SIZE", "")
	t.Setenv("FACES_BATCH_CONCURRENCY", "")
	t.Setenv("FACES_ANALYZE_TIMEOUT", "")

	cfg := LoadConfig()
	if cfg.FacesBatchSize != 8 {
		t.Fatalf("FACES_BATCH_SIZE default = %d, want 8", cfg.FacesBatchSize)
	}
	if cfg.FacesBatchConcurrency != 2 {
		t.Fatalf("FACES_BATCH_CONCURRENCY default = %d, want 2", cfg.FacesBatchConcurrency)
	}
	if cfg.FacesAnalyzeTimeout != 300*time.Second {
		t.Fatalf("FACES_ANALYZE_TIMEOUT default = %v, want 300s", cfg.FacesAnalyzeTimeout)
	}
}

// TestFacesBatchOverrides verifies the documented values parse from the
// environment.
func TestFacesBatchOverrides(t *testing.T) {
	t.Setenv("FACES_BATCH_SIZE", "1")
	t.Setenv("FACES_BATCH_CONCURRENCY", "3")
	t.Setenv("FACES_ANALYZE_TIMEOUT", "90s")

	cfg := LoadConfig()
	if cfg.FacesBatchSize != 1 {
		t.Fatalf("FACES_BATCH_SIZE = %d, want 1", cfg.FacesBatchSize)
	}
	if cfg.FacesBatchConcurrency != 3 {
		t.Fatalf("FACES_BATCH_CONCURRENCY = %d, want 3", cfg.FacesBatchConcurrency)
	}
	if cfg.FacesAnalyzeTimeout != 90*time.Second {
		t.Fatalf("FACES_ANALYZE_TIMEOUT = %v, want 90s", cfg.FacesAnalyzeTimeout)
	}
}

// TestFacesBatchRegistryRoundTrip verifies both knobs survive the registry
// overlay.
func TestFacesBatchRegistryRoundTrip(t *testing.T) {
	cfg := LoadConfig()
	cfg.FacesBatchSize = 4
	cfg.FacesAnalyzeTimeout = 45 * time.Second

	defaults := cfg.RegistryDefaults()
	overlay := LoadConfig()
	overlay.ApplyRegistry(defaults)

	if overlay.FacesBatchSize != 4 {
		t.Fatalf("applied FACES_BATCH_SIZE = %d, want 4", overlay.FacesBatchSize)
	}
	if overlay.FacesAnalyzeTimeout != 45*time.Second {
		t.Fatalf("applied FACES_ANALYZE_TIMEOUT = %v, want 45s", overlay.FacesAnalyzeTimeout)
	}
}

// TestFacesClusterDefaults guards the documented clustering defaults: automatic
// strategy and sixteen candidates per face.
func TestFacesClusterDefaults(t *testing.T) {
	t.Setenv("FACES_CLUSTER_MODE", "")
	t.Setenv("FACES_CLUSTER_NEIGHBOURS", "")

	cfg := LoadConfig()
	if cfg.FacesClusterMode != domain.FacesClusterModeAuto {
		t.Fatalf("FACES_CLUSTER_MODE default = %q, want %q", cfg.FacesClusterMode, domain.FacesClusterModeAuto)
	}
	if cfg.FacesClusterNeighbours != 16 {
		t.Fatalf("FACES_CLUSTER_NEIGHBOURS default = %d, want 16", cfg.FacesClusterNeighbours)
	}
}

// TestFacesClusterOverrides verifies the clustering knobs parse from the
// environment and survive the registry overlay.
func TestFacesClusterOverrides(t *testing.T) {
	t.Setenv("FACES_CLUSTER_MODE", domain.FacesClusterModeMemory)
	t.Setenv("FACES_CLUSTER_NEIGHBOURS", "32")

	cfg := LoadConfig()
	if cfg.FacesClusterMode != domain.FacesClusterModeMemory {
		t.Fatalf("FACES_CLUSTER_MODE = %q, want memory", cfg.FacesClusterMode)
	}
	if cfg.FacesClusterNeighbours != 32 {
		t.Fatalf("FACES_CLUSTER_NEIGHBOURS = %d, want 32", cfg.FacesClusterNeighbours)
	}

	overlay := LoadConfig()
	overlay.ApplyRegistry(cfg.RegistryDefaults())
	if overlay.FacesClusterMode != domain.FacesClusterModeMemory {
		t.Fatalf("applied FACES_CLUSTER_MODE = %q, want memory", overlay.FacesClusterMode)
	}
	if overlay.FacesClusterNeighbours != 32 {
		t.Fatalf("applied FACES_CLUSTER_NEIGHBOURS = %d, want 32", overlay.FacesClusterNeighbours)
	}
}
