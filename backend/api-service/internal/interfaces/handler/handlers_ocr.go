package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleGetOCRStatus returns the current OCR classifier status
func (s *Server) handleGetOCRStatus(c *gin.Context) {
	if s.ocrClient == nil || !s.config.OCREnabled {
		c.JSON(http.StatusOK, dto.OCRStatusResponse{
			Status: dto.OCRStatus{
				Enabled: false,
				Health:  "disabled",
			},
		})
		return
	}

	status := s.ocrClient.GetStatus()
	c.JSON(http.StatusOK, dto.OCRStatusResponse{
		Status: dto.OCRStatus{
			Enabled:    true,
			Health:     string(status.Status),
			LastCheck:  status.LastCheck.Format(helpers.DateTimeFormat),
			Error:      status.Error,
			ServiceURL: s.config.OCRServiceURL,
		},
	})
}

// handleGetExifStatus returns the EXIF service health status.
func (s *Server) handleGetExifStatus(c *gin.Context) {
	if s.exifClient == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":    false,
			"health":     "disabled",
			"lastCheck":  "",
			"error":      "",
			"serviceURL": "",
		})
		return
	}

	health, err := s.exifClient.Health(context.Background())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":    true,
			"health":     "unhealthy",
			"lastCheck":  time.Now().Format(helpers.DateTimeFormat),
			"error":      err.Error(),
			"serviceURL": s.config.ExifServiceURL,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":    true,
		"health":     health.Status,
		"lastCheck":  time.Now().Format(helpers.DateTimeFormat),
		"error":      "",
		"serviceURL": s.config.ExifServiceURL,
	})
}

// handleStartOcrClassification starts the OCR classification process
func (s *Server) handleStartOcrClassification(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgOcrFailed))
		return
	}

	incremental := false
	if err := s.ocrManager.StartClassification(incremental); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgOcrAlreadyRunning))
		return
	}

	c.JSON(http.StatusAccepted, dto.ScanResponse{
		Message: string(i18n.MsgOcrStarted),
	})
}

// handleStartOcrClassificationIncremental starts OCR classification for new/changed files only
func (s *Server) handleStartOcrClassificationIncremental(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgOcrFailed))
		return
	}

	if err := s.ocrManager.StartClassification(true); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgOcrAlreadyRunning))
		return
	}

	c.JSON(http.StatusAccepted, dto.ScanResponse{
		Message: string(i18n.MsgOcrStarted),
	})
}

// handleStopOcrClassification requests a graceful stop of OCR classification
func (s *Server) handleStopOcrClassification(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgOcrFailed))
		return
	}

	if !s.ocrManager.IsProcessing() {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgOcrNotRunning))
		return
	}

	s.ocrManager.StopClassification()

	c.JSON(http.StatusOK, dto.ScanResponse{
		Message: "OCR classification stopping",
	})
}

// handleGetOcrClassificationStatus returns the OCR classification progress
func (s *Server) handleGetOcrClassificationStatus(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusOK, dto.OcrClassificationStatusResponse{
			Processing: false,
			Progress:   "OCR classification disabled",
		})
		return
	}

	status := s.ocrManager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleGetOcrDocuments returns paginated list of images classified as text documents
func (s *Server) handleGetOcrDocuments(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFixed)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset

	total, err := s.ocrRepo.CountDocuments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanFailed))
		return
	}

	results, err := s.ocrRepo.FindDocumentsPaginated(offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanFailed))
		return
	}

	pag := helpers.CalcPagination(page, pageSize, total)

	docs := make([]dto.OcrDocumentDTO, len(results))
	for i, r := range results {
		docs[i] = dto.OcrDocumentDTO{
			ID:                 r.ID,
			ImageFileID:        r.ImageFileID,
			Path:               r.Path,
			FileName:           filepath.Base(r.Path),
			DirPath:            filepath.Dir(r.Path),
			Size:               r.Size,
			SizeHuman:          helpers.FormatSize(r.Size),
			ModTime:            r.ModTime.Format(helpers.DateTimeFormat),
			MeanConfidence:     r.MeanConfidence,
			WeightedConfidence: r.WeightedConfidence,
			TokenCount:         r.TokenCount,
			Angle:              r.Angle,
			ScaleFactor:        r.ScaleFactor,
		}
	}

	paths := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc.Path != "" {
			paths = append(paths, doc.Path)
		}
	}
	s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
		docs[idx].Thumbnail = thumb
	})

	c.JSON(http.StatusOK, dto.OcrDocumentsResponse{
		Documents:   docs,
		Total:       int(total),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleGetOcrData returns OCR classification data and bounding boxes for a specific image
func (s *Server) handleGetOcrData(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	classification, err := s.ocrRepo.FindClassificationByPath(imagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	boxes, _ := s.ocrRepo.FindBoundingBoxesByClassificationID(classification.ID)

	boxDTOs := make([]dto.BoundingBoxDTO, len(boxes))
	for i, b := range boxes {
		boxDTOs[i] = dto.BoundingBoxDTO{
			X:          b.X,
			Y:          b.Y,
			Width:      b.Width,
			Height:     b.Height,
			Word:       b.Word,
			Confidence: b.Confidence,
		}
	}

	c.JSON(http.StatusOK, dto.OcrDataResponse{
		ImagePath:         imagePath,
		Angle:             classification.Angle,
		ScaleFactor:       classification.ScaleFactor,
		IsTextDocument:    classification.IsTextDocument,
		BoundingBoxWidth:  classification.BoundingBoxWidth,
		BoundingBoxHeight: classification.BoundingBoxHeight,
		Boxes:             boxDTOs,
	})
}
