package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const alibabaCatalogFixture = `{
	"success": true,
	"output": {
		"total": 1,
		"models": [
			{
				"model": "qwen3.8-max",
				"name": "Qwen3.8-Max",
				"capabilities": ["Reasoning", "VU", "TG"],
				"prices": [
					{
						"prices": [
							{"type": "input_token", "price": "2", "price_unit": "Per 1M tokens", "price_name": "Input"},
							{"type": "output_token", "price": "6", "price_unit": "Per 1M tokens", "price_name": "Output"}
						],
						"range_name": "Default"
					}
				],
				"inference_metadata": {
					"response_modality": ["Text"],
					"request_modality": ["Image", "Text", "Video"]
				},
				"model_info": {
					"context_window": 1000000
				}
			}
		]
	}
}`

func TestAlibabaPricingParsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(alibabaCatalogFixture))
	}))
	defer server.Close()

	c := NewAlibabaClient(server.URL, "test-key", "qwen3.8-max", 3.6)
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].InputPricePerMTok != 2 {
		t.Errorf("expected input price 2, got %v", models[0].InputPricePerMTok)
	}
	if models[0].OutputPricePerMTok != 6 {
		t.Errorf("expected output price 6, got %v", models[0].OutputPricePerMTok)
	}
}

func TestAlibabaModalitiesParsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(alibabaCatalogFixture))
	}))
	defer server.Close()

	c := NewAlibabaClient(server.URL, "test-key", "qwen3.8-max", 3.6)
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if !containsFold(models[0].InputModalities, "text") {
		t.Errorf("expected text input modality, got %v", models[0].InputModalities)
	}
	if !containsFold(models[0].InputModalities, "image") {
		t.Errorf("expected image input modality, got %v", models[0].InputModalities)
	}
	if !containsFold(models[0].OutputModalities, "text") {
		t.Errorf("expected text output modality, got %v", models[0].OutputModalities)
	}
	if containsFold(models[0].OutputModalities, "image") {
		t.Errorf("image must not be an output modality, got %v", models[0].OutputModalities)
	}
}

// alibabaImageCatalogFixture mirrors a real ModelStudio catalog entry for an
// image-generation model such as qwen-image-2.1-pro, which reports the "IG"
// capability code and image output.
const alibabaImageCatalogFixture = `{
	"success": true,
	"output": {
		"total": 1,
		"models": [
			{
				"model": "qwen-image-2.1-pro",
				"name": "Qwen-Image-2.1-Pro",
				"capabilities": ["IG"],
				"prices": [
					{
						"prices": [
							{"type": "image_number", "price": "0.04", "price_unit": "Per image", "price_name": "Image Generation"}
						],
						"range_name": "Default"
					}
				],
				"inference_metadata": {
					"response_modality": ["Image"],
					"request_modality": ["Text", "Image"]
				},
				"model_info": {
					"context_window": null
				}
			}
		]
	}
}`

func TestNormalizeCapabilitiesImageGeneration(t *testing.T) {
	for _, raw := range []string{"IG", "ig", "image_generation"} {
		caps := normalizeCapabilities([]string{raw})
		if !containsFold(caps, "image_edit") {
			t.Errorf("normalizeCapabilities(%q) = %v, want image_edit", raw, caps)
		}
	}
}

func TestAlibabaImageGenerationModelIsImageEditCapable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(alibabaImageCatalogFixture))
	}))
	defer server.Close()

	c := NewAlibabaClient(server.URL, "test-key", "qwen-image-2.1-pro", 3.6)
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if !containsFold(models[0].Capabilities, "image_edit") {
		t.Errorf("expected image_edit capability, got %v", models[0].Capabilities)
	}
}
