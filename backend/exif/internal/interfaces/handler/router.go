package handler

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"exif/internal/application"
	"exif/internal/domain"
	"exif/internal/interfaces/dto"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler holds all dependencies for the EXIF service REST API.
type Handler struct {
	db          *gorm.DB
	exifService *application.ExifService
	gpsWriter   *application.GPSWriter
	startTime   time.Time
}

// NewHandler creates a new handler instance.
func NewHandler(db *gorm.DB, exifSvc *application.ExifService, gpsWriter *application.GPSWriter) *Handler {
	return &Handler{
		db:          db,
		exifService: exifSvc,
		gpsWriter:   gpsWriter,
		startTime:   time.Now(),
	}
}

// HandleHealth returns the service health status.
func (h *Handler) HandleHealth(c *gin.Context) {
	dbConnected := true
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		dbConnected = false
	}

	c.JSON(http.StatusOK, dto.HealthResponse{
		Status:            "healthy",
		Version:           "1.0.0",
		ExiftoolAvailable: h.exifService.IsAvailable(),
		DatabaseConnected: dbConnected,
		Uptime:            time.Since(h.startTime).Round(time.Second).String(),
	})
}

// HandleGetPreview extracts an embedded preview/thumbnail image from a file
// using exiftool. This is useful for large/panoramic JPEGs that Go's standard
// image.Decode cannot handle. Returns raw image bytes with content-type header.
func (h *Handler) HandleGetPreview(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "path query parameter is required"})
		return
	}

	data, err := h.exifService.ExtractPreviewImage(filepath.FromSlash(path))
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/jpeg", data)
}

