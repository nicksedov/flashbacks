package handler

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleGetImageMetadata returns EXIF metadata for a single image
func (s *Server) handleGetImageMetadata(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(path)
	if err != nil {
		c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: false})
		return
	}

	meta, err := s.metadataRepo.FindByImageFileID(imageFile.ID)
	if err != nil {
		c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: false})
		return
	}

	var geoLat, geoLng *float64
	var nameLocal, nameEng string
	if meta.GeolocationRef != nil {
		geoCache, err := s.metadataRepo.FindGeolocationCacheByID(*meta.GeolocationRef)
		if err == nil {
			lat := geoCache.GPSLatitude
			lng := geoCache.GPSLongitude
			geoLat = &lat
			geoLng = &lng
			nameLocal = geoCache.NameLocal
			nameEng = geoCache.NameEng
		}
	}

	metaDTO := &dto.ImageMetadataDTO{
		Width:        meta.Width,
		Height:       meta.Height,
		Dimensions:   fmt.Sprintf("%d \u00d7 %d", meta.Width, meta.Height),
		CameraModel:  meta.CameraModel,
		LensModel:    meta.LensModel,
		ISO:          meta.ISO,
		Aperture:     meta.Aperture,
		ShutterSpeed: meta.ShutterSpeed,
		FocalLength:  meta.FocalLength,
		Orientation:  meta.Orientation,
		ColorSpace:   meta.ColorSpace,
		Software:     meta.Software,
		GPSLatitude:  geoLat,
		GPSLongitude: geoLng,
		NameLocal:    nameLocal,
		NameEng:      nameEng,
		HasGPS:       meta.GeolocationRef != nil,
		HasExif:      imaging.HasExifData(meta),
	}

	if meta.DateTaken != nil {
		metaDTO.DateTaken = meta.DateTaken.Format(helpers.DateTimeFormat)
	}

	c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: true, Metadata: metaDTO})
}

// handleGetImagesMissingExif returns paginated images that are missing EXIF data (date or GPS)
func (s *Server) handleGetImagesMissingExif(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFixed)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset

	type imageWithMetadata struct {
		domain.ImageFile
		DateTaken      *time.Time
		GeolocationRef *uint
	}

	var totalImages int64
	s.db.Table("image_files").
		Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
		Joins("LEFT JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken IS NULL OR image_metadata.geolocation_ref IS NULL").
		Count(&totalImages)

	var results []imageWithMetadata
	s.db.Table("image_files").
		Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
		Joins("LEFT JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken IS NULL OR image_metadata.geolocation_ref IS NULL").
		Order("image_files.id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&results)

	imageDTOs := make([]dto.GalleryImageDTO, len(results))
	for i, r := range results {
		missingDate := r.DateTaken == nil
		missingGps := r.GeolocationRef == nil
		imageDTOs[i] = dto.GalleryImageDTO{
			ID:          r.ID,
			Path:        r.Path,
			FileName:    filepath.Base(r.Path),
			DirPath:     filepath.Dir(r.Path),
			Size:        r.Size,
			SizeHuman:   helpers.FormatSize(r.Size),
			ModTime:     r.ModTime.Format(helpers.DateTimeFormat),
			MissingDate: missingDate,
			MissingGps:  missingGps,
		}
	}

	if len(results) > 0 {
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
	}

	pag := helpers.CalcPagination(page, pageSize, totalImages)

	c.JSON(http.StatusOK, dto.GalleryImagesResponse{
		Images:      imageDTOs,
		TotalImages: int(totalImages),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleGeocodeSearch searches for locations using the Nominatim geocoding API.
func (s *Server) handleGeocodeSearch(c *gin.Context) {
	q := c.Query("q")
	if strings.TrimSpace(q) == "" {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGeocodeQueryRequired)
		return
	}

	if s.nominatim == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgGeocodeSearchFailed)
		return
	}

	results, err := s.nominatim.Search(q)
	if err != nil {
		log.Printf("GeocodeSearch: failed for query %q: %v", q, err)
		s.respondError(c, http.StatusInternalServerError, i18n.MsgGeocodeSearchFailed)
		return
	}

	dtoResults := make([]dto.GeocodeSearchResult, len(results))
	for i, r := range results {
		dtoResults[i] = dto.GeocodeSearchResult{
			Lat:         r.Lat,
			Lon:         r.Lon,
			DisplayName: r.DisplayName,
			Type:        r.Type,
		}
	}

	s.respondJSON(c, http.StatusOK, dto.GeocodeSearchResponse{Results: dtoResults})
}

