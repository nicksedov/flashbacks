package llm

import (
	"strings"
	"testing"
)

// countCapability counts case-insensitive occurrences of a capability.
func countCapability(caps []string, want string) int {
	count := 0
	for _, c := range caps {
		if strings.EqualFold(c, want) {
			count++
		}
	}
	return count
}

// TestAlibabaCapabilitiesDeduplicated guards
// llm-model-metadata.capabilities-deduplicated: an identifier matching both a
// vision family rule and an OCR rule must still carry a single `vision` entry.
func TestAlibabaCapabilitiesDeduplicated(t *testing.T) {
	caps := inferAlibabaCapabilities("qwen2-vl-ocr-7b")
	if got := countCapability(caps, "vision"); got != 1 {
		t.Fatalf("inferAlibabaCapabilities recorded vision %d times in %v, want exactly 1", got, caps)
	}

	// The same guarantee must hold after the catalog conversion, which is the
	// path that feeds persistence.
	info := alibabaCatalogModelToInfo(alibabaTestModel(
		"qwen2-vl-ocr-7b", nil, []string{"Image", "Text"}, []string{"Text"},
	))
	if got := countCapability(info.Capabilities, "vision"); got != 1 {
		t.Fatalf("catalog conversion recorded vision %d times in %v, want exactly 1", got, info.Capabilities)
	}
}

// TestAlibabaListingDerivesVisionFromModalities guards
// llm-model-metadata.image-input-implies-vision: a listing that reports image
// input and text output but no capability codes must still record vision.
func TestAlibabaListingDerivesVisionFromModalities(t *testing.T) {
	info := alibabaCatalogModelToInfo(alibabaTestModel(
		"acme-multimodal-8b", nil, []string{"Image", "Text"}, []string{"Text"},
	))
	assertHasCapabilities(t, info.Capabilities, "vision")
	if !containsFold(info.InputModalities, "image") {
		t.Fatalf("input modalities = %v, want image", info.InputModalities)
	}
}

// TestAlibabaChatIdentifierNotVision guards
// llm-model-metadata.chat-identifier-not-vision: a chat model described only by
// its identifier and the registry is never granted vision.
func TestAlibabaChatIdentifierNotVision(t *testing.T) {
	info := alibabaEnrichedModelInfo("acme-chat-7b", "Acme Chat 7B")
	if containsFold(info.Capabilities, "vision") {
		t.Fatalf("capabilities = %v, want no vision for a plain chat identifier", info.Capabilities)
	}
	if !containsFold(info.Capabilities, "chat") {
		t.Fatalf("capabilities = %v, want chat", info.Capabilities)
	}
}

// TestModelFamilyRegistryLongestTokenWins verifies the registry resolves the
// most specific family instead of the first matching one.
func TestModelFamilyRegistryLongestTokenWins(t *testing.T) {
	family, ok := lookupModelFamily("qwen-vl-ocr-plus")
	if !ok {
		t.Fatal("expected a registry match for qwen-vl-ocr-plus")
	}
	if family.Token != "qwen-vl-ocr" {
		t.Fatalf("matched family token = %q, want %q", family.Token, "qwen-vl-ocr")
	}
	if !containsFold(family.Capabilities, "vision") {
		t.Fatalf("family capabilities = %v, want vision", family.Capabilities)
	}
}

// TestModelFamilyRegistrySuppliesDefaults verifies a described family receives
// its curated defaults while an undescribed chat model stays text-only.
func TestModelFamilyRegistrySuppliesDefaults(t *testing.T) {
	described := alibabaEnrichedModelInfo("qwen-vl-ocr-plus", "Qwen VL OCR Plus")
	assertHasCapabilities(t, described.Capabilities, "vision")
	if described.ContextLength != 128000 {
		t.Errorf("described family context = %d, want 128000", described.ContextLength)
	}
	if !containsFold(described.InputModalities, "image") {
		t.Errorf("described family input modalities = %v, want image", described.InputModalities)
	}

	undescribed := alibabaEnrichedModelInfo("acme-chat-7b", "Acme Chat 7B")
	if containsFold(undescribed.Capabilities, "vision") {
		t.Errorf("undescribed model capabilities = %v, want no vision", undescribed.Capabilities)
	}
	if undescribed.ContextLength != defaultContextLength {
		t.Errorf("undescribed model context = %d, want %d", undescribed.ContextLength, defaultContextLength)
	}
}

