package handler

import (
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/flashbacks/api-service/internal/application/geo"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleGetGalleryImages returns paginated gallery images
func (s *Server) handleGetGalleryImages(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFlexible)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset
	view := c.DefaultQuery("view", "list")
	sortOrder := c.DefaultQuery("sortOrder", "newest")
	searchQuery := c.DefaultQuery("search", "")
	dirPath := c.DefaultQuery("dirPath", "")

	// Build base query with optional search filter
	query := s.db.Model(&domain.ImageFile{})
	if searchQuery != "" {
		pattern := "%" + searchQuery + "%"
		query = query.Where("path ILIKE ?", pattern)
	}
	if dirPath != "" {
		// Filter images directly in this directory (not recursive into subdirs).
		// path LIKE 'dirPath/%' matches direct children; path NOT LIKE 'dirPath/%/%' excludes nested subdirectories.
		dirPattern := dirPath + "/%"
		subdirPattern := dirPath + "/%/%"
		query = query.Where("path LIKE ? AND path NOT LIKE ?", dirPattern, subdirPattern)
	}

	var totalImages int64
	query.Count(&totalImages)

	pag := helpers.CalcPagination(page, pageSize, totalImages)

	var files []domain.ImageFile
	if sortOrder == "oldest" {
		query.Order("mod_time ASC")
	} else if sortOrder == "none" {
		// No explicit ordering — natural/insertion order
	} else {
		query.Order("mod_time DESC")
	}
	query.Offset(offset).Limit(pageSize).Find(&files)

	imageDTOs := helpers.BuildGalleryImageDTOs(files)

	// Generate thumbnails in parallel for views that show image grids
	if (view == "thumbnails" || view == "folders" || view == "allImages") && len(files) > 0 {
		s.thumbnailBatch.GenerateThumbnailsForDTOs(imageDTOs)
	}

	c.JSON(http.StatusOK, dto.GalleryImagesResponse{
		Images:      imageDTOs,
		TotalImages: int(totalImages),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleServeImage serves a full-size image file
func (s *Server) handleServeImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Security: normalize and resolve path to prevent path traversal attacks
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}
	normalizedPath := filepath.ToSlash(absPath)

	// Security: verify the path is within a gallery folder
	if !s.galleryAccess.VerifyGalleryAccess(c, normalizedPath) {
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		return
	}

	c.File(absPath)
}

// handleServeOcrImage serves an image scaled and rotated for OCR overlay display
func (s *Server) handleServeOcrImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	angleStr := c.DefaultQuery("angle", "0")
	scaleFactorStr := c.DefaultQuery("scaleFactor", "1")

	angle, err := strconv.ParseFloat(angleStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	scaleFactor, err := strconv.ParseFloat(scaleFactorStr, 64)
	if err != nil || scaleFactor <= 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Security: normalize and resolve path to prevent path traversal attacks
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}
	normalizedPath := filepath.ToSlash(absPath)

	// Security: verify the path is within a gallery folder
	if !s.galleryAccess.VerifyGalleryAccess(c, normalizedPath) {
		return
	}

	data, err := imaging.PrepareOcrImage(absPath, scaleFactor, angle)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		} else {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		}
		return
	}

	c.Data(http.StatusOK, "image/webp", data)
}