// handleUpdateGps writes GPS coordinates to a JPEG file's EXIF and updates the database.
func (s *Server) handleUpdateGps(c *gin.Context) {
	var req dto.UpdateGpsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsInvalidCoordinates)
		return
	}

	ext := strings.ToLower(filepath.Ext(req.Path))
	if ext != ".jpg" && ext != ".jpeg" {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsNotJpeg)
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(req.Path)
	if err != nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgImageNotFound)
		return
	}

	existingMeta, _ := s.metadataRepo.FindByImageFileID(imageFile.ID)
	if existingMeta == nil {
		existingMeta = &domain.ImageMetadata{}
	}

	osPath := filepath.FromSlash(req.Path)

	if err := s.exifClient.WriteGPS(context.Background(), osPath, req.Lat, req.Lng, s.settingsLoader.AppSettings().ExifBackupDir, existingMeta); err != nil {
		log.Printf("UpdateGps: WriteGPS failed for %s: %v", req.Path, err)
		if strings.Contains(err.Error(), "backup") {
			s.respondError(c, http.StatusInternalServerError, i18n.MsgGpsBackupFailed)
		} else {
			s.respondError(c, http.StatusInternalServerError, i18n.MsgGpsUpdateFailed)
		}
		return
	}

	enriched, _ := s.exifClient.EnrichMissingMetadata(context.Background(), osPath, existingMeta)

	var nameLocal, nameEng string
	var geoRef *uint
	if s.geolocationService != nil {
		geoEntry, err := s.geolocationService.ResolveGeolocation(req.Lat, req.Lng)
		if err != nil {
			log.Printf("UpdateGps: failed to resolve geolocation for %s: %v", req.Path, err)
		} else {
			nameLocal = geoEntry.NameLocal
			nameEng = geoEntry.NameEng
			geoRef = &geoEntry.ID
		}
	}

	if existingMeta.ID == 0 {
		newMeta := domain.ImageMetadata{
			ImageFileID:    imageFile.ID,
			GeolocationRef: geoRef,
			CameraModel:    existingMeta.CameraModel,
			LensModel:      existingMeta.LensModel,
			ISO:            existingMeta.ISO,
			Aperture:       existingMeta.Aperture,
			ShutterSpeed:   existingMeta.ShutterSpeed,
			FocalLength:    existingMeta.FocalLength,
			DateTaken:      existingMeta.DateTaken,
			Orientation:    existingMeta.Orientation,
			ColorSpace:     existingMeta.ColorSpace,
			Software:       existingMeta.Software,
		}
		s.metadataRepo.Create(&newMeta)
	} else {
		updates := map[string]interface{}{"geolocation_ref": geoRef}
		for k, v := range enriched {
			updates[k] = v
		}
		s.metadataRepo.Update(imageFile.ID, updates)
	}

	s.respondJSON(c, http.StatusOK, dto.UpdateGpsResponse{
		Success:   true,
		Lat:       req.Lat,
		Lng:       req.Lng,
		NameLocal: nameLocal,
		NameEng:   nameEng,
	})
}

