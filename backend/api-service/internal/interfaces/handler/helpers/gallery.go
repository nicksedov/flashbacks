package helpers

import (
	"net/http"
	"strings"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GalleryAccess provides gallery path validation.
type GalleryAccess struct {
	db *gorm.DB
}

// NewGalleryAccess creates a new GalleryAccess.
func NewGalleryAccess(db *gorm.DB) *GalleryAccess {
	return &GalleryAccess{db: db}
}

// IsPathInGallery checks if a path is within any configured gallery folder.
func (ga *GalleryAccess) IsPathInGallery(path string) bool {
	var folders []domain.GalleryFolder
	ga.db.Find(&folders)

	for _, f := range folders {
		if strings.HasPrefix(path, f.Path+"/") || strings.HasPrefix(path, f.Path+"\\") {
			return true
		}
	}
	return false
}

// VerifyGalleryAccess returns an error response if the path is not in a gallery folder.
// Returns true if access is granted, false if denied (and error response written).
func (ga *GalleryAccess) VerifyGalleryAccess(c *gin.Context, path string) bool {
	if !ga.IsPathInGallery(path) {
		c.JSON(http.StatusForbidden, i18n.ErrorResponse(i18n.MsgImageAccessDenied))
		return false
	}
	return true
}