// handleGetGalleryCalendar returns paginated gallery images grouped by date taken
func (s *Server) handleGetGalleryCalendar(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	monthYear := c.Query("monthYear")
	sortOrder := c.DefaultQuery("sortOrder", "oldest")

	type imageWithDate struct {
		domain.ImageFile
		DateTaken      time.Time
		GeolocationRef *uint
	}

	baseQuery := s.db.Table("image_files").
		Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken IS NOT NULL")

	if startDate != "" {
		if t, err := time.Parse(helpers.DateOnlyFormat, startDate); err == nil {
			baseQuery = baseQuery.Where("image_metadata.date_taken >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse(helpers.DateOnlyFormat, endDate); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			baseQuery = baseQuery.Where("image_metadata.date_taken <= ?", endOfDay)
		}
	}

	var totalImages int64
	baseQuery.Count(&totalImages)

	cursorParam := c.Query("cursor")
	var results []imageWithDate
	var nextCursor *string

	if cursorParam != "" {
		decodedDate, decodedID, err := helpers.DecodeCursor(cursorParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidCursor))
			return
		}

		var cursorDate time.Time
		if len(decodedDate) > 10 {
			cursorDate, err = time.Parse(helpers.DateTimeFormat, decodedDate)
		} else {
			cursorDate, err = time.Parse(helpers.DateOnlyFormat, decodedDate)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidCursor))
			return
		}

		pageSize := 50
		if ps := c.Query("pageSize"); ps != "" {
			if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 200 {
				pageSize = parsed
			}
		}

		orderClause := "image_metadata.date_taken ASC, image_metadata.image_file_id ASC"
		if sortOrder == "newest" {
			orderClause = "image_metadata.date_taken DESC, image_metadata.image_file_id DESC"
		}

		cursorQuery := s.db.Table("image_files").
			Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
			Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
			Where("image_metadata.date_taken IS NOT NULL")

		if startDate != "" {
			if t, err := time.Parse(helpers.DateOnlyFormat, startDate); err == nil {
				cursorQuery = cursorQuery.Where("image_metadata.date_taken >= ?", t)
			}
		}
		if endDate != "" {
			if t, err := time.Parse(helpers.DateOnlyFormat, endDate); err == nil {
				endOfDay := t.Add(24*time.Hour - time.Second)
				cursorQuery = cursorQuery.Where("image_metadata.date_taken <= ?", endOfDay)
			}
		}

		query := cursorQuery.Order(orderClause).Limit(pageSize + 1)

		if sortOrder == "newest" {
			query = query.Where(
				"(image_metadata.date_taken < ?) OR (image_metadata.date_taken = ? AND image_metadata.image_file_id < ?)",
				cursorDate, cursorDate, decodedID,
			)
		} else {
			query = query.Where(
				"(image_metadata.date_taken > ?) OR (image_metadata.date_taken = ? AND image_metadata.image_file_id > ?)",
				cursorDate, cursorDate, decodedID,
			)
		}

		query.Find(&results)

		if len(results) > pageSize {
			overflowItem := results[pageSize]
			lastKept := results[pageSize-1]

			if overflowItem.DateTaken.Format(helpers.DateOnlyFormat) == lastKept.DateTaken.Format(helpers.DateOnlyFormat) {
				var extra []imageWithDate
				if sortOrder == "newest" {
					cursorQuery.Where(
						"image_metadata.date_taken::date = ? AND image_files.id < ?",
						lastKept.DateTaken.Format(helpers.DateOnlyFormat), lastKept.ID,
					).Order(orderClause).Find(&extra)
				} else {
					cursorQuery.Where(
						"image_metadata.date_taken::date = ? AND image_files.id > ?",
						lastKept.DateTaken.Format(helpers.DateOnlyFormat), lastKept.ID,
					).Order(orderClause).Find(&extra)
				}
				results = append(results[:pageSize], extra...)
				lastResult := results[len(results)-1]
				cursorStr := helpers.EncodeCursor(lastResult.DateTaken.Format(helpers.DateTimeFormat), lastResult.ID)
				nextCursor = &cursorStr
			} else {
				cursorStr := helpers.EncodeCursor(lastKept.DateTaken.Format(helpers.DateTimeFormat), lastKept.ID)
				nextCursor = &cursorStr
				results = results[:pageSize]
			}
		}
	} else {
		params := helpers.ParsePagination(c, helpers.ModeFlexible)
		pageSize := params.PageSize
		offset := params.Offset

		orderClause := "image_metadata.date_taken ASC, image_metadata.image_file_id ASC"
		if sortOrder == "newest" {
			orderClause = "image_metadata.date_taken DESC, image_metadata.image_file_id DESC"
		}

		baseQuery.Order(orderClause).Offset(offset).Limit(pageSize + 1).Find(&results)

		if len(results) > pageSize {
			overflowItem := results[pageSize]
			lastKept := results[pageSize-1]

			if overflowItem.DateTaken.Format(helpers.DateOnlyFormat) == lastKept.DateTaken.Format(helpers.DateOnlyFormat) {
				legacyQuery := s.db.Table("image_files").
					Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
					Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
					Where("image_metadata.date_taken IS NOT NULL")

				if startDate != "" {
					if t, err := time.Parse(helpers.DateOnlyFormat, startDate); err == nil {
						legacyQuery = legacyQuery.Where("image_metadata.date_taken >= ?", t)
					}
				}
				if endDate != "" {
					if t, err := time.Parse(helpers.DateOnlyFormat, endDate); err == nil {
						endOfDay := t.Add(24*time.Hour - time.Second)
						legacyQuery = legacyQuery.Where("image_metadata.date_taken <= ?", endOfDay)
					}
				}

				var extra []imageWithDate
				if sortOrder == "newest" {
					legacyQuery.Where(
						"image_metadata.date_taken::date = ? AND image_files.id < ?",
						lastKept.DateTaken.Format(helpers.DateOnlyFormat), lastKept.ID,
					).Order(orderClause).Find(&extra)
				} else {
					legacyQuery.Where(
						"image_metadata.date_taken::date = ? AND image_files.id > ?",
						lastKept.DateTaken.Format(helpers.DateOnlyFormat), lastKept.ID,
					).Order(orderClause).Find(&extra)
				}
				results = append(results[:pageSize], extra...)
				lastResult := results[len(results)-1]
				cursorStr := helpers.EncodeCursor(lastResult.DateTaken.Format(helpers.DateTimeFormat), lastResult.ID)
				nextCursor = &cursorStr
			} else {
				cursorStr := helpers.EncodeCursor(lastKept.DateTaken.Format(helpers.DateTimeFormat), lastKept.ID)
				nextCursor = &cursorStr
				results = results[:pageSize]
			}
		}
	}

	type dateGroup struct {
		date   time.Time
		images []imageWithDate
	}
	groupsMap := make(map[string]*dateGroup)
	var dateOrder []string

	for _, r := range results {
		dateStr := r.DateTaken.Format(helpers.DateOnlyFormat)
		if _, ok := groupsMap[dateStr]; !ok {
			groupsMap[dateStr] = &dateGroup{date: r.DateTaken}
			dateOrder = append(dateOrder, dateStr)
		}
		groupsMap[dateStr].images = append(groupsMap[dateStr].images, r)
	}

	groupDTOs := make([]dto.CalendarDateGroup, 0, len(dateOrder))
	for _, dateStr := range dateOrder {
		g := groupsMap[dateStr]
		imageDTOs := make([]dto.GalleryImageDTO, len(g.images))
		for i, r := range g.images {
			missingGps := r.GeolocationRef == nil
			imageDTOs[i] = dto.GalleryImageDTO{
				ID:         r.ID,
				Path:       r.Path,
				FileName:   filepath.Base(r.Path),
				DirPath:    filepath.Dir(r.Path),
				Size:       r.Size,
				SizeHuman:  helpers.FormatSize(r.Size),
				ModTime:    r.ModTime.Format(helpers.DateTimeFormat),
				MissingGps: missingGps,
			}
		}

		if len(g.images) > 0 {
			paths := make([]string, len(g.images))
			for i, r := range g.images {
				paths[i] = r.Path
			}
			s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
				imageDTOs[idx].Thumbnail = thumb
			})
		}

		label := g.date.Format("Monday, January 2, 2006")

		groupDTOs = append(groupDTOs, dto.CalendarDateGroup{
			Date:       dateStr,
			Label:      label,
			ImageCount: len(g.images),
			Images:     imageDTOs,
		})
	}

	var dateRange dto.CalendarDateRange
	var minDate, maxDate *time.Time
	s.db.Raw("SELECT MIN(date_taken), MAX(date_taken) FROM image_metadata WHERE date_taken IS NOT NULL").Row().Scan(&minDate, &maxDate)
	if minDate != nil {
		dateRange.MinDate = minDate.Format(helpers.DateOnlyFormat)
	}
	if maxDate != nil {
		dateRange.MaxDate = maxDate.Format(helpers.DateOnlyFormat)
	}
	dateRange.TotalWithDate = int(totalImages)

	var months []dto.CalendarMonthInfo
	if monthYear != "" {
		if t, err := time.Parse(helpers.YearMonthFormat, monthYear); err == nil {
			year := t.Year()
			month := int(t.Month())
			nextMonth := t.AddDate(0, 1, 0)

			var days []int
			s.db.Raw(`
				SELECT DISTINCT CAST(EXTRACT(DAY FROM date_taken) AS INTEGER) as day
				FROM image_metadata
				WHERE date_taken >= $1 AND date_taken < $2 AND date_taken IS NOT NULL
				ORDER BY day
			`, t, nextMonth).Pluck("day", &days)

			months = append(months, dto.CalendarMonthInfo{
				Year:  year,
				Month: month,
				Days:  days,
			})
		}
	}

	hasMore := nextCursor != nil
	if cursorParam == "" {
		params := helpers.ParsePagination(c, helpers.ModeFlexible)
		pag := helpers.CalcPagination(params.Page, params.PageSize, totalImages)
		hasMore = pag.HasNextPage
	}

	c.JSON(http.StatusOK, dto.GalleryCalendarResponse{
		Groups:      groupDTOs,
		TotalImages: int(totalImages),
		TotalGroups: len(groupDTOs),
		HasMore:     hasMore,
		DateRange:   dateRange,
		Months:      months,
		NextCursor:  nextCursor,
	})
}

