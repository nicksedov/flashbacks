package helpers

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// BindJSON binds JSON from the request body to target.
// Returns true if binding succeeded, false if error response was written.
func BindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return false
	}
	return true
}
