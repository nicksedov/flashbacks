package middleware

import (
	"github.com/flashbacks/api-service/internal/domain"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const ContextKeyLanguage = "language"

// LanguageMiddleware extracts the user's language from session settings
// and stores it in the context for i18n message resolution
func LanguageMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Default language
		lang := "en"

		// Try to get user ID from context (set by auth middleware)
		if userID, exists := c.Get(ContextKeyUserID); exists {
			if uid, ok := userID.(uint); ok && uid > 0 {
				var settings domain.UserSettings
				if result := db.Where("user_id = ?", uid).First(&settings); result.Error == nil {
					lang = settings.Language
				}
			}
		}

		c.Set(ContextKeyLanguage, lang)
		c.Next()
	}
}

// GetLanguage returns the language stored in the context
func GetLanguage(c *gin.Context) string {
	if lang, exists := c.Get(ContextKeyLanguage); exists {
		return lang.(string)
	}
	return "en"
}
