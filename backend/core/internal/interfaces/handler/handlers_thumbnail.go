package handler

import (
	"log"
	"net/http"

	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleThumbnail serves a thumbnail for a specific file
func (s *Server) handleThumbnail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	if s.thumbnailProvider == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	thumbnailStr, err := s.thumbnailProvider.GetOrGenerate(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, dto.ThumbnailResponse{Thumbnail: thumbnailStr})
}

// handleThumbnailCacheStats returns thumbnail cache statistics
func (s *Server) handleThumbnailCacheStats(c *gin.Context) {
	if s.thumbnailProvider == nil {
		log.Printf("Thumbnail stats requested: provider is nil")
		c.JSON(http.StatusOK, thumbnail.ThumbnailStats{})
		return
	}

	stats, err := s.thumbnailProvider.GetStats()
	if err != nil {
		c.JSON(http.StatusOK, thumbnail.ThumbnailStats{})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// handleThumbnailCacheInvalidate removes a single thumbnail from cache
func (s *Server) handleThumbnailCacheInvalidate(c *gin.Context) {
	var req dto.InvalidateThumbnailRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	if s.thumbnailProvider == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailProvider.Invalidate(req.FilePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheInvalidated)
}

// handleThumbnailCacheInvalidateAll removes all thumbnails from cache
func (s *Server) handleThumbnailCacheInvalidateAll(c *gin.Context) {
	if s.thumbnailProvider == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailProvider.InvalidateAll(); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheAllInvalidated)
}

// handleThumbnailCacheWarmup pre-generates thumbnails for files
func (s *Server) handleThumbnailCacheWarmup(c *gin.Context) {
	var req dto.WarmupThumbnailsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgScanNoFilesSelected))
		return
	}

	if s.thumbnailProvider == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailProvider.Warmup(req.FilePaths); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheWarmedUp)
}

// handleThumbnailCacheEnable enables the thumbnail cache
func (s *Server) handleThumbnailCacheEnable(c *gin.Context) {
	if s.thumbnailProvider == nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgThumbnailCacheNotAvailable)
		return
	}

	s.thumbnailProvider.Enable()
	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheEnabled)
}

// handleThumbnailCacheDisable disables the thumbnail cache
func (s *Server) handleThumbnailCacheDisable(c *gin.Context) {
	if s.thumbnailProvider == nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgThumbnailCacheNotAvailable)
		return
	}

	s.thumbnailProvider.Disable()
	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheDisabled)
}