// handleGetCalendarAllDates returns all dates that have images with their counts (lightweight, no thumbnails)
func (s *Server) handleGetCalendarAllDates(c *gin.Context) {
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var minDate, maxDate *time.Time
	s.db.Raw("SELECT MIN(im.date_taken), MAX(im.date_taken) FROM image_files f INNER JOIN image_metadata im ON im.image_file_id = f.id WHERE im.date_taken IS NOT NULL").Row().Scan(&minDate, &maxDate)

	if minDate == nil || maxDate == nil {
		c.JSON(http.StatusOK, dto.CalendarAllDatesResponse{
			MinDate: "",
			MaxDate: "",
			Dates:   []dto.TimelineDateMarker{},
		})
		return
	}

	minDateStr := minDate.Format(helpers.DateOnlyFormat)
	maxDateStr := maxDate.Format(helpers.DateOnlyFormat)

	type dateCount struct {
		Date  time.Time
		Count int64
	}
	var dateCounts []dateCount
	s.db.Raw(`
		SELECT DATE(im.date_taken) as date, COUNT(*) as count
		FROM image_files f
		INNER JOIN image_metadata im ON im.image_file_id = f.id
		WHERE im.date_taken IS NOT NULL
		GROUP BY DATE(im.date_taken)
		ORDER BY date ASC
	`).Scan(&dateCounts)

	dates := make([]dto.TimelineDateMarker, 0, len(dateCounts))
	imageIndex := 0
	for _, dc := range dateCounts {
		page := (imageIndex / pageSize) + 1
		cursor := helpers.EncodeCursor(dc.Date.Format(helpers.DateTimeFormat), 1)
		dates = append(dates, dto.TimelineDateMarker{
			Date:       dc.Date.Format(helpers.DateOnlyFormat),
			ImageCount: int(dc.Count),
			Page:       page,
			Cursor:     cursor,
		})
		imageIndex += int(dc.Count)
	}

	c.JSON(http.StatusOK, dto.CalendarAllDatesResponse{
		MinDate: minDateStr,
		MaxDate: maxDateStr,
		Dates:   dates,
	})
}