// HandleGetMetadata reads EXIF metadata from a file or from DB by imageFileId.
// Supports two mutually exclusive query parameters:
//   - path: absolute file path (reads from file)
//   - imageFileId: database ID (reads from DB)
func (h *Handler) HandleGetMetadata(c *gin.Context) {
	path := c.Query("path")
	imageFileIDStr := c.Query("imageFileId")

	// Validate: at least one parameter required, but not both
	if path == "" && imageFileIDStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "either path or imageFileId query parameter is required"})
		return
	}

	// If path is provided, read from file (existing behavior)
	if path != "" {
		meta, err := h.exifService.ExtractMetadata(filepath.FromSlash(path))
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
			return
		}

		resp := dto.MetadataResponse{
			Width:        meta.Width,
			Height:       meta.Height,
			CameraModel:  meta.CameraModel,
			LensModel:    meta.LensModel,
			ISO:          meta.ISO,
			Aperture:     meta.Aperture,
			ShutterSpeed: meta.ShutterSpeed,
			FocalLength:  meta.FocalLength,
			Orientation:  meta.Orientation,
			ColorSpace:   meta.ColorSpace,
			Software:     meta.Software,
		}

		if meta.DateTaken != nil {
			resp.DateTaken = meta.DateTaken.Format(time.RFC3339)
		}

		// Try to extract GPS
		if lat, lng, ok := h.exifService.ExtractGPS(filepath.FromSlash(path)); ok {
			resp.GPSLatitude = &lat
			resp.GPSLongitude = &lng
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	// If imageFileId is provided, read from DB
	imageFileID, err := strconv.ParseUint(imageFileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid imageFileId"})
		return
	}

	meta, err := h.exifService.GetMetadataByImageID(uint(imageFileID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if meta == nil {
		c.JSON(http.StatusOK, dto.MetadataResponse{})
		return
	}

	resp := dto.MetadataResponse{
		Width:        meta.Width,
		Height:       meta.Height,
		CameraModel:  meta.CameraModel,
		LensModel:    meta.LensModel,
		ISO:          meta.ISO,
		Aperture:     meta.Aperture,
		ShutterSpeed: meta.ShutterSpeed,
		FocalLength:  meta.FocalLength,
		Orientation:  meta.Orientation,
		ColorSpace:   meta.ColorSpace,
		Software:     meta.Software,
	}

	if meta.DateTaken != nil {
		resp.DateTaken = meta.DateTaken.Format(time.RFC3339)
	}

	// Look up geolocation data
	if meta.GeolocationRef != nil {
		var geoCache struct {
			GPSLatitude  float64
			GPSLongitude float64
		}
		if err := h.db.Table("geolocation_caches").
			Select("gps_latitude, gps_longitude").
			Where("id = ?", *meta.GeolocationRef).
			First(&geoCache).Error; err == nil {
			resp.GPSLatitude = &geoCache.GPSLatitude
			resp.GPSLongitude = &geoCache.GPSLongitude
		}
	}

	c.JSON(http.StatusOK, resp)
}

// HandleUpsertMetadata creates or updates metadata in the database.
func (h *Handler) HandleUpsertMetadata(c *gin.Context) {
	var req dto.UpsertMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	meta := MetaFromUpsertRequest(req)
	result, err := h.exifService.UpsertMetadata(meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	resp := metaToUpsertResponse(result)
	c.JSON(http.StatusOK, resp)
}

// HandleDeleteMetadata deletes metadata for a given image file ID.
func (h *Handler) HandleDeleteMetadata(c *gin.Context) {
	imageFileIDStr := c.Param("imageFileId")
	imageFileID, err := strconv.ParseUint(imageFileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid imageFileId"})
		return
	}

	if err := h.exifService.DeleteMetadata(uint(imageFileID)); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.DeleteMetadataResponse{Deleted: true})
}

// HandleGetMetadataBatch retrieves metadata for multiple image file IDs.
func (h *Handler) HandleGetMetadataBatch(c *gin.Context) {
	idsStr := c.Query("ids")
	if idsStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "ids query parameter is required (comma-separated)"})
		return
	}

	parts := strings.Split(idsStr, ",")
	ids := make([]uint, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: fmt.Sprintf("invalid id: %s", p)})
			return
		}
		ids = append(ids, uint(id))
	}

	metaMap, err := h.exifService.GetMetadataBatch(ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]dto.MetadataResponse, 0, len(metaMap))
	for _, meta := range metaMap {
		item := dto.MetadataResponse{
			Width:        meta.Width,
			Height:       meta.Height,
			CameraModel:  meta.CameraModel,
			LensModel:    meta.LensModel,
			ISO:          meta.ISO,
			Aperture:     meta.Aperture,
			ShutterSpeed: meta.ShutterSpeed,
			FocalLength:  meta.FocalLength,
			Orientation:  meta.Orientation,
			ColorSpace:   meta.ColorSpace,
			Software:     meta.Software,
		}
		if meta.DateTaken != nil {
			item.DateTaken = meta.DateTaken.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, dto.MetadataBatchResponse{Metadata: items})
}

