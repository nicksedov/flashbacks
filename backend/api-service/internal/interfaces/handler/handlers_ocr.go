package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
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
			Health:     string(status.HealthStatus),
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

	var total int64
	s.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("ocr_classifications.is_text_document = true").
		Count(&total)

	var results []struct {
		ID                 uint
		ImageFileID        uint
		Path               string
		Size               int64
		Hash               string
		ModTime            time.Time
		MeanConfidence     float32
		WeightedConfidence float32
		TokenCount         int
		Angle              int
		ScaleFactor        float32
	}

	if err := s.db.Table("ocr_classifications").
		Select("image_files.id, image_files.path, image_files.size, image_files.hash, image_files.mod_time, ocr_classifications.image_file_id, ocr_classifications.mean_confidence, ocr_classifications.weighted_confidence, ocr_classifications.token_count, ocr_classifications.angle, ocr_classifications.scale_factor").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("ocr_classifications.is_text_document = true").
		Order("image_files.id").
		Offset(offset).
		Limit(pageSize).
		Find(&results).Error; err != nil {
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
	pathToIdx := make(map[string]int)
	for i, doc := range docs {
		if doc.Path != "" {
			paths = append(paths, doc.Path)
			pathToIdx[doc.Path] = i
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

	var classification domain.OcrClassification
	if err := s.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("image_files.path = ?", imagePath).
		First(&classification).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	var boxes []domain.OcrBoundingBox
	s.db.Where("classification_id = ?", classification.ID).Find(&boxes)

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