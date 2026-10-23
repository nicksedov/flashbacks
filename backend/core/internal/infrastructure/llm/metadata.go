package llm

import (
	"strings"
)

// ModelPricing holds a known input/output price per one million tokens in USD.
type ModelPricing struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

// ModelFamily is one entry of the curated model-family registry. It records what
// the project knows about a family without asking the provider: its default
// capabilities, modalities, context window, and published prices.
//
// Registry entries are consulted only for fields a provider left unknown, so a
// stale entry can never overwrite richer provider metadata.
type ModelFamily struct {
	// Token is matched case-insensitively anywhere in the model identifier.
	Token string

	// Capabilities is the family's default standardized capability set. An empty
	// value means the registry has no opinion and lower-priority sources decide.
	Capabilities []string

	// InputModalities/OutputModalities default the model's modality lists.
	InputModalities  []string
	OutputModalities []string

	// ContextLength is the family's context window in tokens (0 = unknown).
	ContextLength int

	// Pricing is the family's published per-1M-token price (0 = unknown).
	Pricing ModelPricing
}

// chatCapabilities returns the standard chat capability set plus extra entries.
func chatCapabilities(extra ...string) []string {
	caps := []string{"chat", "tool_calling"}
	return append(caps, extra...)
}

// textOnly is the default modality pair for a text-in/text-out family.
func textOnly() []string {
	return []string{"text"}
}

