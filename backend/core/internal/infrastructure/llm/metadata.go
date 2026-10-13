package llm

import (
	"strings"
)

// ModelPricing holds a known input/output price per one million tokens in USD.
type ModelPricing struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

// knownModelPrices maps exact model IDs (and prefixes) to published per-1M-token
// prices in USD. It is a best-effort fallback for providers that do not expose
// pricing through their model-listing API; values are refreshed manually.
var knownModelPrices = map[string]ModelPricing{
	"deepseek-chat":     {InputPerMTok: 0.27, OutputPerMTok: 1.10},
	"deepseek-reasoner": {InputPerMTok: 0.55, OutputPerMTok: 2.19},
	"deepseek-v4-pro":   {InputPerMTok: 0.55, OutputPerMTok: 2.19},
	"deepseek-v4-flash": {InputPerMTok: 0.20, OutputPerMTok: 0.80},
	"deepseek-flash":    {InputPerMTok: 0.20, OutputPerMTok: 0.80},
	"gpt-4o":            {InputPerMTok: 2.50, OutputPerMTok: 10.00},
	"gpt-4o-mini":       {InputPerMTok: 0.15, OutputPerMTok: 0.60},
	"gpt-4.1":           {InputPerMTok: 2.00, OutputPerMTok: 8.00},
	"gpt-4.1-mini":      {InputPerMTok: 0.40, OutputPerMTok: 1.60},
	"gpt-4.1-nano":      {InputPerMTok: 0.10, OutputPerMTok: 0.40},
	"o3-mini":           {InputPerMTok: 1.10, OutputPerMTok: 4.40},
}

// lookupModelPricing returns the known pricing for a model ID, preferring the
// longest matching prefix when there is no exact match.
func lookupModelPricing(modelID string) (ModelPricing, bool) {
	if p, ok := knownModelPrices[modelID]; ok {
		return p, true
	}
	best := ModelPricing{}
	found := false
	bestLen := -1
	for prefix, p := range knownModelPrices {
		if strings.HasPrefix(modelID, prefix) && len(prefix) > bestLen {
			best = p
			bestLen = len(prefix)
			found = true
		}
	}
	return best, found
}

// normalizeCapabilities converts provider-specific capability names into the
// standardized capability set: chat, tool_calling, vision, embedding, image_edit.
// Names that do not map to an instrument-relevant capability are dropped.
func normalizeCapabilities(raw []string) []string {
	var out []string
	for _, r := range raw {
		switch strings.ToLower(strings.TrimSpace(r)) {
		case "completion", "chat", "tg", "text":
			out = appendUnique(out, "chat")
		case "tools", "tool_calling", "function-calling", "functions":
			out = appendUnique(out, "tool_calling")
		case "vision", "visual", "vu", "image":
			out = appendUnique(out, "vision")
		case "embedding", "embeddings":
			out = appendUnique(out, "embedding")
		case "image_edit", "image-edit", "image_generation", "image-generation", "ig":
			// Alibaba ModelStudio reports image-generation models with the "IG"
			// capability code. Qwen-Image models unify generation and editing, so
			// IG is treated as image-edit capable for instrument filtering.
			out = appendUnique(out, "image_edit")
		}
	}
	return out
}

// inferModalities derives input and output modality lists from standardized
// capabilities. A vision capability implies image input.
func inferModalities(caps []string) (input []string, output []string) {
	input = []string{"text"}
	if containsFold(caps, "vision") {
		input = append(input, "image")
	}
	output = []string{"text"}
	return input, output
}

// capabilitiesFromModalities builds standardized capabilities from modality
// lists: an image input implies vision, and all other models are treated as
// chat + tool_calling.
func capabilitiesFromModalities(modalities []string) []string {
	caps := []string{"chat", "tool_calling"}
	if containsFold(modalities, "image") {
		caps = appendUnique(caps, "vision")
	}
	return caps
}

// mergeModalities returns the deduplicated union of the provided modality
// lists, preserving order.
func mergeModalities(lists ...[]string) []string {
	var out []string
	for _, list := range lists {
		for _, m := range list {
			m = strings.ToLower(strings.TrimSpace(m))
			if m != "" {
				out = appendUnique(out, m)
			}
		}
	}
	return out
}

// inferCapabilitiesByID falls back to model-ID heuristics for providers that do
// not report capabilities. Embedding models are detected first; image-edit and
// vision models are detected from family tokens.
func inferCapabilitiesByID(modelID string) []string {
	lower := strings.ToLower(modelID)
	if strings.Contains(lower, "embedding") {
		return []string{"embedding"}
	}
	caps := []string{"chat", "tool_calling"}
	if isVisionModelID(lower) {
		caps = appendUnique(caps, "vision")
	}
	if strings.Contains(lower, "image-edit") || strings.Contains(lower, "image_edit") {
		caps = appendUnique(caps, "image_edit")
	}
	return caps
}

func isVisionModelID(lower string) bool {
	for _, tok := range []string{
		"vl", "vision", "minicpm", "llava", "bakllava", "cogvlm", "pixtral",
		"qwen2-vl", "qwen-vl", "qwen-image-edit", "gpt-4o", "gpt-4-vision",
		"gemini", "claude", "glm-4v", "internvl", "deepseek-vl", "deepseek-flash",
	} {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

func appendUnique(list []string, s string) []string {
	for _, item := range list {
		if item == s {
			return list
		}
	}
	return append(list, s)
}
