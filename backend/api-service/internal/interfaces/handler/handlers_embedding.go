package handler

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleEmbeddingStatus returns the current embedding backfill status.
func (s *Server) handleEmbeddingStatus(c *gin.Context) {
	if s.embeddingBackfill == nil {
		c.JSON(http.StatusOK, imaging.EmbeddingBackfillStatus{Running: false})
		return
	}
	status := s.embeddingBackfill.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleEmbeddingStart starts the embedding backfill process.
func (s *Server) handleEmbeddingStart(c *gin.Context) {
	if s.embeddingBackfill == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgEmbeddingManagerNotAvailable)
		return
	}

	if err := s.embeddingBackfill.Start(); err != nil {
		s.respondError(c, http.StatusConflict, i18n.MsgEmbeddingManagerNotAvailable)
		return
	}
	s.respondSuccess(c, http.StatusOK, i18n.MsgEmbeddingBackfillStarted)
}

// handleEmbeddingStop stops the embedding backfill process.
func (s *Server) handleEmbeddingStop(c *gin.Context) {
	if s.embeddingBackfill == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgEmbeddingManagerNotAvailable)
		return
	}
	s.embeddingBackfill.Stop()
	s.respondSuccess(c, http.StatusOK, i18n.MsgEmbeddingBackfillStopped)
}
