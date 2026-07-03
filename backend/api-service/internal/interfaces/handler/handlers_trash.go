package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// TrashFileInfo represents a single file in trash
type TrashFileInfo struct {
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"sizeHuman"`
	ModTime   string `json:"modTime"`
}

// handleGetTrashInfo returns information about files in the trash directory
func (s *Server) handleGetTrashInfo(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	var fileCount int
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileCount++
		if fi, err := entry.Info(); err == nil {
			totalSize += fi.Size()
		}
	}

	c.JSON(http.StatusOK, dto.TrashInfoResponse{
		FileCount:      fileCount,
		TotalSize:      totalSize,
		TotalSizeHuman: helpers.FormatSize(totalSize),
	})
}

// handleCleanTrash removes all files from the trash directory
func (s *Server) handleCleanTrash(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotExists))
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashReadFailed))
		return
	}

	var deleted, failed int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(settings.TrashDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			failed++
		} else {
			deleted++
		}
	}

	c.JSON(http.StatusOK, dto.CleanTrashResponse{
		Deleted: deleted,
		Failed:  failed,
	})
}

// handleListTrashFiles returns a list of all files in the trash directory
func (s *Server) handleListTrashFiles(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	files := make([]TrashFileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if fi, err := entry.Info(); err == nil {
			files = append(files, TrashFileInfo{
				FileName:  entry.Name(),
				Size:      fi.Size(),
				SizeHuman: helpers.FormatSize(fi.Size()),
				ModTime:   fi.ModTime().Format(helpers.RFC3339Format),
			})
		}
	}

	c.JSON(http.StatusOK, files)
}

// handleRestoreTrashFile moves a file from trash back to the original location
func (s *Server) handleRestoreTrashFile(c *gin.Context) {
	var req struct {
		FileName   string `json:"fileName"`
		TargetPath string `json:"targetPath"`
	}
	if !helpers.BindJSON(c, &req) || req.FileName == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashFileNameRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	restoredPath, err := helpers.RestoreFile(settings.TrashDir, req.FileName, req.TargetPath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashRestored, "restoredPath": restoredPath})
}

// handleDeleteTrashFile permanently deletes a single file from trash
func (s *Server) handleDeleteTrashFile(c *gin.Context) {
	var req struct {
		FileName string `json:"fileName"`
	}
	if !helpers.BindJSON(c, &req) || req.FileName == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashFileNameRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	filePath := filepath.Join(settings.TrashDir, req.FileName)
	if _, err := os.Stat(filePath); err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashDeleteFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashFileDeleted})
}