// modelFamilies is the curated model-family registry. lookupModelFamily resolves
// a model identifier with longest-token-first matching, so "qwen-vl-ocr" beats
// "qwen-vl", which beats "qwen".
//
// A family only carries capabilities when the identifier itself is unambiguous
// about them: chat families stay text-only, and vision is granted only to
// families whose token explicitly names a vision lineage.
var modelFamilies = []ModelFamily{
	// ── Vision-language / OCR families ───────────────────────────────
	{
		Token:            "qwen-vl-ocr",
		Capabilities:     chatCapabilities("vision", "reasoning"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    128000,
	},
	{
		Token:            "qwen2-vl",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    32768,
	},
	{
		Token:            "qwen-vl",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    32768,
	},
	{
		Token:            "qvq",
		Capabilities:     chatCapabilities("vision", "reasoning"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    128000,
	},
	{
		Token:            "minicpm",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
	},
	{
		Token:            "llava",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
	},
	{
		Token:            "pixtral",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
	},
	{
		Token:            "internvl",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
	},
	{
		Token:            "deepseek-vl",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
	},
	{
		Token:            "glm-4v",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    128000,
	},
	{
		Token:            "gpt-4o-mini",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    128000,
		Pricing:          ModelPricing{InputPerMTok: 0.15, OutputPerMTok: 0.60},
	},
	{
		Token:            "gpt-4o",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    128000,
		Pricing:          ModelPricing{InputPerMTok: 2.50, OutputPerMTok: 10.00},
	},
	{
		Token:            "gpt-4-vision",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    128000,
	},
	{
		Token:            "gemini",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    1000000,
	},
	{
		Token:            "claude",
		Capabilities:     chatCapabilities("vision"),
		InputModalities:  []string{"text", "image"},
		OutputModalities: textOnly(),
		ContextLength:    200000,
	},

	// ── Text chat families ───────────────────────────────────────────
	//
	// These entries deliberately carry context length (and prices) only. A
	// non-vision family must not claim capabilities or modality lists: those
	// entries match broad identifier substrings (the "qwen" token matches every
	// Qwen model), so asserting "text only" here would hide the vision models of
	// the same family and pre-empt the identifier heuristics that detect them.
	// Capabilities and modalities are left to the provider or, when absent, to
	// the vision families above and the ID heuristics.
	{
		Token:         "qwen3.5-flash",
		ContextLength: 1000000,
	},
	{
		Token:         "qwen3-max",
		ContextLength: 1000000,
	},
	{
		Token:         "qwen-long",
		ContextLength: 1000000,
	},
	{
		Token:         "qwen-turbo",
		ContextLength: 1000000,
	},
	{
		Token:         "qwen-plus",
		ContextLength: 262144,
	},
	{
		Token:         "qwen",
		ContextLength: 32768,
	},
	{
		Token:         "kimi-k2.5",
		ContextLength: 262144,
	},
	{
		Token:         "glm-5.1",
		ContextLength: 128000,
	},
	{
		Token:         "glm-5.2",
		ContextLength: 128000,
	},
	{
		Token:         "glm-5",
		ContextLength: 1000000,
	},
	{
		Token:         "glm-4",
		ContextLength: 128000,
	},
	{
		Token:         "deepseek-v4-pro",
		ContextLength: 1000000,
		Pricing:       ModelPricing{InputPerMTok: 0.55, OutputPerMTok: 2.19},
	},
	{
		Token:         "deepseek-v4-flash",
		ContextLength: 1000000,
		Pricing:       ModelPricing{InputPerMTok: 0.20, OutputPerMTok: 0.80},
	},
	{
		Token:         "deepseek-chat",
		ContextLength: 128000,
		Pricing:       ModelPricing{InputPerMTok: 0.27, OutputPerMTok: 1.10},
	},
	{
		Token:         "deepseek-reasoner",
		ContextLength: 128000,
		Pricing:       ModelPricing{InputPerMTok: 0.55, OutputPerMTok: 2.19},
	},
	{
		Token:         "deepseek-flash",
		ContextLength: 128000,
		Pricing:       ModelPricing{InputPerMTok: 0.20, OutputPerMTok: 0.80},
	},
	{
		Token:         "gpt-4.1-mini",
		ContextLength: 1000000,
		Pricing:       ModelPricing{InputPerMTok: 0.40, OutputPerMTok: 1.60},
	},
	{
		Token:         "gpt-4.1-nano",
		ContextLength: 1000000,
		Pricing:       ModelPricing{InputPerMTok: 0.10, OutputPerMTok: 0.40},
	},
	{
		Token:         "gpt-4.1",
		ContextLength: 1000000,
		Pricing:       ModelPricing{InputPerMTok: 2.00, OutputPerMTok: 8.00},
	},
	{
		Token:         "o3-mini",
		ContextLength: 200000,
		Pricing:       ModelPricing{InputPerMTok: 1.10, OutputPerMTok: 4.40},
	},
}

// lookupModelFamily resolves the most specific curated family for a model ID.
// Matching is case-insensitive and longest-token-first: "qwen-vl-ocr" wins over
// "qwen-vl", which wins over "qwen".
func lookupModelFamily(modelID string) (ModelFamily, bool) {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	if lower == "" {
		return ModelFamily{}, false
	}

	best := ModelFamily{}
	found := false
	bestLen := 0
	for _, family := range modelFamilies {
		token := strings.ToLower(family.Token)
		if token == "" || !strings.Contains(lower, token) {
			continue
		}
		if len(token) > bestLen {
			best = family
			bestLen = len(token)
			found = true
		}
	}
	return best, found
}

// lookupModelPricing returns the curated pricing for a model ID, preferring the
// longest matching family token.
func lookupModelPricing(modelID string) (ModelPricing, bool) {
	family, ok := lookupModelFamily(modelID)
	if !ok {
		return ModelPricing{}, false
	}
	if family.Pricing.InputPerMTok == 0 && family.Pricing.OutputPerMTok == 0 {
		return ModelPricing{}, false
	}
	return family.Pricing, true
}

// enrichFromRegistry fills the fields a provider left unknown with the curated
// defaults of the model's family. Known values are never overwritten.
func enrichFromRegistry(info ModelInfo) ModelInfo {
	family, ok := lookupModelFamily(info.ID)
	if !ok {
		return info
	}
	if len(info.Capabilities) == 0 {
		info.Capabilities = append([]string(nil), family.Capabilities...)
	}
	if len(info.InputModalities) == 0 {
		info.InputModalities = append([]string(nil), family.InputModalities...)
	}
	if len(info.OutputModalities) == 0 {
		info.OutputModalities = append([]string(nil), family.OutputModalities...)
	}
	if info.ContextLength == 0 {
		info.ContextLength = family.ContextLength
	}
	if info.InputPricePerMTok == 0 {
		info.InputPricePerMTok = family.Pricing.InputPerMTok
	}
	if info.OutputPricePerMTok == 0 {
		info.OutputPricePerMTok = family.Pricing.OutputPerMTok
	}
	return info
}

// enrichFromHeuristics fills the fields still unknown after the registry from
// model-ID naming patterns.
func enrichFromHeuristics(info ModelInfo) ModelInfo {
	if len(info.Capabilities) == 0 {
		info.Capabilities = inferCapabilitiesByID(info.ID)
	}
	if len(info.InputModalities) == 0 && len(info.OutputModalities) == 0 {
		info.InputModalities, info.OutputModalities = inferModalities(info.Capabilities)
	}
	if info.ContextLength == 0 {
		info.ContextLength = inferContextLength(info.ID)
	}
	if p, ok := lookupModelPricing(info.ID); ok {
		if info.InputPricePerMTok == 0 {
			info.InputPricePerMTok = p.InputPerMTok
		}
		if info.OutputPricePerMTok == 0 {
			info.OutputPricePerMTok = p.OutputPerMTok
		}
	}
	return info
}

// canonicalCapabilities is the standardized capability vocabulary. Any value
// outside this set is dropped during normalization.
var canonicalCapabilities = map[string]struct{}{
	"chat":               {},
	"tool_calling":       {},
	"vision":             {},
	"embedding":          {},
	"image_edit":         {},
	"reasoning":          {},
	"video_generation":   {},
	"text_to_speech":     {},
	"speech_recognition": {},
	"realtime":           {},
	"omni":               {},
	"world_model":        {},
}

// canonicalModalities is the standardized modality vocabulary. Direction is
// carried by the separate input/output lists, so it survives normalization.
var canonicalModalities = map[string]struct{}{
	"text":  {},
	"image": {},
	"video": {},
	"audio": {},
}

// defaultContextLength is the safe context-window default for an unknown model.
const defaultContextLength = 128000

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
	// Qwen-Image and wan image models unify generation and editing, so every
	// image-generation code is treated as image-edit capable for instrument
	// filtering. The separator-less spellings are what some catalog revisions
	// return for the same capability.
	"image_edit":       {"image_edit"},
	"image-edit":       {"image_edit"},
	"image_generation": {"image_edit"},
	"image-generation": {"image_edit"},
	"imagegeneration":  {"image_edit"},
	"imagegen":         {"image_edit"},
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
// standardized capability vocabulary, lower-cases them, and deduplicates the
// result. Names that do not map to a standardized capability are dropped, so a
// single unknown code can never reach persistence.
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
		if _, ok := canonicalCapabilities[code]; ok {
			out = appendUnique(out, code)
		}
	}
	return out
}