// handleGetCalendarMonthInfo returns days with image counts for a specific month (lightweight, no thumbnails)
func (s *Server) handleGetCalendarMonthInfo(c *gin.Context) {
	monthYear := c.Query("monthYear")
	if monthYear == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarMonthYearRequired))
		return
	}

	t, err := time.Parse(helpers.YearMonthFormat, monthYear)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidMonthYear))
		return
	}

	year := t.Year()
	month := int(t.Month())
	nextMonth := t.AddDate(0, 1, 0)

	type dayCount struct {
		Day   int `json:"day"`
		Count int `json:"count"`
	}
	var dayCounts []dayCount
	s.db.Raw(`
		SELECT
			CAST(EXTRACT(DAY FROM date_taken) AS INTEGER) as day,
			COUNT(*) as count
		FROM image_metadata
		WHERE date_taken >= $1 AND date_taken < $2 AND date_taken IS NOT NULL
		GROUP BY EXTRACT(DAY FROM date_taken)
		ORDER BY day
	`, t, nextMonth).Scan(&dayCounts)

	days := make([]int, 0, len(dayCounts))
	for _, dc := range dayCounts {
		days = append(days, dc.Day)
	}

	var totalInMonth int
	s.db.Raw(`
		SELECT COUNT(*) FROM image_metadata
		WHERE date_taken >= $1 AND date_taken < $2 AND date_taken IS NOT NULL
	`, t, nextMonth).Scan(&totalInMonth)

	c.JSON(http.StatusOK, gin.H{
		"year":      year,
		"month":     month,
		"days":      days,
		"dayCounts": dayCounts,
		"total":     totalInMonth,
	})
}

