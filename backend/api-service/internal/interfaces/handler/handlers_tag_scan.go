package handler

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleTagScanStatus returns the current tag scanning status
func (s *Server) handleTagScanStatus(c *gin.Context) {
	if s.tagScanManager == nil {
		c.JSON(http.StatusServiceUnavailable, dto.TagScanStatusResponse{
			Running:    false,
			Paused:     false,
			Enabled:    false,
			WindowOpen: false,
		})
		return
	}

	status := s.tagScanManager.GetStatus()
	c.JSON(http.StatusOK, dto.TagScanStatusResponse{
		Running:      status.Running,
		Paused:       status.Paused,
		Enabled:      status.Enabled,
		Schedule:     status.Schedule,
		WindowOpen:   status.WindowOpen,
		Scanned:      status.Progress.Scanned,
		Remaining:    status.Progress.Remaining,
		Total:        status.Progress.Total,
		CurrentImage: status.Progress.CurrentImage,
		LastError:    status.Progress.LastError,
	})
}

// handleTagScanPause pauses the tag scanning
func (s *Server) handleTagScanPause(c *gin.Context) {
	if s.tagScanManager == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgTagScanManagerNotAvailable)
		return
	}

	s.tagScanManager.RequestPause()
	s.respondSuccess(c, http.StatusOK, i18n.MsgTagScanPaused)
}

// handleTagScanResume resumes the tag scanning
func (s *Server) handleTagScanResume(c *gin.Context) {
	if s.tagScanManager == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgTagScanManagerNotAvailable)
		return
	}

	s.tagScanManager.Resume()
	s.respondSuccess(c, http.StatusOK, i18n.MsgTagScanResumed)
}