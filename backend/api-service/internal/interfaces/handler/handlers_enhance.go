package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleReplaceEnhancement backs up the original image to the configured EXIF
// backup directory, then replaces it with the enhanced .enhanced temp file.
// After replacement, it updates the DB record, invalidates caches (thumbnail,
// OCR, AI tags/embeddings), regenerates the thumbnail, and re-extracts metadata.
func (s *Server) handleReplaceEnhancement(c *gin.Context) {
	var req dto.EnhancementActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	absPath, err := resolveImagePath(c, req.ImagePath, s.galleryAccess)
	if err != nil {
		return
	}

	enhancedPath := absPath + ".enhanced"
	if _, err := os.Stat(enhancedPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		return
	}

	// Copy EXIF metadata (including GPS) from the original to the enhanced
	// temp file BEFORE replacing the original.
	if s.exifClient != nil {
		if err := s.exifClient.CopyExif(context.Background(), absPath, enhancedPath); err != nil {
			fmt.Printf("handleReplaceEnhancement: EXIF copy warning: %v\n", err)
		}
	}

	// Backup original if backup directory is configured
	settings, settingsErr := s.appSettingsRepo.Get()
	if settingsErr == nil && settings.ExifBackupDir != "" {
		backupDest := filepath.Join(settings.ExifBackupDir, filepath.Base(absPath))
		if copyErr := copyFile(absPath, backupDest); copyErr != nil {
			fmt.Printf("handleReplaceEnhancement: backup warning: %v\n", copyErr)
		}
	}

	// Replace original with enhanced temp file
	if err := os.Rename(enhancedPath, absPath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	// --- Post-replace: update DB, invalidate caches, regenerate thumbnail ---

	// Find the existing DB record for this image
	var imageFile domain.ImageFile
	if dbErr := s.db.Where("path = ?", absPath).First(&imageFile).Error; dbErr != nil {
		log.Printf("handleReplaceEnhancement: image not found in DB for %s: %v", absPath, dbErr)
		// Still try to update thumbnail even if DB record lookup fails
		if s.thumbnailProvider != nil {
			s.thumbnailProvider.Invalidate(absPath)
		}
		c.JSON(http.StatusOK, dto.EnhancementActionResponse{
			Success: true,
			Message: "Enhancement applied — original replaced (DB sync skipped: image not in database)",
		})
		return
	}

	// Recompute hash and update DB record
	fileInfo, statErr := os.Stat(absPath)
	if statErr != nil {
		log.Printf("handleReplaceEnhancement: failed to stat replaced file %s: %v", absPath, statErr)
	} else {
		hash, hashErr := imaging.CalculateFileHash(absPath)
		if hashErr != nil {
			log.Printf("handleReplaceEnhancement: failed to hash replaced file %s: %v", absPath, hashErr)
		} else {
			imageFile.Size = fileInfo.Size()
			imageFile.Hash = hash
			imageFile.ModTime = fileInfo.ModTime()
			if saveErr := s.db.Save(&imageFile).Error; saveErr != nil {
				log.Printf("handleReplaceEnhancement: failed to update DB record for %s: %v", absPath, saveErr)
			}
		}
	}

	// Invalidate and regenerate thumbnail for the replaced file
	if s.thumbnailProvider != nil {
		s.thumbnailProvider.Invalidate(absPath)
		if _, thumbErr := s.thumbnailProvider.GetOrGenerate(absPath); thumbErr != nil {
			log.Printf("handleReplaceEnhancement: thumbnail regeneration warning for %s: %v", absPath, thumbErr)
		}
	}

	// Invalidate OCR classification, AI tags, and embeddings (async — content has changed)
	if s.backgroundSync != nil {
		s.backgroundSync.ExtractAndSaveMetadataAsync(absPath, imageFile.ID)
		s.backgroundSync.InvalidateOCRClassificationAsync(imageFile.ID)
		s.backgroundSync.InvalidateTagsAndEmbeddingsAsync(imageFile.ID)
	}

	c.JSON(http.StatusOK, dto.EnhancementActionResponse{
		Success: true,
		Message: "Enhancement applied — original replaced",
	})
}

// handleSaveCopyEnhancement saves the enhanced .enhanced temp file as a copy
// alongside the original, using a "_enhanced" suffix with an auto-incrementing
// numeric index to avoid overwriting existing copies.
// After saving, it registers the new file in the database, generates a thumbnail,
// and triggers EXIF metadata extraction — the same steps that happen when a new
// file appears in the gallery.
func (s *Server) handleSaveCopyEnhancement(c *gin.Context) {
	var req dto.EnhancementActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	absPath, err := resolveImagePath(c, req.ImagePath, s.galleryAccess)
	if err != nil {
		return
	}

	enhancedPath := absPath + ".enhanced"
	if _, err := os.Stat(enhancedPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		return
	}

	// Build copy path: photo.jpg → photo_enhanced.jpg, photo_enhanced_2.jpg, ...
	copyPath := findAvailableCopyPath(absPath, "_enhanced")

	if err := copyFile(enhancedPath, copyPath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	// Copy EXIF metadata (including GPS) from the original to the saved copy.
	if s.exifClient != nil {
		if err := s.exifClient.CopyExif(context.Background(), absPath, copyPath); err != nil {
			fmt.Printf("handleSaveCopyEnhancement: EXIF copy warning: %v\n", err)
		}
	}

	// Clean up the temp file after saving the copy
	_ = os.Remove(enhancedPath)

	// --- Post-copy: register new file in DB, generate thumbnail, extract metadata ---

	// Register the new file in the database (same steps as background sync for new files)
	copyInfo, statErr := os.Stat(copyPath)
	if statErr != nil {
		log.Printf("handleSaveCopyEnhancement: failed to stat copy %s: %v", copyPath, statErr)
		c.JSON(http.StatusOK, dto.EnhancementActionResponse{
			Success: true,
			Message: fmt.Sprintf("Enhancement saved as copy: %s (DB registration skipped: cannot stat file)", filepath.Base(copyPath)),
		})
		return
	}

	hash, hashErr := imaging.CalculateFileHash(copyPath)
	if hashErr != nil {
		log.Printf("handleSaveCopyEnhancement: failed to hash copy %s: %v", copyPath, hashErr)
		c.JSON(http.StatusOK, dto.EnhancementActionResponse{
			Success: true,
			Message: fmt.Sprintf("Enhancement saved as copy: %s (DB registration skipped: cannot hash file)", filepath.Base(copyPath)),
		})
		return
	}

	newFile := domain.ImageFile{
		Path:    copyPath,
		Size:    copyInfo.Size(),
		Hash:    hash,
		ModTime: copyInfo.ModTime(),
	}

	if createErr := s.db.Create(&newFile).Error; createErr != nil {
		log.Printf("handleSaveCopyEnhancement: failed to create DB record for %s: %v", copyPath, createErr)
	} else {
		log.Printf("handleSaveCopyEnhancement: registered new file %s (ID=%d) in database", copyPath, newFile.ID)

		// Generate thumbnail for the new file
		if s.thumbnailProvider != nil {
			if _, thumbErr := s.thumbnailProvider.GetOrGenerate(copyPath); thumbErr != nil {
				log.Printf("handleSaveCopyEnhancement: thumbnail generation warning for %s: %v", copyPath, thumbErr)
			}
		}

		// Extract EXIF/geo metadata for the new file (async)
		if s.backgroundSync != nil {
			s.backgroundSync.ExtractAndSaveMetadataAsync(copyPath, newFile.ID)
		}
	}

	c.JSON(http.StatusOK, dto.EnhancementActionResponse{
		Success: true,
		Message: fmt.Sprintf("Enhancement saved as copy: %s", filepath.Base(copyPath)),
	})
}

// handleRejectEnhancement deletes the temporary .enhanced file, leaving the original untouched.
func (s *Server) handleRejectEnhancement(c *gin.Context) {
	var req dto.EnhancementActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	absPath, err := resolveImagePath(c, req.ImagePath, s.galleryAccess)
	if err != nil {
		return
	}

	enhancedPath := absPath + ".enhanced"
	_ = os.Remove(enhancedPath)

	c.JSON(http.StatusOK, dto.EnhancementActionResponse{
		Success: true,
		Message: "Enhancement rejected",
	})
}

// findAvailableCopyPath builds a copy path with a suffix and auto-incrementing
// numeric index. For /path/photo.jpg with suffix "_enhanced":
//
//	→ /path/photo_enhanced.jpg
//	→ /path/photo_enhanced_2.jpg  (if the above exists)
//	→ /path/photo_enhanced_3.jpg  (if the above exists)
//	…
func findAvailableCopyPath(originalPath, suffix string) string {
	dir := filepath.Dir(originalPath)
	ext := filepath.Ext(originalPath)
	base := filepath.Base(originalPath)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// First candidate: name_suffix.ext
	candidate := filepath.Join(dir, nameWithoutExt+suffix+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	// Subsequent candidates: name_suffix_N.ext
	for i := 2; ; i++ {
		candidate = filepath.Join(dir, nameWithoutExt+suffix+"_"+strconv.Itoa(i)+ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// handleServeEnhancedImage serves the temporary .enhanced image file.
func (s *Server) handleServeEnhancedImage(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	absPath, err := resolveImagePath(c, imagePath, s.galleryAccess)
	if err != nil {
		return
	}

	enhancedPath := absPath + ".enhanced"
	if _, statErr := os.Stat(enhancedPath); os.IsNotExist(statErr) {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		return
	}

	c.File(enhancedPath)
}

// resolveImagePath normalizes and validates an image path against gallery folders.
// On error, it writes the error response to c and returns a non-nil error.
func resolveImagePath(c *gin.Context, imagePath string, ga *helpers.GalleryAccess) (string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(imagePath))
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return "", err
	}
	normalizedPath := filepath.ToSlash(absPath)

	if !ga.VerifyGalleryAccess(c, normalizedPath) {
		return "", fmt.Errorf("access denied")
	}

	return absPath, nil
}

// copyFile copies a file from src to dst, overwriting dst if it exists.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}
