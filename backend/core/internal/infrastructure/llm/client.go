package llm

import "context"

// Client interface for VL LLM communication
type Client interface {
	// Recognize performs image recognition with given system and user prompts
	// Returns response content and error
	Recognize(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error)

	// ListModels returns a list of available models from the LLM server
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// ImageEditor is an optional interface for clients that support native
// image-to-image editing (quality enhancement, style transfer, etc.).
// Clients that implement this interface use a dedicated image editing API
// (e.g. DashScope multimodal generation) instead of the general-purpose
// chat completions endpoint.
type ImageEditor interface {
	// EditImage performs image-to-image editing and returns the result
	// as a data URL string (data:image/...;base64,...).
	EditImage(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error)
}

// Provenance levels describe how much of a model's metadata the provider source
// actually reported. They decide whether a collected catalog may overwrite (or
// delete) what is already stored: only a "rich" source is authoritative.
const (
	// ProvenanceRich means the provider described the model itself (capabilities,
	// modalities, context window and/or prices), e.g. a native model catalog.
	ProvenanceRich = "rich"
	// ProvenancePartial means the provider reported some fields (for example a
	// capability list or a context window) but not a complete description.
	ProvenancePartial = "partial"
	// ProvenanceHeuristic means only the model identifier is known; everything
	// else was inferred from the identifier and the curated registry.
	ProvenanceHeuristic = "heuristic"
)

// ModelInfo represents information about an available LLM model
type ModelInfo struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Size               int64    `json:"size,omitempty"`
	ContextLength      int      `json:"contextLength,omitempty"`      // 0 = unknown
	Capabilities       []string `json:"capabilities,omitempty"`       // e.g. ["chat","tool_calling","vision","embedding","image_edit"]
	InputModalities    []string `json:"inputModalities,omitempty"`    // e.g. ["text","image"]; empty = unknown
	OutputModalities   []string `json:"outputModalities,omitempty"`   // e.g. ["text"]; empty = unknown
	InputPricePerMTok  float64  `json:"inputPricePerMTok,omitempty"`  // USD per 1M tokens; 0 = unknown
	OutputPricePerMTok float64  `json:"outputPricePerMTok,omitempty"` // USD per 1M tokens; 0 = unknown
	// Provenance records how the metadata above was obtained (one of the
	// Provenance* constants). It is internal: it never reaches the API response.
	Provenance string `json:"-"`
}

// Provider type enumeration
const (
	ProviderOllama      = "ollama"
	ProviderOllamaCloud = "ollama_cloud"
	ProviderOpenAI      = "openai"
	ProviderDeepSeek    = "deepseek"
	ProviderAlibaba     = "alibaba"
)
