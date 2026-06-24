package llm

// Client interface for VL LLM communication
type Client interface {
	// Recognize performs image recognition with given system and user prompts
	// Returns response content and error
	Recognize(imagePath string, systemPrompt string, userMessage string) (string, error)

	// ListModels returns a list of available models from the LLM server
	ListModels() ([]ModelInfo, error)
}

// ModelInfo represents information about an available LLM model
type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Size          int64  `json:"size,omitempty"`
	ContextLength int    `json:"contextLength,omitempty"` // 0 = unknown
}

// Provider type enumeration
const (
	ProviderOllama      = "ollama"
	ProviderOllamaCloud = "ollama_cloud"
	ProviderOpenAI      = "openai"
)
