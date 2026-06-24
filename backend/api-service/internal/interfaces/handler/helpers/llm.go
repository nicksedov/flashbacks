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

// CreateClient creates an LLM client from the current database settings.
// Returns (client, provider, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateClient(c *gin.Context) (llm.Client, domain.LlmProvider, bool) {
	var settings domain.LlmSettings
	if err := f.db.First(&settings).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return nil, domain.LlmProvider{}, false
	}

	var provider domain.LlmProvider
	if err := f.db.Where("alias = ?", settings.ActiveProvider).First(&provider).Error; err != nil {
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