// HandleGetMissingImages returns images missing EXIF data with pagination.
func (h *Handler) HandleGetMissingImages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	rows, total, err := h.exifService.GetMissingImages(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	images := make([]dto.MissingImageItem, 0, len(rows))
	for _, r := range rows {
		images = append(images, dto.MissingImageItem{
			ImageFileID: r.ImageFileID,
			MissingDate: r.MissingDate,
			MissingGPS:  r.MissingGPS,
		})
	}

	c.JSON(http.StatusOK, dto.MissingImagesResponse{
		Images:   images,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// HandleGetCalendar returns calendar gallery items with cursor-based pagination.
func (h *Handler) HandleGetCalendar(c *gin.Context) {
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))

	var startDate, endDate *time.Time
	if sd := c.Query("startDate"); sd != "" {
		t, err := time.Parse("2006-01-02", sd)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid startDate format, expected YYYY-MM-DD"})
			return
		}
		startDate = &t
	}
	if ed := c.Query("endDate"); ed != "" {
		t, err := time.Parse("2006-01-02", ed)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid endDate format, expected YYYY-MM-DD"})
			return
		}
		endDate = &t
	}

	// Parse cursor (base64-encoded "date_taken|image_file_id")
	var cursor *time.Time
	var cursorID *uint
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursorStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cursor"})
			return
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cursor format"})
			return
		}
		t, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cursor date"})
			return
		}
		id, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cursor id"})
			return
		}
		cursor = &t
		idUint := uint(id)
		cursorID = &idUint
	}

	rows, nextCursor, nextCursorID, err := h.exifService.GetCalendarItems(startDate, endDate, cursor, cursorID, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]dto.CalendarItem, 0, len(rows))
	for _, r := range rows {
		item := dto.CalendarItem{
			ImageFileID:    r.ImageFileID,
			GeolocationRef: r.GeolocationRef,
			GPSLatitude:    r.GPSLatitude,
			GPSLongitude:   r.GPSLongitude,
			NameLocal:      r.NameLocal,
			NameEng:        r.NameEng,
		}
		if r.DateTaken != nil {
			item.DateTaken = r.DateTaken.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	// Build next cursor
	var nextCursorStr string
	if nextCursor != nil && nextCursorID != nil {
		raw := fmt.Sprintf("%s|%d", nextCursor.Format(time.RFC3339), *nextCursorID)
		nextCursorStr = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	// Get date range and total count
	minDate, maxDate, _ := h.exifService.GetCalendarDateRange()
	totalWithDate, _ := h.exifService.GetCalendarDayCount()

	var minDateStr, maxDateStr string
	if minDate != nil {
		minDateStr = minDate.Format("2006-01-02")
	}
	if maxDate != nil {
		maxDateStr = maxDate.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, dto.CalendarResponse{
		Items:         items,
		NextCursor:    nextCursorStr,
		MinDate:       minDateStr,
		MaxDate:       maxDateStr,
		TotalWithDate: totalWithDate,
	})
}

// HandleGetGeoPoints returns GPS points for clustering within bounding box.
func (h *Handler) HandleGetGeoPoints(c *gin.Context) {
	// Parse optional bounding box
	var minLat, maxLat, minLng, maxLng float64
	var err error

	if ml := c.Query("minLat"); ml != "" {
		minLat, err = strconv.ParseFloat(ml, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid minLat"})
			return
		}
	}
	if ml := c.Query("maxLat"); ml != "" {
		maxLat, err = strconv.ParseFloat(ml, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid maxLat"})
			return
		}
	}
	if ml := c.Query("minLng"); ml != "" {
		minLng, err = strconv.ParseFloat(ml, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid minLng"})
			return
		}
	}
	if ml := c.Query("maxLng"); ml != "" {
		maxLng, err = strconv.ParseFloat(ml, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid maxLng"})
			return
		}
	}

	points, err := h.exifService.GetGeoPoints(minLat, maxLat, minLng, maxLng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]dto.GeoPointItem, 0, len(points))
	for _, p := range points {
		items = append(items, dto.GeoPointItem{
			ImageFileID:  p.ImageFileID,
			GPSLatitude:  p.GPSLatitude,
			GPSLongitude: p.GPSLongitude,
			NameLocal:    p.NameLocal,
			NameEng:      p.NameEng,
		})
	}

	c.JSON(http.StatusOK, dto.GeoPointsResponse{Points: items})
}

// HandleResolveGeolocation resolves GPS coordinates to location names.
func (h *Handler) HandleResolveGeolocation(c *gin.Context) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "lat and lng query parameters are required"})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid lat"})
		return
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid lng"})
		return
	}

	entry, err := h.exifService.ResolveGeolocation(lat, lng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.GeolocationResponse{
		ID:           entry.ID,
		GPSLatitude:  entry.GPSLatitude,
		GPSLongitude: entry.GPSLongitude,
		NameLocal:    entry.NameLocal,
		NameEng:      entry.NameEng,
	})
}

