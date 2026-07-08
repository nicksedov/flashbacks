package helpers

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LLMFactory creates LLM clients from database settings.
type LLMFactory struct {
	db                 *gorm.DB
	maxImageMegapixels float64
}

// NewLLMFactory creates a new LLMFactory.
func NewLLMFactory(db *gorm.DB, maxImageMegapixels float64) *LLMFactory {
	return &LLMFactory{db: db, maxImageMegapixels: maxImageMegapixels}
}

// CreateChatClient creates a text/chat LLM client from the ActiveProvider (chat provider).
// Returns (ChatClient, provider, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateChatClient(c *gin.Context) (llm.ChatClient, domain.LlmProvider, bool) {
	client, provider, ok := f.createClientByProviderField(c, "active_provider")
	if !ok {
		return nil, domain.LlmProvider{}, false
	}

	chatClient, ok := llm.NewChatClient(client)
	if !ok {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgChatLlmNoChatSupport))
		return nil, domain.LlmProvider{}, false
	}

	return chatClient, provider, true
}

// CreateVLClient creates a VL (vision-language) LLM client from the VlProvider.
// Returns (Client, provider, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateVLClient(c *gin.Context) (llm.Client, domain.LlmProvider, bool) {
	return f.createClientByProviderField(c, "vl_provider")
}

// createClientByProviderField loads the LlmSettings, resolves the provider by the given
// DB column name, and creates an llm.Client. Used internally by CreateChatClient and CreateVLClient.
func (f *LLMFactory) createClientByProviderField(c *gin.Context, providerField string) (llm.Client, domain.LlmProvider, bool) {
	var settings domain.LlmSettings
	if err := f.db.First(&settings).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return nil, domain.LlmProvider{}, false
	}

	alias := settings.ActiveProvider
	if providerField == "vl_provider" {
		alias = settings.VlProvider
		if alias == "" {
			alias = settings.ActiveProvider // fallback to chat provider if VL not configured
		}
	}

	var provider domain.LlmProvider
	if err := f.db.Where("alias = ?", alias).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return nil, domain.LlmProvider{}, false
	}

	client, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, provider.Model, f.maxImageMegapixels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return nil, domain.LlmProvider{}, false
	}

	return client, provider, true
}