// handleGetLocationCandidates returns suggested locations from same-day photos.
func (s *Server) handleGetLocationCandidates(c *gin.Context) {
	dateParam := c.Query("date")
	if dateParam == "" {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGeocodeDateRequired)
		return
	}

	targetDate, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsInvalidCoordinates)
		return
	}
	nextDay := targetDate.AddDate(0, 0, 1)

	type gpsRow struct {
		GPSLatitude  float64
		GPSLongitude float64
		NameLocal    string
		NameEng      string
	}

	var rows []gpsRow
	s.db.Table("image_metadata").
		Select("geolocation_caches.gps_latitude, geolocation_caches.gps_longitude, geolocation_caches.name_local, geolocation_caches.name_eng").
		Joins("JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("image_metadata.date_taken >= ? AND image_metadata.date_taken < ?", targetDate, nextDay).
		Limit(200).Find(&rows)

	if len(rows) == 0 {
		s.respondJSON(c, http.StatusOK, dto.LocationCandidatesResponse{Candidates: []dto.LocationCandidate{}})
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

	s.respondJSON(c, http.StatusOK, dto.LocationCandidatesResponse{Candidates: candidates})
}

// handleBatchUpdateGps writes GPS coordinates to multiple JPEG files' EXIF and updates the database.
func (s *Server) handleBatchUpdateGps(c *gin.Context) {
	var req dto.BatchUpdateGpsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsInvalidCoordinates)
		return
	}

	if len(req.Paths) == 0 {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgBatchGpsNoPaths)
		return
	}

	var nameLocal, nameEng string
	var geoRef *uint
	if s.geolocationService != nil {
		geoEntry, err := s.geolocationService.ResolveGeolocation(req.Lat, req.Lng)
		if err != nil {
			log.Printf("BatchUpdateGps: failed to resolve geolocation: %v", err)
		} else {
			nameLocal = geoEntry.NameLocal
			nameEng = geoEntry.NameEng
			geoRef = &geoEntry.ID
		}
	}

	var successCount, failedCount, skippedCount int
	var failedFiles []string

	for _, p := range req.Paths {
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".jpg" && ext != ".jpeg" {
			failedCount++
			failedFiles = append(failedFiles, p)
			continue
		}

		imageFile, err := s.imageFileRepo.FindByPath(p)
		if err != nil {
			failedCount++
			failedFiles = append(failedFiles, p)
			continue
		}

		existingMeta, _ := s.metadataRepo.FindByImageFileID(imageFile.ID)
		if existingMeta == nil {
			existingMeta = &domain.ImageMetadata{}
		}

		if existingMeta.GeolocationRef != nil {
			skippedCount++
			continue
		}

		osPath := filepath.FromSlash(p)

		if err := s.exifClient.WriteGPS(context.Background(), osPath, req.Lat, req.Lng, s.settingsLoader.AppSettings().ExifBackupDir, existingMeta); err != nil {
			log.Printf("BatchUpdateGps: WriteGPS failed for %s: %v", p, err)
			failedCount++
			failedFiles = append(failedFiles, p)
			continue
		}

		enriched, _ := s.exifClient.EnrichMissingMetadata(context.Background(), osPath, existingMeta)

		if existingMeta.ID == 0 {
			newMeta := domain.ImageMetadata{
				ImageFileID:    imageFile.ID,
				GeolocationRef: geoRef,
				CameraModel:    existingMeta.CameraModel,
				LensModel:      existingMeta.LensModel,
				ISO:            existingMeta.ISO,
				Aperture:       existingMeta.Aperture,
				ShutterSpeed:   existingMeta.ShutterSpeed,
				FocalLength:    existingMeta.FocalLength,
				DateTaken:      existingMeta.DateTaken,
				Orientation:    existingMeta.Orientation,
				ColorSpace:     existingMeta.ColorSpace,
				Software:       existingMeta.Software,
			}
			s.metadataRepo.Create(&newMeta)
		} else {
			updates := map[string]interface{}{"geolocation_ref": geoRef}
			for k, v := range enriched {
				updates[k] = v
			}
			s.metadataRepo.Update(imageFile.ID, updates)
		}

		successCount++
	}

	s.respondJSON(c, http.StatusOK, dto.BatchUpdateGpsResponse{
		Success:     successCount,
		Failed:      failedCount,
		Skipped:     skippedCount,
		FailedFiles: failedFiles,
		NameLocal:   nameLocal,
		NameEng:     nameEng,
		Lat:         req.Lat,
		Lng:         req.Lng,
	})
}