// HandleUpdateGPS writes GPS coordinates to a single image file.
func (h *Handler) HandleUpdateGPS(c *gin.Context) {
	var req dto.GPSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	osPath := filepath.FromSlash(req.Path)
	if err := h.gpsWriter.WriteGPS(osPath, req.Latitude, req.Longitude, nil, req.BackupDir); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.GPSResponse{
		Success:   true,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	})
}

// HandleBatchUpdateGPS writes GPS coordinates to multiple images.
func (h *Handler) HandleBatchUpdateGPS(c *gin.Context) {
	var req dto.GPSBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	var successCount, failedCount int
	var failedFiles []string

	for _, item := range req.Items {
		osPath := filepath.FromSlash(item.Path)
		if err := h.gpsWriter.WriteGPS(osPath, item.Latitude, item.Longitude, nil, req.BackupDir); err != nil {
			failedCount++
			failedFiles = append(failedFiles, item.Path)
			continue
		}
		successCount++
	}

	c.JSON(http.StatusOK, dto.GPSBatchResponse{
		Success:     successCount,
		Failed:      failedCount,
		FailedFiles: failedFiles,
	})
}

// HandleGetLocationCandidates returns location suggestions from same-day photos.
func (h *Handler) HandleGetLocationCandidates(c *gin.Context) {
	dateParam := c.Query("date")
	if dateParam == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "date query parameter is required (YYYY-MM-DD)"})
		return
	}

	if _, err := time.Parse("2006-01-02", dateParam); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid date format, expected YYYY-MM-DD"})
		return
	}

	targetDate, _ := time.Parse("2006-01-02", dateParam)
	nextDay := targetDate.AddDate(0, 0, 1)

	type gpsRow struct {
		GPSLatitude  float64
		GPSLongitude float64
		NameLocal    string
		NameEng      string
	}

	var rows []gpsRow
	h.db.Table("image_metadata").
		Select("geolocation_caches.gps_latitude, geolocation_caches.gps_longitude, geolocation_caches.name_local, geolocation_caches.name_eng").
		Joins("JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("image_metadata.date_taken >= ? AND image_metadata.date_taken < ?", targetDate, nextDay).
		Limit(200).
		Find(&rows)

	if len(rows) == 0 {
		c.JSON(http.StatusOK, dto.LocationCandidatesResponse{Candidates: []dto.LocationCandidate{}})
		return
	}

	type locationKey struct {
		Lat float64
		Lng float64
	}
	type locationGroup struct {
		LatSum     float64
		LngSum     float64
		NameLocal  string
		NameEng    string
		PhotoCount int
	}

	groupMap := make(map[locationKey]*locationGroup)
	var order []locationKey

	for _, r := range rows {
		roundedLat := math.Round(r.GPSLatitude*20) / 20
		roundedLng := math.Round(r.GPSLongitude*20) / 20
		key := locationKey{Lat: roundedLat, Lng: roundedLng}

		if g, ok := groupMap[key]; ok {
			g.LatSum += r.GPSLatitude
			g.LngSum += r.GPSLongitude
			g.PhotoCount++
		} else {
			groupMap[key] = &locationGroup{
				LatSum:     r.GPSLatitude,
				LngSum:     r.GPSLongitude,
				NameLocal:  r.NameLocal,
				NameEng:    r.NameEng,
				PhotoCount: 1,
			}
			order = append(order, key)
		}
	}

	candidates := make([]dto.LocationCandidate, 0, len(order))
	for i, key := range order {
		if i >= 20 {
			break
		}
		g := groupMap[key]
		candidates = append(candidates, dto.LocationCandidate{
			Lat:        g.LatSum / float64(g.PhotoCount),
			Lng:        g.LngSum / float64(g.PhotoCount),
			NameLocal:  g.NameLocal,
			NameEng:    g.NameEng,
			PhotoCount: g.PhotoCount,
		})
	}

	slices.SortFunc(candidates, func(a, b dto.LocationCandidate) int {
		return b.PhotoCount - a.PhotoCount
	})

	c.JSON(http.StatusOK, dto.LocationCandidatesResponse{Candidates: candidates})
}