// normalizeModalityList lower-cases, trims, deduplicates, and filters a modality
// list to the standardized vocabulary. Order (and therefore direction) is
// preserved by the caller's separate input/output lists.
func normalizeModalityList(raw []string) []string {
	var out []string
	for _, r := range raw {
		modality := strings.ToLower(strings.TrimSpace(r))
		if modality == "" {
			continue
		}
		if _, ok := canonicalModalities[modality]; !ok {
			continue
		}
		out = appendUnique(out, modality)
	}
	return out
}

// normalizeModelInfo is the single normalization step every collected model goes
// through before it is stored or served: capabilities are mapped to the
// standardized vocabulary, deduplicated and free of unknown codes; modalities
// are lower-cased and deduplicated while keeping their input/output direction.
func normalizeModelInfo(info ModelInfo) ModelInfo {
	info.Capabilities = normalizeCapabilities(info.Capabilities)
	if len(info.Capabilities) == 0 && (len(info.InputModalities) > 0 || len(info.OutputModalities) > 0) {
		info.Capabilities = capabilitiesFromModalities(info.InputModalities, info.OutputModalities)
	}
	info.InputModalities = normalizeModalityList(info.InputModalities)
	info.OutputModalities = normalizeModalityList(info.OutputModalities)
	if len(info.InputModalities) == 0 && len(info.OutputModalities) == 0 && len(info.Capabilities) > 0 {
		input, output := inferModalities(info.Capabilities)
		info.InputModalities = normalizeModalityList(input)
		info.OutputModalities = normalizeModalityList(output)
	}
	return info
}

