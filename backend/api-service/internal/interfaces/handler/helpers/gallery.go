package helpers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// defaultGalleryCacheTTL is the default TTL for the gallery folders cache.
const defaultGalleryCacheTTL = 30 * time.Second

// GalleryAccess provides gallery path validation with an in-memory TTL cache.
type GalleryAccess struct {
	db        *gorm.DB
	mu        sync.RWMutex
	folders   []domain.GalleryFolder
	updatedAt time.Time
	ttl       time.Duration
}

// NewGalleryAccess creates a new GalleryAccess with a 30-second cache TTL.
func NewGalleryAccess(db *gorm.DB) *GalleryAccess {
	return &GalleryAccess{
		db:  db,
		ttl: defaultGalleryCacheTTL,
	}
}

// loadFolders loads gallery folders from the database and updates the cache.
// Caller must hold ga.mu (write lock).
func (ga *GalleryAccess) loadFolders() {
	var folders []domain.GalleryFolder
	ga.db.Find(&folders)
	ga.folders = folders
	ga.updatedAt = time.Now()
}

// getFolders returns cached folders, refreshing from DB if the cache is
// empty or the TTL has expired.
func (ga *GalleryAccess) getFolders() []domain.GalleryFolder {
	ga.mu.RLock()
	if len(ga.folders) > 0 && time.Since(ga.updatedAt) < ga.ttl {
		folders := ga.folders
		ga.mu.RUnlock()
		return folders
	}
	ga.mu.RUnlock()

	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Double-check after acquiring write lock
	if len(ga.folders) == 0 || time.Since(ga.updatedAt) >= ga.ttl {
		ga.loadFolders()
	}
	return ga.folders
}

// IsPathInGallery checks if a path is within any configured gallery folder.
// Uses a TTL-based in-memory cache to avoid full table scans on every request.
func (ga *GalleryAccess) IsPathInGallery(path string) bool {
	folders := ga.getFolders()

	for _, f := range folders {
		if strings.HasPrefix(path, f.Path+"/") || strings.HasPrefix(path, f.Path+"\\") {
			return true
		}
	}
	return false
}

// Invalidate clears the cache, forcing a reload on the next IsPathInGallery call.
// Should be called after add/remove folder operations.
func (ga *GalleryAccess) Invalidate() {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.folders = nil
	ga.updatedAt = time.Time{}
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
