package handler

import (
	"log"
	"net/http"

	"github.com/flashbacks/api-service/internal/application/imaging"
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

	var thumbnailStr string
	var err error

	if s.thumbnailService != nil {
		thumbnailStr, err = s.thumbnailService.GetOrGenerate(path)
	} else {
		thumbnailStr, err = imaging.GenerateThumbnail(path, s.thumbnailCache)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, dto.ThumbnailResponse{Thumbnail: thumbnailStr})
}

// handleThumbnailCacheStats возвращает статистику кэша миниатюр
func (s *Server) handleThumbnailCacheStats(c *gin.Context) {
	if s.thumbnailService == nil {
		log.Printf("Thumbnail stats requested: service is nil")
		c.JSON(http.StatusOK, thumbnail.ThumbnailStats{})
		return
	}

	stats := s.thumbnailService.Stats()
	log.Printf("Thumbnail stats: %+v", stats)
	c.JSON(http.StatusOK, stats)
}

// handleThumbnailCacheInvalidate удаляет миниатюру из кэша
func (s *Server) handleThumbnailCacheInvalidate(c *gin.Context) {
	var req dto.InvalidateThumbnailRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailService.Invalidate(req.FilePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheInvalidated)
}

// handleThumbnailCacheInvalidateAll удаляет все миниатюры из кэша
func (s *Server) handleThumbnailCacheInvalidateAll(c *gin.Context) {
	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailService.InvalidateAll(); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheAllInvalidated)
}

// handleThumbnailCacheWarmup предварительно генерирует миниатюры для файлов
func (s *Server) handleThumbnailCacheWarmup(c *gin.Context) {
	var req dto.WarmupThumbnailsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgScanNoFilesSelected))
		return
	}

	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailService.Warmup(req.FilePaths); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheWarmedUp)
}

// handleThumbnailCacheEnable включает кэш миниатюр
func (s *Server) handleThumbnailCacheEnable(c *gin.Context) {
	if s.thumbnailService == nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgThumbnailCacheNotAvailable)
		return
	}

	s.thumbnailService.Enable()
	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheEnabled)
}

// handleThumbnailCacheDisable выключает кэш миниатюр
func (s *Server) handleThumbnailCacheDisable(c *gin.Context) {
	if s.thumbnailService == nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgThumbnailCacheNotAvailable)
		return
	}

	s.thumbnailService.Disable()
	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheDisabled)
}