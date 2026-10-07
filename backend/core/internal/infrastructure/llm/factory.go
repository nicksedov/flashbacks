package llm

import "fmt"

// NewClient creates an LLM client based on provider type
func NewClient(provider, baseURL, apiKey, model string, maxImageMegapixels float64) (Client, error) {
	switch provider {
	case ProviderOllama:
		return NewOllamaClient(baseURL, "", model, maxImageMegapixels), nil
	case ProviderOllamaCloud:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for Ollama Cloud provider")
		}
		return NewOllamaClient(baseURL, apiKey, model, maxImageMegapixels), nil
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for OpenAI provider")
		}
		return NewOpenAIClient(baseURL, apiKey, model, maxImageMegapixels), nil
	case ProviderDeepSeek:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for DeepSeek provider")
		}
		return NewDeepSeekClient(baseURL, apiKey, model), nil
	case ProviderAlibaba:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for Alibaba Cloud provider")
		}
		return NewAlibabaClient(baseURL, apiKey, model, maxImageMegapixels), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}
