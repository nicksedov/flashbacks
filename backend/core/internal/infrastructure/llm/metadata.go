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

// capabilityCodeMap maps a lower-cased provider-native capability code to every
// standardized capability it implies. One native code may yield several values
// (for example "World-Model" implies both world_model and video_generation).
var capabilityCodeMap = map[string][]string{
	"completion":        {"chat"},
	"chat":              {"chat"},
	"tg":                {"chat"},
	"text":              {"chat"},
	"realtime-chatting": {"chat"},

	"tools":            {"tool_calling"},
	"tool_calling":     {"tool_calling"},
	"function-calling": {"tool_calling"},
	"functions":        {"tool_calling"},

	"vision": {"vision"},
	"visual": {"vision"},
	"vu":     {"vision"},
	"image":  {"vision"},

	"embedding":  {"embedding"},
	"embeddings": {"embedding"},

	// Alibaba ModelStudio reports image-generation models with the "IG" code.
	// Qwen-Image models unify generation and editing, so IG is treated as
	// image-edit capable for instrument filtering.
	"image_edit":       {"image_edit"},
	"image-edit":       {"image_edit"},
	"image_generation": {"image_edit"},
	"image-generation": {"image_edit"},
	"ig":               {"image_edit"},

	"reasoning": {"reasoning"},

	"vg":               {"video_generation"},
	"video_generation": {"video_generation"},
	"video-generation": {"video_generation"},
	"world-model":      {"video_generation", "world_model"},
	"world_model":      {"world_model"},

	"tts":                     {"text_to_speech"},
	"text_to_speech":          {"text_to_speech"},
	"text-to-speech":          {"text_to_speech"},
	"realtime-text-to-speech": {"text_to_speech"},

	"asr":                {"speech_recognition"},
	"speech_recognition": {"speech_recognition"},
	"speech-recognition": {"speech_recognition"},
	"realtime-asr":       {"speech_recognition"},

	"omni":            {"omni"},
	"multimodal-omni": {"omni", "chat"},
	"realtime-omni":   {"omni"},
}

// realtimeCodePrefix marks the provider's streaming family; every such code also
// implies the standardized realtime capability.
const realtimeCodePrefix = "realtime-"

// normalizeCapabilities converts provider-specific capability names into the
// standardized capability vocabulary: chat, tool_calling, vision, embedding,
// image_edit, reasoning, video_generation, text_to_speech, speech_recognition,
// realtime, omni, and world_model. Names that do not map to a standardized
// capability are dropped.
func normalizeCapabilities(raw []string) []string {
	var out []string
	for _, r := range raw {
		code := strings.ToLower(strings.TrimSpace(r))
		if code == "" {
			continue
		}
		for _, capability := range capabilityCodeMap[code] {
			out = appendUnique(out, capability)
		}
		if strings.HasPrefix(code, realtimeCodePrefix) {
			out = appendUnique(out, "realtime")
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

// capabilitiesFromModalities builds standardized capabilities from a model's
// input modality list: an image input implies vision, and all other models are
// treated as chat + tool_calling.
func capabilitiesFromModalities(inputModalities []string) []string {
	caps := []string{"chat", "tool_calling"}
	if containsFold(inputModalities, "image") {
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
