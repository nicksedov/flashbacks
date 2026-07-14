package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleAcceptEnhancement backs up the original image to the configured EXIF
// backup directory, then replaces it with the enhanced .enhanced temp file.
func (s *Server) handleAcceptEnhancement(c *gin.Context) {
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

	// Backup original if backup directory is configured
	settings, settingsErr := s.appSettingsRepo.Get()
	if settingsErr == nil && settings.ExifBackupDir != "" {
		backupDest := filepath.Join(settings.ExifBackupDir, filepath.Base(absPath))
		if copyErr := copyFile(absPath, backupDest); copyErr != nil {
			fmt.Printf("handleAcceptEnhancement: backup warning: %v\n", copyErr)
		}
	}

	// Replace original with enhanced temp file
	if err := os.Rename(enhancedPath, absPath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, dto.EnhancementActionResponse{
		Success: true,
		Message: "Enhancement accepted and applied",
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