// EnrichModelInfo runs the full metadata pipeline for a model a provider
// described only partially: the curated model-family registry, then model-ID
// heuristics, then the safe default, followed by normalization. Values the
// provider already supplied are never overwritten.
func EnrichModelInfo(info ModelInfo) ModelInfo {
	return normalizeModelInfo(enrichFromHeuristics(enrichFromRegistry(info)))
}

// NormalizeModelInfo is the single normalization step every collected model goes
// through before it is stored or served.
func NormalizeModelInfo(info ModelInfo) ModelInfo {
	return normalizeModelInfo(info)
}

// inferModalities derives input and output modality lists from standardized
// capabilities. A vision capability implies image input; an image_edit
// capability accepts an image prompt and answers with an image.
func inferModalities(caps []string) (input []string, output []string) {
	input = []string{"text"}
	if containsFold(caps, "vision") || containsFold(caps, "image_edit") {
		input = append(input, "image")
	}
	output = []string{"text"}
	if containsFold(caps, "image_edit") {
		output = []string{"image"}
	}
	return input, output
}

// capabilitiesFromModalities builds standardized capabilities from a model's
// reported modalities. An image output means the model generates or edits
// images, so it is image_edit capable and never a chat model (the same reading
// the catalog's "IG" code receives). Otherwise an image input implies vision and
// the model is treated as chat + tool_calling.
func capabilitiesFromModalities(inputModalities, outputModalities []string) []string {
	if containsFold(outputModalities, "image") {
		return []string{"image_edit"}
	}
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
// not report capabilities. Embedding models are detected first, then image
// generation/editing models; everything else is a chat model plus vision when
// the identifier names a vision lineage.
func inferCapabilitiesByID(modelID string) []string {
	lower := strings.ToLower(modelID)
	if strings.Contains(lower, "embedding") {
		return []string{"embedding"}
	}
	// An image generator is image-edit capable and nothing else: it must not
	// inherit chat/tool_calling from the default chat set.
	if isImageGenerationModelID(lower) {
		return []string{"image_edit"}
	}
	caps := []string{"chat", "tool_calling"}
	if isVisionModelID(lower) {
		caps = appendUnique(caps, "vision")
	}
	return caps
}

// imageGenerationFamilyTokens are the identifier tokens that explicitly name an
// image generation/editing lineage, independent of the provider's capability
// codes. Qwen-Image and wan image models unify text-to-image and
// image-to-image, so every one of them is treated as image-edit capable for
// instrument filtering (the same meaning the catalog's "IG" code receives).
var imageGenerationFamilyTokens = []string{
	"image-edit", "image_edit", "imageedit",
	"qwen-image", "wanx", "cogview", "gpt-image", "dall-e",
	"stable-diffusion", "i2i", "t2i",
}

// isImageGenerationModelID reports whether a lower-cased model identifier names
// an image generation or editing model.
func isImageGenerationModelID(lower string) bool {
	for _, tok := range imageGenerationFamilyTokens {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	// Versioned wan and qwen image models (wan2.7-image, wan2.6-image-pro,
	// qwen-image-2.1-pro) interleave the family name with a version number, so
	// no single static token survives. They are matched structurally instead:
	// the family prefix plus the "image" token. The video variants
	// (wan2.7-t2v, wan2.7-i2v) never carry "image" and stay unaffected.
	if strings.Contains(lower, "image") &&
		(strings.Contains(lower, "wan") || strings.Contains(lower, "qwen")) {
		return true
	}
	return false
}

// visionFamilyTokens are the identifier tokens that explicitly name a vision
// lineage. Vision is never granted from any other token.
var visionFamilyTokens = []string{
	"vl", "vision", "minicpm", "llava", "bakllava", "cogvlm", "pixtral",
	"qwen2-vl", "qwen-vl", "qwen-image-edit", "gpt-4o", "gpt-4-vision",
	"gemini", "claude", "glm-4v", "internvl", "deepseek-vl",
}

func isVisionModelID(lower string) bool {
	for _, tok := range visionFamilyTokens {
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