// SetupRouter creates the Gin router with all EXIF service routes.
func SetupRouter(db *gorm.DB, exifSvc *application.ExifService, gpsWriter *application.GPSWriter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS, DELETE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	h := NewHandler(db, exifSvc, gpsWriter)

	exif := r.Group("/exif")
	{
		exif.GET("/health", h.HandleHealth)

		// Metadata endpoints
		exif.GET("/metadata", h.HandleGetMetadata)                    // File-based (path) or DB-based (imageFileId)
		exif.PUT("/metadata", h.HandleUpsertMetadata)                 // NEW: upsert
		exif.DELETE("/metadata/:imageFileId", h.HandleDeleteMetadata) // NEW: delete
		exif.GET("/metadata/batch", h.HandleGetMetadataBatch)         // NEW: batch get
		exif.GET("/metadata/calendar", h.HandleGetCalendar)           // NEW: calendar gallery
		exif.GET("/metadata/geo-points", h.HandleGetGeoPoints)        // NEW: GPS points for clustering

		// Preview extraction for images Go cannot decode (e.g. large panoramic JPEGs)
		exif.GET("/preview", h.HandleGetPreview)

		// Existing GPS endpoints
		exif.PUT("/gps", h.HandleUpdateGPS)
		exif.PUT("/gps/batch", h.HandleBatchUpdateGPS)

		// Missing and location-candidates
		exif.GET("/missing", h.HandleGetMissingImages)                  // NEW: paginated missing images
		exif.GET("/location-candidates", h.HandleGetLocationCandidates) // Existing

		// Geolocation
		exif.GET("/geolocation", h.HandleResolveGeolocation) // NEW: resolve geolocation
	}

	return r
}

// --- helper functions ---

// MetaFromUpsertRequest converts an UpsertMetadataRequest to a domain.ImageMetadata.
func MetaFromUpsertRequest(req dto.UpsertMetadataRequest) *domain.ImageMetadata {
	meta := &domain.ImageMetadata{
		ImageFileID:    req.ImageFileID,
		Width:          req.Width,
		Height:         req.Height,
		CameraModel:    req.CameraModel,
		LensModel:      req.LensModel,
		ISO:            req.ISO,
		Aperture:       req.Aperture,
		ShutterSpeed:   req.ShutterSpeed,
		FocalLength:    req.FocalLength,
		Orientation:    req.Orientation,
		ColorSpace:     req.ColorSpace,
		Software:       req.Software,
		GeolocationRef: req.GeolocationRef,
	}
	if req.DateTaken != nil {
		t, err := time.Parse(time.RFC3339, *req.DateTaken)
		if err == nil {
			meta.DateTaken = &t
		}
	}
	return meta
}

// metaToUpsertResponse converts a domain.ImageMetadata to an UpsertMetadataResponse DTO.
func metaToUpsertResponse(meta *domain.ImageMetadata) dto.UpsertMetadataResponse {
	resp := dto.UpsertMetadataResponse{
		ID:             meta.ID,
		ImageFileID:    meta.ImageFileID,
		Width:          meta.Width,
		Height:         meta.Height,
		CameraModel:    meta.CameraModel,
		LensModel:      meta.LensModel,
		ISO:            meta.ISO,
		Aperture:       meta.Aperture,
		ShutterSpeed:   meta.ShutterSpeed,
		FocalLength:    meta.FocalLength,
		Orientation:    meta.Orientation,
		ColorSpace:     meta.ColorSpace,
		Software:       meta.Software,
		GeolocationRef: meta.GeolocationRef,
	}
	if meta.DateTaken != nil {
		formatted := meta.DateTaken.Format(time.RFC3339)
		resp.DateTaken = &formatted
	}
	return resp
}
