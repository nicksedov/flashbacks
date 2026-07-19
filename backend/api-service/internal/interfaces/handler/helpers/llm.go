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

// CreateChatClient creates a text/chat LLM client from the chat instrument settings.
// Returns (ChatClient, provider, instrument, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateChatClient(c *gin.Context) (llm.ChatClient, domain.LlmProvider, domain.LlmInstrumentSettings, bool) {
	client, provider, instrument, ok := f.createClientByInstrument(c, domain.InstrumentChat)
	if !ok {
		return nil, domain.LlmProvider{}, domain.LlmInstrumentSettings{}, false
	}

	chatClient, ok := llm.NewChatClient(client)
	if !ok {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgChatLlmNoChatSupport))
		return nil, domain.LlmProvider{}, domain.LlmInstrumentSettings{}, false
	}

	return chatClient, provider, instrument, true
}

// CreateVLClient creates a VL (vision-language) LLM client from the VL instrument settings.
// Returns (Client, provider, instrument, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateVLClient(c *gin.Context) (llm.Client, domain.LlmProvider, domain.LlmInstrumentSettings, bool) {
	return f.createClientByInstrument(c, domain.InstrumentVL)
}

// CreateImgEditClient creates an image edit LLM client from the image_edit instrument settings.
// Returns (Client, provider, instrument, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateImgEditClient(c *gin.Context) (llm.Client, domain.LlmProvider, domain.LlmInstrumentSettings, bool) {
	return f.createClientByInstrument(c, domain.InstrumentImageEdit)
}

// createClientByInstrument loads the LlmInstrumentSettings for the given type, resolves the provider,
// and creates an llm.Client using the model from the instrument settings (not from the provider).
func (f *LLMFactory) createClientByInstrument(c *gin.Context, instrumentType domain.InstrumentType) (llm.Client, domain.LlmProvider, domain.LlmInstrumentSettings, bool) {
	var instrument domain.LlmInstrumentSettings
	if err := f.db.Where("type = ?", instrumentType).Preload("Provider").First(&instrument).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return nil, domain.LlmProvider{}, domain.LlmInstrumentSettings{}, false
	}

	// Use the model from the instrument settings, not from the provider
	client, err := llm.NewClient(instrument.Provider.Name, instrument.Provider.ApiUrl, instrument.Provider.ApiKey, instrument.Model, f.maxImageMegapixels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return nil, domain.LlmProvider{}, domain.LlmInstrumentSettings{}, false
	}

	return client, instrument.Provider, instrument, true
}
