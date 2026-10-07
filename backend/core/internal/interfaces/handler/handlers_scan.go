package handler

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleScan triggers an async scan of directories
func (s *Server) handleScan(c *gin.Context) {
	if err := s.scanManager.StartScan(); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgScanFailed))
		return
	}
	c.JSON(http.StatusAccepted, dto.ScanResponse{Message: string(i18n.MsgScanStarted)})
}

// handleFastScan triggers an async fast scan of directories
// Fast scan only computes hash when file record doesn't exist or size differs
func (s *Server) handleFastScan(c *gin.Context) {
	result := s.scanManager.FastScanGallery()
	c.JSON(http.StatusOK, dto.FastScanResponse{
		Message:   string(i18n.MsgScanStarted),
		Unchanged: result.Unchanged,
		Modified:  result.Modified,
		Created:   result.Created,
		Deleted:   result.Deleted,
		Total:     result.TotalChecked,
	})
}

// handleGetStatus returns the current scan status
func (s *Server) handleGetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.scanManager.GetStatus())
}

// handleGetSyncStatus returns the current background sync status
func (s *Server) handleGetSyncStatus(c *gin.Context) {
	if s.backgroundSync == nil {
		c.JSON(http.StatusOK, dto.SyncStatusResponse{Running: false})
		return
	}
	status := s.backgroundSync.GetStatus()
	settings := s.settingsLoader.AppSettings()
	c.JSON(http.StatusOK, dto.SyncStatusResponse{
		Running:            status.Running,
		SyncInProgress:     status.SyncInProgress,
		NextRunAt:          helpers.FormatTimeInUserTZ(status.NextRunAt, settings.SyncTimezoneOffset),
		LastSyncAt:         helpers.FormatTimeInUserTZ(status.LastSyncAt, settings.SyncTimezoneOffset),
		LastSyncNew:        status.LastSyncNew,
		LastSyncUpdated:    status.LastSyncUpdated,
		LastSyncDeleted:    status.LastSyncDeleted,
		LastSyncThumbnails: status.LastSyncThumbnails,
		ProcessedFiles:     status.ProcessedFiles,
		TotalFiles:         status.TotalFiles,
	})
}