// handleGetCalendarSeek returns a cursor pointing to a specific date
func (s *Server) handleGetCalendarSeek(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarMonthYearRequired))
		return
	}

	requestedDate, err := time.Parse(helpers.DateOnlyFormat, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidMonthYear))
		return
	}

	var firstResult struct {
		ID        uint
		DateTaken time.Time
	}

	err = s.db.Table("image_files").
		Select("image_files.id, image_metadata.date_taken").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("DATE(image_metadata.date_taken) = ?", requestedDate).
		Order("image_files.id ASC").
		First(&firstResult).Error

	if err == nil {
		var preID uint
		if firstResult.ID > 0 {
			preID = firstResult.ID - 1
		}
		c.JSON(http.StatusOK, dto.CalendarSeekResponse{
			Cursor:     helpers.EncodeCursor(firstResult.DateTaken.Format(helpers.DateOnlyFormat), preID),
			ActualDate: firstResult.DateTaken.Format(helpers.DateOnlyFormat),
		})
		return
	}

	var nearestDate time.Time
	var nearestID uint

	var nextResult struct {
		ID        uint
		DateTaken time.Time
	}
	err = s.db.Table("image_files").
		Select("image_files.id, image_metadata.date_taken").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken > ?", requestedDate).
		Order("image_metadata.date_taken ASC, image_files.id ASC").
		First(&nextResult).Error

	if err == nil {
		nearestDate = nextResult.DateTaken
		nearestID = nextResult.ID
	} else {
		var prevResult struct {
			ID        uint
			DateTaken time.Time
		}
		err = s.db.Table("image_files").
			Select("image_files.id, image_metadata.date_taken").
			Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
			Where("image_metadata.date_taken < ?", requestedDate).
			Order("image_metadata.date_taken DESC, image_files.id ASC").
			First(&prevResult).Error

		if err == nil {
			nearestDate = prevResult.DateTaken
			nearestID = prevResult.ID
		} else {
			c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
			return
		}
	}

	c.JSON(http.StatusOK, dto.CalendarSeekResponse{
		Cursor:     helpers.EncodeCursor(nearestDate.Format(helpers.DateTimeFormat), nearestID),
		ActualDate: nearestDate.Format(helpers.DateOnlyFormat),
	})
}

// handleGetGalleryClusters returns clustered image markers for map view
func (s *Server) handleGetGalleryClusters(c *gin.Context) {
	minLat, _ := strconv.ParseFloat(c.Query("minLat"), 64)
	maxLat, _ := strconv.ParseFloat(c.Query("maxLat"), 64)
	minLng, _ := strconv.ParseFloat(c.Query("minLng"), 64)
	maxLng, _ := strconv.ParseFloat(c.Query("maxLng"), 64)
	zoom, _ := strconv.Atoi(c.DefaultQuery("zoom", "2"))
	width, _ := strconv.Atoi(c.DefaultQuery("width", "800"))
	height, _ := strconv.Atoi(c.DefaultQuery("height", "600"))

	minLat = math.Max(-90, math.Min(90, minLat))
	maxLat = math.Max(-90, math.Min(90, maxLat))
	for minLng < -180 {
		minLng += 360
	}
	for minLng > 180 {
		minLng -= 360
	}
	for maxLng < -180 {
		maxLng += 360
	}
	for maxLng > 180 {
		maxLng -= 360
	}
	if minLng > maxLng {
		minLng = -180
		maxLng = 180
	}
	if minLat > maxLat {
		minLat, maxLat = maxLat, minLat
	}
	if zoom < 0 || zoom > 20 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgGeoInvalidZoom))
		return
	}
	if width <= 0 || height <= 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgGeoInvalidDimensions))
		return
	}

	params := geo.ClusterParams{
		MinLat:         minLat,
		MaxLat:         maxLat,
		MinLng:         minLng,
		MaxLng:         maxLng,
		Zoom:           zoom,
		ViewportWidth:  width,
		ViewportHeight: height,
	}

	clusters, totalImages, err := geo.ComputeClusters(s.db, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgGeoClusterFailed))
		return
	}

	s.clusterStorage.StoreClusters(clusters)

	for i := range clusters {
		clusters[i].ImagePaths = nil
	}

	c.JSON(http.StatusOK, dto.GeoClustersResponse{
		Clusters:    clusters,
		TotalImages: totalImages,
	})
}

