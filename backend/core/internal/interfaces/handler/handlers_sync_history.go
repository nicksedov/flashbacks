package handler

import (
	"net/http"
	"time"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleGetSyncHistory returns sync history entries filtered by date range.
// Query params: from (ISO 8601), to (ISO 8601). Defaults to last 7 days.
func (s *Server) handleGetSyncHistory(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	now := time.Now()
	from := now.AddDate(0, 0, -7) // default: last 7 days
	to := now

	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
			return
		}
		from = parsed
	}
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgAuthInvalidRequestFormat))
			return
		}
		to = parsed
	}

	entries, err := s.syncHistoryRepo.FindByDateRange(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.Error))
		return
	}

	// Load settings for timezone formatting
	settings := s.settingsLoader.AppSettings()

	dtos := make([]dto.SyncHistoryEntry, len(entries))
	for i, e := range entries {
		dtos[i] = dto.SyncHistoryEntry{
			ID:                  e.ID,
			CreatedAt:           helpers.FormatTimeInUserTZ(&e.CreatedAt, settings.SyncTimezoneOffset),
			NewFiles:            e.NewFiles,
			UpdatedFiles:        e.UpdatedFiles,
			DeletedFiles:        e.DeletedFiles,
			ThumbnailsGenerated: e.ThumbnailsGenerated,
		}
	}

	c.JSON(http.StatusOK, dto.SyncHistoryResponse{
		Entries: dtos,
		Total:   len(dtos),
	})
}