// TestModelFamilyRegistryNeverOverwritesKnownFields verifies the registry only
// fills fields the provider left unknown.
func TestModelFamilyRegistryNeverOverwritesKnownFields(t *testing.T) {
	known := ModelInfo{
		ID:                 "qwen-vl-ocr-plus",
		ContextLength:      32768,
		Capabilities:       []string{"image_edit"},
		InputModalities:    []string{"audio"},
		OutputModalities:   []string{"audio"},
		InputPricePerMTok:  7,
		OutputPricePerMTok: 9,
	}
	got := enrichFromRegistry(known)
	if got.ContextLength != 32768 {
		t.Errorf("context length = %d, want the known 32768", got.ContextLength)
	}
	assertCapabilitySet(t, got.Capabilities, []string{"image_edit"})
	if !containsFold(got.InputModalities, "audio") || containsFold(got.InputModalities, "image") {
		t.Errorf("input modalities = %v, want the known audio only", got.InputModalities)
	}
	if got.InputPricePerMTok != 7 || got.OutputPricePerMTok != 9 {
		t.Errorf("prices = %v/%v, want the known 7/9", got.InputPricePerMTok, got.OutputPricePerMTok)
	}
}

// TestNormalizeModelInfoNativeCodes verifies provider-native codes are mapped to
// the standardized vocabulary, lower-cased, deduplicated, and stripped of
// unknown values.
func TestNormalizeModelInfoNativeCodes(t *testing.T) {
	got := normalizeModelInfo(ModelInfo{
		ID:               "acme-native-1",
		Capabilities:     []string{"VU", "Vision", "vision", "TG", "Tools", "not-a-capability"},
		InputModalities:  []string{"TEXT", "Image", "image", "hologram"},
		OutputModalities: []string{"Text", "TEXT"},
	})

	assertCapabilitySet(t, got.Capabilities, []string{"chat", "tool_calling", "vision"})
	if len(got.InputModalities) != 2 {
		t.Fatalf("input modalities = %v, want deduplicated [text image]", got.InputModalities)
	}
	if got.InputModalities[0] != "text" || got.InputModalities[1] != "image" {
		t.Errorf("input modalities = %v, want [text image]", got.InputModalities)
	}
	if len(got.OutputModalities) != 1 || got.OutputModalities[0] != "text" {
		t.Errorf("output modalities = %v, want [text]", got.OutputModalities)
	}
	if containsFold(got.InputModalities, "hologram") {
		t.Errorf("unknown modality must be dropped, got %v", got.InputModalities)
	}
}

// TestNormalizeModelInfoListingInput verifies the normalization step derives
// capabilities from reported modalities and keeps input/output direction apart.
func TestNormalizeModelInfoListingInput(t *testing.T) {
	got := normalizeModelInfo(ModelInfo{
		ID:               "acme-listing-1",
		InputModalities:  []string{"Image", "Text"},
		OutputModalities: []string{"Text"},
	})

	assertHasCapabilities(t, got.Capabilities, "vision")
	if !containsFold(got.InputModalities, "image") {
		t.Errorf("input modalities = %v, want image", got.InputModalities)
	}
	if containsFold(got.OutputModalities, "image") {
		t.Errorf("output modalities = %v, must not contain image", got.OutputModalities)
	}

	// Re-normalizing must be idempotent.
	again := normalizeModelInfo(got)
	assertCapabilitySet(t, again.Capabilities, got.Capabilities)
	if len(again.InputModalities) != len(got.InputModalities) {
		t.Errorf("normalization is not idempotent: %v -> %v", got.InputModalities, again.InputModalities)
	}
}

// TestInferContextLengthUsesRegistry verifies the curated registry fixes the
// previously hard-coded context windows.
func TestInferContextLengthUsesRegistry(t *testing.T) {
	if got := inferContextLength("qwen3.5-flash"); got != 1000000 {
		t.Errorf("inferContextLength(qwen3.5-flash) = %d, want 1000000", got)
	}
	if got := inferContextLength("acme-unknown-1"); got != defaultContextLength {
		t.Errorf("inferContextLength(acme-unknown-1) = %d, want %d", got, defaultContextLength)
	}
}

// TestLookupModelPricingFromRegistry verifies pricing still resolves through the
// registry, longest token first.
func TestLookupModelPricingFromRegistry(t *testing.T) {
	pricing, ok := lookupModelPricing("gpt-4o-mini-2024-07-18")
	if !ok {
		t.Fatal("expected pricing for gpt-4o-mini")
	}
	if pricing.InputPerMTok != 0.15 || pricing.OutputPerMTok != 0.60 {
		t.Errorf("pricing = %+v, want the gpt-4o-mini entry", pricing)
	}

	if _, ok := lookupModelPricing("acme-unknown-1"); ok {
		t.Error("expected no curated pricing for an unknown model")
	}
}