// handleGetGeoImages returns paginated images within geographic bounds or by cluster ID
func (s *Server) handleGetGeoImages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	clusterID := c.Query("clusterId")

	if clusterID != "" {
		s.handleGetGeoImagesByCluster(c, clusterID, page, pageSize)
		return
	}

	s.handleGetGeoImagesByBounds(c, page, pageSize)
}

// handleGetGeoImagesByCluster returns images for a specific cluster
func (s *Server) handleGetGeoImagesByCluster(c *gin.Context, clusterID string, page, pageSize int) {
	imagePaths, found := s.clusterStorage.GetClusterImagePaths(clusterID)
	if !found {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgGeoClusterNotFound))
		return
	}

	totalImages := len(imagePaths)
	totalPages := (totalImages + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > totalImages {
		end = totalImages
	}

	paginatedPaths := imagePaths[offset:end]

	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.*").
		Where("image_files.path IN ?", paginatedPaths).
		Order("image_files.path").
		Find(&files)

	pathToDTO := make(map[string]int)
	for i, path := range paginatedPaths {
		pathToDTO[path] = i
	}
	imageDTOs := make([]dto.GalleryImageDTO, len(files))
	for _, f := range files {
		if idx, ok := pathToDTO[f.Path]; ok {
			imageDTOs[idx] = dto.GalleryImageDTO{
				ID:        f.ID,
				Path:      f.Path,
				FileName:  filepath.Base(f.Path),
				DirPath:   filepath.Dir(f.Path),
				Size:      f.Size,
				SizeHuman: helpers.FormatSize(f.Size),
				ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
			}
		}
	}

	validDTOs := make([]dto.GalleryImageDTO, 0, len(imageDTOs))
	for _, d := range imageDTOs {
		if d.Path != "" {
			validDTOs = append(validDTOs, d)
		}
	}
	imageDTOs = validDTOs

	if len(imageDTOs) > 0 {
		paths := make([]string, len(imageDTOs))
		for i, imgDTO := range imageDTOs {
			paths[i] = imgDTO.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
	}

	c.JSON(http.StatusOK, dto.GeoImagesResponse{
		Images:      imageDTOs,
		TotalImages: totalImages,
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasNextPage: page < totalPages,
	})
}

// handleGetGeoImagesByBounds returns paginated images within geographic bounds
func (s *Server) handleGetGeoImagesByBounds(c *gin.Context, page, pageSize int) {
	minLat, _ := strconv.ParseFloat(c.Query("minLat"), 64)
	maxLat, _ := strconv.ParseFloat(c.Query("maxLat"), 64)
	minLng, _ := strconv.ParseFloat(c.Query("minLng"), 64)
	maxLng, _ := strconv.ParseFloat(c.Query("maxLng"), 64)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var totalImages int64
	s.db.Table("image_files").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Joins("INNER JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("geolocation_caches.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
		Where("geolocation_caches.gps_longitude BETWEEN ? AND ?", minLng, maxLng).
		Count(&totalImages)

	pag := helpers.CalcPagination(page, pageSize, totalImages)
	offset := (page - 1) * pageSize

	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.*").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Joins("INNER JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("geolocation_caches.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
		Where("geolocation_caches.gps_longitude BETWEEN ? AND ?", minLng, maxLng).
		Order("image_files.path").
		Offset(offset).
		Limit(pageSize).
		Find(&files)

	imageDTOs := helpers.BuildGalleryImageDTOs(files)

	if len(files) > 0 {
		s.thumbnailBatch.GenerateThumbnailsForDTOs(imageDTOs)
	}

	c.JSON(http.StatusOK, dto.GeoImagesResponse{
		Images:      imageDTOs,
		TotalImages: int(totalImages),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}
