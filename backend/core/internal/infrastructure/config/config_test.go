package config

import (
	"testing"
	"time"
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
