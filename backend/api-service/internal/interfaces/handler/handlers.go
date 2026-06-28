package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/geo"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// handleGetDuplicates returns paginated duplicate groups as JSON
func (s *Server) handleGetDuplicates(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFixed)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset

	groups, totalGroups, totalFiles, err := imaging.FindDuplicatesPaginated(s.db, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	pag := helpers.CalcPagination(page, pageSize, int64(totalGroups))

	// Prepare group DTOs with parallel thumbnail generation
	groupDTOs := make([]dto.DuplicateGroupDTO, len(groups))
	pageFiles := 0

	for _, g := range groups {
		pageFiles += len(g.Files)
	}

	// Collect paths for thumbnail generation
	paths := make([]string, len(groups))
	pathToIdx := make(map[string]int)
	for i, g := range groups {
		fileDTOs := make([]dto.FileDTO, len(g.Files))
		for j, f := range g.Files {
			fileDTOs[j] = dto.FileDTO{
				ID:       f.ID,
				Path:     f.Path,
				FileName: filepath.Base(f.Path),
				DirPath:  filepath.Dir(f.Path),
				ModTime:  f.ModTime.Format(helpers.DateTimeFormat),
			}
		}

		groupDTOs[i] = dto.DuplicateGroupDTO{
			Index:     offset + i + 1,
			Hash:      g.Hash,
			Size:      g.Size,
			SizeHuman: formatSize(g.Size),
			Files:     fileDTOs,
		}

		if len(g.Files) > 0 {
			paths[i] = g.Files[0].Path
			pathToIdx[g.Files[0].Path] = i
		}
	}

	s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
		groupDTOs[idx].Thumbnail = thumb
	})

	// Get scanned dirs from gallery folders
	var galleryFolders []domain.GalleryFolder
	s.db.Order("created_at").Find(&galleryFolders)
	scannedDirs := make([]string, len(galleryFolders))
	for i, f := range galleryFolders {
		scannedDirs[i] = f.Path
	}

	response := dto.DuplicatesResponse{
		Groups:      groupDTOs,
		TotalFiles:  totalFiles,
		PageFiles:   pageFiles,
		TotalGroups: totalGroups,
		ScannedDirs: scannedDirs,
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasPrevPage: pag.HasPrevPage,
		HasNextPage: pag.HasNextPage,
		PageSizes:   helpers.FixedPageSizes,
	}

	c.JSON(http.StatusOK, response)
}

// handleScan triggers an async scan of directories
func (s *Server) handleScan(c *gin.Context) {
	if err := s.scanManager.StartScan(); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgScanFailed))
		return
	}
	c.JSON(http.StatusAccepted, dto.ScanResponse{Message: string(i18n.MsgScanStarted)})
}

// handleFastScan triggers an async fast scan of directories
// Fast scan only computes hash when file record doesn't exist or size differs
func (s *Server) handleFastScan(c *gin.Context) {
	result := s.scanManager.FastScanGallery()
	c.JSON(http.StatusOK, dto.FastScanResponse{
		Message:   string(i18n.MsgScanStarted),
		Unchanged: result.Unchanged,
		Modified:  result.Modified,
		Created:   result.Created,
		Deleted:   result.Deleted,
		Total:     result.TotalChecked,
	})
}

// handleGetStatus returns the current scan status
func (s *Server) handleGetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.scanManager.GetStatus())
}

// handleThumbnail serves a thumbnail for a specific file
func (s *Server) handleThumbnail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	var thumbnail string
	var err error

	// Use thumbnail service if available
	if s.thumbnailService != nil {
		thumbnail, err = s.thumbnailService.GetOrGenerate(path)
	} else {
		thumbnail, err = imaging.GenerateThumbnail(path, s.thumbnailCache)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, dto.ThumbnailResponse{Thumbnail: thumbnail})
}

// handleDeleteFiles deletes selected files directly (moves to trash)
func (s *Server) handleDeleteFiles(c *gin.Context) {
	var req dto.DeleteFilesRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgScanNoFilesSelected))
		return
	}

	result := s.fileMover.BatchProcess(req.FilePaths, req.TrashDir)

	c.JSON(http.StatusOK, dto.DeleteFilesResponse{
		Success:     result.Success,
		Failed:      result.Failed,
		FailedFiles: result.FailedFiles,
	})
}

// handleGetFolderPatterns returns all unique folder patterns from duplicates
func (s *Server) handleGetFolderPatterns(c *gin.Context) {
	groups, _, _, err := imaging.FindDuplicatesPaginated(s.db, 0, 100000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	patternMap := make(map[string]*dto.FolderPattern)
	singleFolderDuplicateCount := 0

	for _, group := range groups {
		folderSet := make(map[string]bool)
		for _, file := range group.Files {
			dir := filepath.Dir(file.Path)
			folderSet[dir] = true
		}

		// Skip groups where all duplicates are in a single folder
		// These can't be handled by batch dedup (no cross-folder choice to make)
		if len(folderSet) <= 1 {
			singleFolderDuplicateCount += len(group.Files)
			continue
		}

		folders := make([]string, 0, len(folderSet))
		for folder := range folderSet {
			folders = append(folders, folder)
		}

		slices.Sort(folders)

		patternID := createPatternID(folders)

		if existing, ok := patternMap[patternID]; ok {
			existing.DuplicateCount++
			existing.TotalFiles += len(group.Files)
		} else {
			patternMap[patternID] = &dto.FolderPattern{
				ID:             patternID,
				Folders:        folders,
				DuplicateCount: 1,
				TotalFiles:     len(group.Files),
			}
		}
	}

	patterns := make([]dto.FolderPattern, 0, len(patternMap))
	for _, p := range patternMap {
		patterns = append(patterns, *p)
	}

	sortPatternsByCount(patterns)

	c.JSON(http.StatusOK, dto.FolderPatternsResponse{
		Patterns:                   patterns,
		SingleFolderDuplicateCount: singleFolderDuplicateCount,
	})
}

// handleBatchDelete applies batch deletion rules to all matching duplicates
func (s *Server) handleBatchDelete(c *gin.Context) {
	var req dto.BatchDeleteRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.Rules) == 0 {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	ruleMap := make(map[string]string)
	for _, rule := range req.Rules {
		ruleMap[rule.PatternID] = rule.KeepFolder
	}

	groups, _, _, err := imaging.FindDuplicatesPaginated(s.db, 0, 100000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	var rulesApplied, filesDeleted, failedCount int
	var failedFiles []string

	for _, group := range groups {
		folderSet := make(map[string]bool)
		for _, file := range group.Files {
			dir := filepath.Dir(file.Path)
			folderSet[dir] = true
		}

		folders := make([]string, 0, len(folderSet))
		for folder := range folderSet {
			folders = append(folders, folder)
		}
		slices.Sort(folders)

		patternID := createPatternID(folders)

		keepFolder, hasRule := ruleMap[patternID]
		if !hasRule {
			continue
		}

		rulesApplied++

		for _, file := range group.Files {
			fileDir := filepath.Dir(file.Path)
			if fileDir == keepFolder {
				continue
			}

			if err := s.fileMover.MoveToTrashOrDelete(file.Path, req.TrashDir); err != nil {
				failedCount++
				failedFiles = append(failedFiles, filepath.Base(file.Path)+": "+err.Error())
				continue
			}

			s.fileMover.DeleteFromDB(file.Path)
			filesDeleted++
		}
	}

	c.JSON(http.StatusOK, dto.BatchDeleteResponse{
		RulesApplied: rulesApplied,
		FilesDeleted: filesDeleted,
		Failed:       failedCount,
		FailedFiles:  failedFiles,
	})
}

// --- Gallery Folder Handlers ---

// handleGetFolders returns all gallery folders
func (s *Server) handleGetFolders(c *gin.Context) {
	var folders []domain.GalleryFolder
	s.db.Order("created_at").Find(&folders)

	folderDTOs := make([]dto.GalleryFolderDTO, len(folders))
	for i, f := range folders {
		var count int64
		prefix := f.Path + "/"
		s.db.Model(&domain.ImageFile{}).Where("path LIKE ?", prefix+"%").Count(&count)

		folderDTOs[i] = dto.GalleryFolderDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileCount: int(count),
			CreatedAt: f.CreatedAt.Format(helpers.DateTimeFormat),
		}
	}

	c.JSON(http.StatusOK, dto.GalleryFoldersResponse{
		Folders:      folderDTOs,
		TotalFolders: len(folderDTOs),
	})
}

// handleAddFolder adds a new gallery folder and triggers a scan
func (s *Server) handleAddFolder(c *gin.Context) {
	var req dto.AddFolderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Validate directory exists
	normalizedPath, err := helpers.ValidateDirectory(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgFolderInvalidPath))
		return
	}

	// Check conflict with trash directory
	settings := s.settingsLoader.AppSettings()
	if settings.TrashDir != "" {
		if helpers.CheckPathsConflict(normalizedPath, settings.TrashDir) {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgFolderConflictTrash))
			return
		}
	}

	folder := domain.GalleryFolder{Path: normalizedPath}
	if result := s.db.Create(&folder); result.Error != nil {
		if strings.Contains(result.Error.Error(), "duplicate") || strings.Contains(result.Error.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgFolderAlreadyInGallery))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgFolderAddFailed))
		return
	}

	// Trigger background scan for this folder
	scanStarted := true
	if err := s.scanManager.ScanSingleDir(normalizedPath); err != nil {
		scanStarted = false
	}

	c.JSON(http.StatusOK, dto.AddFolderResponse{
		Message: string(i18n.MsgFolderAdded),
		Folder: dto.GalleryFolderDTO{
			ID:        folder.ID,
			Path:      folder.Path,
			FileCount: 0,
			CreatedAt: folder.CreatedAt.Format(helpers.DateTimeFormat),
		},
		ScanStarted: scanStarted,
	})
}

// handleRemoveFolder removes a gallery folder and its files from the database
func (s *Server) handleRemoveFolder(c *gin.Context) {
	id := c.Param("id")

	var folder domain.GalleryFolder
	if result := s.db.First(&folder, id); result.Error != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgFolderNotFound))
		return
	}

	// Delete all image files under this folder
	prefix := folder.Path + "/"
	result := s.db.Where("path LIKE ?", prefix+"%").Delete(&domain.ImageFile{})
	filesRemoved := int(result.RowsAffected)

	// Delete the folder record
	s.db.Delete(&folder)

	c.JSON(http.StatusOK, dto.RemoveFolderResponse{
		Message:      string(i18n.MsgFolderRemoved),
		FilesRemoved: filesRemoved,
	})
}

// handleGetGalleryImages returns paginated gallery images
func (s *Server) handleGetGalleryImages(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFlexible)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset
	view := c.DefaultQuery("view", "list")
	sortOrder := c.DefaultQuery("sortOrder", "newest")
	searchQuery := c.DefaultQuery("search", "")

	// Build base query with optional search filter
	query := s.db.Model(&domain.ImageFile{})
	if searchQuery != "" {
		// Search in filename or directory path (case-insensitive)
		// PostgreSQL: ILIKE for case-insensitive matching
		pattern := "%" + searchQuery + "%"
		query = query.Where("path ILIKE ?", pattern)
	}

	var totalImages int64
	query.Count(&totalImages)

	pag := helpers.CalcPagination(page, pageSize, totalImages)

	var files []domain.ImageFile
	orderClause := "mod_time DESC"
	if sortOrder == "oldest" {
		orderClause = "mod_time ASC"
	}
	query.Order(orderClause).Offset(offset).Limit(pageSize).Find(&files)

	imageDTOs := make([]dto.GalleryImageDTO, len(files))
	for i, f := range files {
		imageDTOs[i] = dto.GalleryImageDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileName:  filepath.Base(f.Path),
			DirPath:   filepath.Dir(f.Path),
			Size:      f.Size,
			SizeHuman: formatSize(f.Size),
			ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
		}
	}

	// Generate thumbnails in parallel if thumbnail or folders view
	if (view == "thumbnails" || view == "folders") && len(files) > 0 {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
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
	// Use the normalized path for gallery access check and serving
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

// --- App Settings Handlers ---

// handleGetSettings returns the current application settings
func (s *Server) handleGetSettings(c *gin.Context) {
	settings := s.settingsLoader.AppSettings()
	c.JSON(http.StatusOK, dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ExifBackupDir:         settings.ExifBackupDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		SyncDays:              settings.SyncDays,
		DailySyncHour:         settings.DailySyncHour,
		DailySyncMinute:       settings.DailySyncMinute,
		SyncTimezoneOffset:    settings.SyncTimezoneOffset,
		LastSyncAt:            settings.LastSyncAt,
		LastSyncNew:           settings.LastSyncNew,
		LastSyncUpdated:       settings.LastSyncUpdated,
		LastSyncDeleted:       settings.LastSyncDeleted,
		LastSyncThumbnails:    settings.LastSyncThumbnails,
	})
}

// handleUpdateSettings updates the application settings
func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req dto.UpdateSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	settings := s.settingsLoader.AppSettings()

	if req.TrashDir != nil {
		newTrashDir := strings.TrimSpace(*req.TrashDir)
		if newTrashDir != "" {
			// Normalize the trash dir path
			absTrash, err := filepath.Abs(newTrashDir)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTrashPath))
				return
			}
			normalizedTrash := filepath.ToSlash(absTrash)

			// Check conflict with all gallery folders
			var galleryFolders []domain.GalleryFolder
			s.db.Find(&galleryFolders)
			for _, gf := range galleryFolders {
				if helpers.CheckPathsConflict(normalizedTrash, gf.Path) {
					c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageTrashConflict))
					return
				}
			}
			settings.TrashDir = normalizedTrash
		} else {
			settings.TrashDir = ""
		}
	}
	if req.ExifBackupDir != nil {
		newBackupDir := strings.TrimSpace(*req.ExifBackupDir)
		if newBackupDir != "" {
			// Normalize the backup dir path
			absBackup, err := filepath.Abs(newBackupDir)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidBackupPath))
				return
			}
			normalizedBackup := filepath.ToSlash(absBackup)

			// Check conflict with all gallery folders
			var galleryFolders []domain.GalleryFolder
			s.db.Find(&galleryFolders)
			for _, gf := range galleryFolders {
				if helpers.CheckPathsConflict(normalizedBackup, gf.Path) {
					c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageBackupConflict))
					return
				}
			}
			settings.ExifBackupDir = normalizedBackup
		} else {
			settings.ExifBackupDir = ""
		}
	}
	if req.ThumbnailCachePath != nil {
		newCachePath := strings.TrimSpace(*req.ThumbnailCachePath)
		if newCachePath != "" {
			// Normalize the cache path
			absCache, err := filepath.Abs(newCachePath)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTrashPath))
				return
			}
			normalizedCache := filepath.ToSlash(absCache)
			settings.ThumbnailCachePath = normalizedCache

			// Update thumbnail service if available
			if s.thumbnailService != nil {
				log.Printf("Updating thumbnail cache path from %s to %s", s.thumbnailService.Stats().CacheDir, normalizedCache)
				if err := s.thumbnailService.UpdateCachePath(normalizedCache); err != nil {
					log.Printf("Failed to update thumbnail cache path: %v", err)
				} else {
					log.Printf("Thumbnail cache path updated successfully. New stats: %+v", s.thumbnailService.Stats())
				}
			} else {
				log.Printf("Thumbnail service is nil, cannot update cache path")
			}
		} else {
			settings.ThumbnailCachePath = ""
		}
	}

	if req.OcrConcurrentRequests != nil {
		val := *req.OcrConcurrentRequests
		if val < 0 {
			val = 0
		}
		settings.OcrConcurrentRequests = val

		// Update OcrManager in real-time if it exists
		if s.ocrManager != nil {
			s.ocrManager.SetMaxWorkers(val)
		}
	}

	scheduleChanged := false
	if req.SyncDays != nil {
		settings.SyncDays = *req.SyncDays
		scheduleChanged = true
	}
	if req.DailySyncHour != nil {
		hour := *req.DailySyncHour
		if hour < 0 || hour > 23 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		settings.DailySyncHour = hour
		scheduleChanged = true
	}
	if req.DailySyncMinute != nil {
		minute := *req.DailySyncMinute
		if minute < 0 || minute > 59 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		settings.DailySyncMinute = minute
		scheduleChanged = true
	}
	if req.SyncTimezoneOffset != nil {
		settings.SyncTimezoneOffset = *req.SyncTimezoneOffset
		scheduleChanged = true
	}

	// Apply schedule changes to background sync manager in real-time
	if s.backgroundSync != nil && scheduleChanged {
		syncDays := imaging.ParseSyncDays(settings.SyncDays)
		s.backgroundSync.UpdateSchedule(syncDays, settings.DailySyncHour, settings.DailySyncMinute, settings.SyncTimezoneOffset)
	}

	s.db.Save(&settings)

	c.JSON(http.StatusOK, dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ExifBackupDir:         settings.ExifBackupDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		SyncDays:              settings.SyncDays,
		DailySyncHour:         settings.DailySyncHour,
		DailySyncMinute:       settings.DailySyncMinute,
		SyncTimezoneOffset:    settings.SyncTimezoneOffset,
		LastSyncAt:            settings.LastSyncAt,
		LastSyncNew:           settings.LastSyncNew,
		LastSyncUpdated:       settings.LastSyncUpdated,
		LastSyncDeleted:       settings.LastSyncDeleted,
		LastSyncThumbnails:    settings.LastSyncThumbnails,
	})
}

// handleGetSyncStatus returns the current background sync status
func (s *Server) handleGetSyncStatus(c *gin.Context) {
	if s.backgroundSync == nil {
		c.JSON(http.StatusOK, dto.SyncStatusResponse{Running: false})
		return
	}
	status := s.backgroundSync.GetStatus()
	settings := s.settingsLoader.AppSettings()
	c.JSON(http.StatusOK, dto.SyncStatusResponse{
		Running:            status.Running,
		SyncInProgress:     status.SyncInProgress,
		NextRunAt:          formatTimeInUserTZ(status.NextRunAt, settings.SyncTimezoneOffset),
		LastSyncAt:         formatTimeInUserTZ(status.LastSyncAt, settings.SyncTimezoneOffset),
		LastSyncNew:        status.LastSyncNew,
		LastSyncUpdated:    status.LastSyncUpdated,
		LastSyncDeleted:    status.LastSyncDeleted,
		LastSyncThumbnails: status.LastSyncThumbnails,
		ProcessedFiles:     status.ProcessedFiles,
		TotalFiles:         status.TotalFiles,
	})
}

// formatTimeInUserTZ formats a time pointer as an ISO 8601 string in the user's configured timezone.
// This prevents browser double-conversion: the string is already in the user's local time.
func formatTimeInUserTZ(t *time.Time, timezoneOffset int) string {
	if t == nil {
		return ""
	}
	userTZ := time.FixedZone("UserTZ", -timezoneOffset*60)
	return t.In(userTZ).Format("2006-01-02T15:04:05")
}

// --- User Settings Handlers ---

// handleGetUserSettings returns the current user's settings
func (s *Server) handleGetUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var settings domain.UserSettings
	if result := s.db.FirstOrCreate(&settings, domain.UserSettings{UserID: user.ID}); result.Error != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}

// handleUpdateUserSettings updates the current user's settings
func (s *Server) handleUpdateUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var req dto.UpdateUserSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	validThemes := map[string]bool{
		"light-purple":  true,
		"dark-purple":   true,
		"light-green":   true,
		"dark-green":    true,
		"light-blue":    true,
		"dark-blue":     true,
		"light-orange":  true,
		"dark-orange":   true,
		"dark-contrast": true,
	}
	validLanguages := map[string]bool{"en": true, "ru": true}

	if req.Theme != "" && !validThemes[req.Theme] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTheme))
		return
	}
	if req.Language != "" && !validLanguages[req.Language] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidLanguage))
		return
	}

	var settings domain.UserSettings
	result := s.db.FirstOrCreate(&settings, domain.UserSettings{UserID: user.ID})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	if req.Theme != "" {
		settings.Theme = req.Theme
	}
	if req.Language != "" {
		settings.Language = req.Language
	}

	s.db.Save(&settings)

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}

// handleGetTrashInfo returns information about files in the trash directory
func (s *Server) handleGetTrashInfo(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	var fileCount int
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileCount++
		if fi, err := entry.Info(); err == nil {
			totalSize += fi.Size()
		}
	}

	c.JSON(http.StatusOK, dto.TrashInfoResponse{
		FileCount:      fileCount,
		TotalSize:      totalSize,
		TotalSizeHuman: formatSize(totalSize),
	})
}

// handleCleanTrash removes all files from the trash directory
func (s *Server) handleCleanTrash(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotExists))
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashReadFailed))
		return
	}

	var deleted, failed int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(settings.TrashDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			failed++
		} else {
			deleted++
		}
	}

	c.JSON(http.StatusOK, dto.CleanTrashResponse{
		Deleted: deleted,
		Failed:  failed,
	})
}

// TrashFileInfo represents a single file in trash
type TrashFileInfo struct {
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"sizeHuman"`
	ModTime   string `json:"modTime"`
}

// handleListTrashFiles returns a list of all files in the trash directory
func (s *Server) handleListTrashFiles(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	files := make([]TrashFileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if fi, err := entry.Info(); err == nil {
			files = append(files, TrashFileInfo{
				FileName:  entry.Name(),
				Size:      fi.Size(),
				SizeHuman: formatSize(fi.Size()),
				ModTime:   fi.ModTime().Format(helpers.RFC3339Format),
			})
		}
	}

	c.JSON(http.StatusOK, files)
}

// handleRestoreTrashFile moves a file from trash back to the original location
// If targetPath is not provided, restores to current working directory
func (s *Server) handleRestoreTrashFile(c *gin.Context) {
	var req struct {
		FileName   string `json:"fileName"`
		TargetPath string `json:"targetPath"` // Where to restore the file
	}
	if !helpers.BindJSON(c, &req) || req.FileName == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashFileNameRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	restoredPath, err := helpers.RestoreFile(settings.TrashDir, req.FileName, req.TargetPath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashRestored, "restoredPath": restoredPath})
}

// handleDeleteTrashFile permanently deletes a single file from trash
func (s *Server) handleDeleteTrashFile(c *gin.Context) {
	var req struct {
		FileName string `json:"fileName"`
	}
	if !helpers.BindJSON(c, &req) || req.FileName == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashFileNameRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	filePath := filepath.Join(settings.TrashDir, req.FileName)
	if _, err := os.Stat(filePath); err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashDeleteFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashFileDeleted})
}

// handleGetImageMetadata returns EXIF metadata for a single image
func (s *Server) handleGetImageMetadata(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Find the image file in DB
	var imageFile domain.ImageFile
	if result := s.db.Where("path = ?", path).First(&imageFile); result.Error != nil {
		c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: false})
		return
	}

	// Find metadata for this image
	var meta domain.ImageMetadata
	if result := s.db.Where("image_file_id = ?", imageFile.ID).First(&meta); result.Error != nil {
		c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: false})
		return
	}

	// Resolve geolocation from cache if referenced
	var geoLat, geoLng *float64
	var nameLocal, nameEng string
	if meta.GeolocationRef != nil {
		var geoCache domain.GeolocationCache
		if result := s.db.First(&geoCache, *meta.GeolocationRef); result.Error == nil {
			lat := geoCache.GPSLatitude
			lng := geoCache.GPSLongitude
			geoLat = &lat
			geoLng = &lng
			nameLocal = geoCache.NameLocal
			nameEng = geoCache.NameEng
		}
	}

	// Build the DTO
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
		HasExif:      imaging.HasExifData(&meta),
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

	// Query: join image_files with image_metadata where date_taken OR geolocation is missing
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

	// Build DTOs
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
			SizeHuman:   formatSize(r.Size),
			ModTime:     r.ModTime.Format(helpers.DateTimeFormat),
			MissingDate: missingDate,
			MissingGps:  missingGps,
		}
	}

	// Generate thumbnails in parallel
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

// handleGetGalleryCalendar returns paginated gallery images grouped by date taken
// Supports both cursor-based pagination (new) and offset-based pagination (legacy)
func (s *Server) handleGetGalleryCalendar(c *gin.Context) {
	startDate := c.Query("startDate") // "YYYY-MM-DD" or empty
	endDate := c.Query("endDate")     // "YYYY-MM-DD" or empty
	monthYear := c.Query("monthYear") // "YYYY-MM" for calendar widget
	sortOrder := c.DefaultQuery("sortOrder", "oldest")

	// Query: join image_files with image_metadata where date_taken is not null
	type imageWithDate struct {
		domain.ImageFile
		DateTaken      time.Time
		GeolocationRef *uint
	}

	baseQuery := s.db.Table("image_files").
		Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken IS NOT NULL")

	// Apply date range filter
	if startDate != "" {
		if t, err := time.Parse(helpers.DateOnlyFormat, startDate); err == nil {
			baseQuery = baseQuery.Where("image_metadata.date_taken >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse(helpers.DateOnlyFormat, endDate); err == nil {
			// End of the end date
			endOfDay := t.Add(24*time.Hour - time.Second)
			baseQuery = baseQuery.Where("image_metadata.date_taken <= ?", endOfDay)
		}
	}

	// Count total
	var totalImages int64
	baseQuery.Count(&totalImages)

	// Determine pagination mode: cursor or page-based
	cursorParam := c.Query("cursor")
	var results []imageWithDate
	var nextCursor *string

	if cursorParam != "" {
		// Cursor-based pagination - recreate baseQuery from scratch to avoid mutation issues
		decodedDate, decodedID, err := helpers.DecodeCursor(cursorParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidCursor))
			return
		}

		// Parse the decoded date/timestamp (supports both "YYYY-MM-DD" and "YYYY-MM-DD HH:MM:SS")
		var cursorDate time.Time
		if len(decodedDate) > 10 {
			// New format with time: "YYYY-MM-DD HH:MM:SS"
			cursorDate, err = time.Parse(helpers.DateTimeFormat, decodedDate)
		} else {
			// Old format without time: "YYYY-MM-DD" (backward compatibility)
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

		// Build order clause
		orderClause := "image_metadata.date_taken ASC, image_metadata.image_file_id ASC"
		if sortOrder == "newest" {
			orderClause = "image_metadata.date_taken DESC, image_metadata.image_file_id DESC"
		}

		// Create a FRESH baseQuery for cursor pagination to avoid state mutation
		cursorQuery := s.db.Table("image_files").
			Select("image_files.*, image_metadata.date_taken, image_metadata.geolocation_ref").
			Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
			Where("image_metadata.date_taken IS NOT NULL")

		// Apply date range filters
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

		// Query with cursor: fetch limit+1 to detect if more exists
		query := cursorQuery.Order(orderClause).Limit(pageSize + 1)

		if sortOrder == "newest" {
			// For newest first: get items before the cursor
			query = query.Where(
				"(image_metadata.date_taken < ?) OR (image_metadata.date_taken = ? AND image_metadata.image_file_id < ?)",
				cursorDate, cursorDate, decodedID,
			)
		} else {
			// For oldest first: get items after the cursor
			query = query.Where(
				"(image_metadata.date_taken > ?) OR (image_metadata.date_taken = ? AND image_metadata.image_file_id > ?)",
				cursorDate, cursorDate, decodedID,
			)
		}

		query.Find(&results)

		// If we got more than pageSize, the last item is used for next cursor
		if len(results) > pageSize {
			overflowItem := results[pageSize]
			lastKept := results[pageSize-1]

			if overflowItem.DateTaken.Format(helpers.DateOnlyFormat) == lastKept.DateTaken.Format(helpers.DateOnlyFormat) {
				// Date group is split across pages - fetch all remaining images
				// for this date to keep the group intact
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
				// Cursor at the end of the completed date group
				lastResult := results[len(results)-1]
				cursorStr := helpers.EncodeCursor(lastResult.DateTaken.Format(helpers.DateTimeFormat), lastResult.ID)
				nextCursor = &cursorStr
			} else {
				// Clean date boundary - cursor points to last kept item (with full timestamp);
				// next page starts from the overflow item via strict > comparison
				cursorStr := helpers.EncodeCursor(lastKept.DateTaken.Format(helpers.DateTimeFormat), lastKept.ID)
				nextCursor = &cursorStr
				results = results[:pageSize]
			}
		}
	} else {
		// Legacy offset-based pagination (first page or when no cursor provided)
		params := helpers.ParsePagination(c, helpers.ModeFlexible)
		pageSize := params.PageSize
		offset := params.Offset

		orderClause := "image_metadata.date_taken ASC, image_metadata.image_file_id ASC"
		if sortOrder == "newest" {
			orderClause = "image_metadata.date_taken DESC, image_metadata.image_file_id DESC"
		}

		// Fetch pageSize + 1 to detect if there's more data and generate a cursor
		baseQuery.Order(orderClause).Offset(offset).Limit(pageSize + 1).Find(&results)

		// If we got more than pageSize, generate a next cursor from the last item
		if len(results) > pageSize {
			overflowItem := results[pageSize]
			lastKept := results[pageSize-1]

			if overflowItem.DateTaken.Format(helpers.DateOnlyFormat) == lastKept.DateTaken.Format(helpers.DateOnlyFormat) {
				// Date group is split across pages - fetch all remaining images
				// for this date to keep the group intact
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
				// Cursor at the end of the completed date group
				lastResult := results[len(results)-1]
				cursorStr := helpers.EncodeCursor(lastResult.DateTaken.Format(helpers.DateTimeFormat), lastResult.ID)
				nextCursor = &cursorStr
			} else {
				// Clean date boundary - cursor points to last kept item (with full timestamp);
				// next page starts from the overflow item via strict > comparison
				cursorStr := helpers.EncodeCursor(lastKept.DateTaken.Format(helpers.DateTimeFormat), lastKept.ID)
				nextCursor = &cursorStr
				results = results[:pageSize]
			}
		}
	}

	// Group by date
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

	// Build response groups
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
				SizeHuman:  formatSize(r.Size),
				ModTime:    r.ModTime.Format(helpers.DateTimeFormat),
				MissingGps: missingGps,
			}
		}

		// Generate thumbnails in parallel
		if len(g.images) > 0 {
			paths := make([]string, len(g.images))
			for i, r := range g.images {
				paths[i] = r.Path
			}
			s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
				imageDTOs[idx].Thumbnail = thumb
			})
		}

		// Human-readable label
		label := g.date.Format("Monday, January 2, 2006")

		groupDTOs = append(groupDTOs, dto.CalendarDateGroup{
			Date:       dateStr,
			Label:      label,
			ImageCount: len(g.images),
			Images:     imageDTOs,
		})
	}

	// Get date range
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

	// Get month info for calendar widget
	var months []dto.CalendarMonthInfo
	if monthYear != "" {
		if t, err := time.Parse(helpers.YearMonthFormat, monthYear); err == nil {
			year := t.Year()
			month := int(t.Month())
			nextMonth := t.AddDate(0, 1, 0)

			// Get distinct days that have images in this month (PostgreSQL)
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

	// Calculate hasMore for legacy mode or use cursor result
	hasMore := nextCursor != nil
	if cursorParam == "" {
		// Legacy mode: calculate from page
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

	// Get min/max dates using the same JOIN as calendar API for consistency
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

	// Get all dates with image counts, ordered by date ASC (oldest first)
	// Use the same JOIN as the calendar API to ensure consistent counts
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

	// Compute page number and cursor for each date.
	// The calendar API paginates by images (not dates), ordered ASC.
	// We track a running total of images to determine which page each date falls on.
	// We also generate a cursor pointing to the start of each date.
	dates := make([]dto.TimelineDateMarker, 0, len(dateCounts))
	imageIndex := 0
	for _, dc := range dateCounts {
		page := (imageIndex / pageSize) + 1
		// Generate a synthetic cursor pointing to the start of this date
		// Using a midnight timestamp as the cursor start point
		cursor := helpers.EncodeCursor(dc.Date.Format(helpers.DateTimeFormat), 1)

		dates = append(dates, dto.TimelineDateMarker{
			Date:       dc.Date.Format(helpers.DateOnlyFormat),
			ImageCount: int(dc.Count),
			Page:       page, // Deprecated, kept for backward compatibility
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
	monthYear := c.Query("monthYear") // "YYYY-MM"
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

	// Get day-level counts: how many images per day in this month (PostgreSQL)
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

	// Build days array (only days that have images)
	days := make([]int, 0, len(dayCounts))
	for _, dc := range dayCounts {
		days = append(days, dc.Day)
	}

	// Get total images in this month
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
// If the requested date has no images, returns the nearest date's cursor
func (s *Server) handleGetCalendarSeek(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarMonthYearRequired))
		return
	}

	// Parse the requested date
	requestedDate, err := time.Parse(helpers.DateOnlyFormat, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidMonthYear))
		return
	}

	// Try to find the first image on this exact date
	var firstResult struct {
		ID        uint
		DateTaken time.Time
	}

	// Query: find the minimum ID for images on this date
	err = s.db.Table("image_files").
		Select("image_files.id, image_metadata.date_taken").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("DATE(image_metadata.date_taken) = ?", requestedDate).
		Order("image_files.id ASC").
		First(&firstResult).Error

	if err == nil {
		// Found images on this exact date.
		// Position cursor BEFORE the first image so the strict > comparison includes it.
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

	// No images on this date - find nearest date with images
	// Try next date first, then previous
	var nearestDate time.Time
	var nearestID uint

	// Find next date with images
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
		// Try previous date
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
			// No images at all
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

	// Normalize latitude: clamp to [-90, 90] (Mercator projection can produce extreme values at poles)
	minLat = math.Max(-90, math.Min(90, minLat))
	maxLat = math.Max(-90, math.Min(90, maxLat))
	// Normalize longitude: wrap into [-180, 180] range (Leaflet allows dragging past date line)
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
	// Ensure proper ordering after normalization
	if minLng > maxLng {
		// If normalized bounds crossed, the view covers the whole world longitudinally
		minLng = -180
		maxLng = 180
	}
	// Ensure latitude ordering
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

	// Store clusters in memory for later retrieval by clusterId
	s.clusterStorage.StoreClusters(clusters)

	// Clear ImagePaths from response (frontend will use clusterId to fetch images)
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

	// Check if clusterId is provided
	if clusterID != "" {
		s.handleGetGeoImagesByCluster(c, clusterID, page, pageSize)
		return
	}

	// Fallback to bounds-based query (for backward compatibility)
	s.handleGetGeoImagesByBounds(c, page, pageSize)
}

// handleGetGeoImagesByCluster returns images for a specific cluster
func (s *Server) handleGetGeoImagesByCluster(c *gin.Context, clusterID string, page, pageSize int) {
	// Get image paths from cluster storage
	imagePaths, found := s.clusterStorage.GetClusterImagePaths(clusterID)
	if !found {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgGeoClusterNotFound))
		return
	}

	// Calculate pagination
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

	// Get paginated paths
	paginatedPaths := imagePaths[offset:end]

	// Fetch files from database
	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.*").
		Where("image_files.path IN ?", paginatedPaths).
		Order("image_files.path").
		Find(&files)

	// Create DTOs
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
				SizeHuman: formatSize(f.Size),
				ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
			}
		}
	}

	// Filter out any nil entries (paths not found in DB)
	validDTOs := make([]dto.GalleryImageDTO, 0, len(imageDTOs))
	for _, dto := range imageDTOs {
		if dto.Path != "" {
			validDTOs = append(validDTOs, dto)
		}
	}
	imageDTOs = validDTOs

	// Generate thumbnails in parallel
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

	imageDTOs := make([]dto.GalleryImageDTO, len(files))
	for i, f := range files {
		imageDTOs[i] = dto.GalleryImageDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileName:  filepath.Base(f.Path),
			DirPath:   filepath.Dir(f.Path),
			Size:      f.Size,
			SizeHuman: formatSize(f.Size),
			ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
		}
	}

	// Generate thumbnails in parallel
	if len(files) > 0 {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
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

	// Default to full scan (non-incremental)
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

	// Query documents classified as text documents
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

	// Build DTOs with thumbnails
	docs := make([]dto.OcrDocumentDTO, len(results))
	for i, r := range results {
		docs[i] = dto.OcrDocumentDTO{
			ID:                 r.ID,
			ImageFileID:        r.ImageFileID,
			Path:               r.Path,
			FileName:           filepath.Base(r.Path),
			DirPath:            filepath.Dir(r.Path),
			Size:               r.Size,
			SizeHuman:          formatSize(r.Size),
			ModTime:            r.ModTime.Format(helpers.DateTimeFormat),
			MeanConfidence:     r.MeanConfidence,
			WeightedConfidence: r.WeightedConfidence,
			TokenCount:         r.TokenCount,
			Angle:              r.Angle,
			ScaleFactor:        r.ScaleFactor,
		}
	}

	// Generate thumbnails in parallel
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

	// Find classification
	var classification domain.OcrClassification
	if err := s.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("image_files.path = ?", imagePath).
		First(&classification).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	// Find bounding boxes
	var boxes []domain.OcrBoundingBox
	s.db.Where("classification_id = ?", classification.ID).Find(&boxes)

	// Convert to DTOs
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

// handleGetLlmSettings returns LLM settings with all providers
func (s *Server) handleGetLlmSettings(c *gin.Context) {
	settings := s.settingsLoader.LlmSettings()
	providers := s.settingsLoader.AllLlmProviders()

	// Load all cached model lists in a single query
	var cacheRows []domain.LlmProviderModelCache
	s.db.Find(&cacheRows)
	cacheMap := make(map[string][]dto.LlmModelDTO, len(cacheRows))
	for _, row := range cacheRows {
		var models []dto.LlmModelDTO
		if err := json.Unmarshal([]byte(row.ModelsJSON), &models); err == nil {
			cacheMap[row.ProviderAlias] = models
		}
	}

	// Build provider DTOs with masked API keys
	providerDTOs := make([]dto.LlmProviderDTO, len(providers))
	for i, p := range providers {
		apiKey := p.ApiKey
		if len(apiKey) > 8 {
			apiKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
		}
		providerDTOs[i] = dto.LlmProviderDTO{
			ID:           p.ID,
			Alias:        p.Alias,
			Name:         p.Name,
			ApiUrl:       p.ApiUrl,
			ApiKey:       apiKey,
			Model:        p.Model,
			CachedModels: cacheMap[p.Alias],
		}
	}

	c.JSON(http.StatusOK, dto.LlmSettingsResponse{
		ID:                     settings.ID,
		ActiveProvider:         settings.ActiveProvider,
		TagScanEnabled:         settings.TagScanEnabled,
		TagScanStartHour:       settings.TagScanStartHour,
		TagScanStartMinute:     settings.TagScanStartMinute,
		TagScanEndHour:         settings.TagScanEndHour,
		TagScanEndMinute:       settings.TagScanEndMinute,
		TagScanTimezoneOffset:  settings.TagScanTimezoneOffset,
		EmbeddingProviderAlias: settings.EmbeddingProviderAlias,
		EmbeddingModel:         settings.EmbeddingModel,
		EmbeddingDimension:     settings.EmbeddingDimension,
		EmbeddingBatchSize:     settings.EmbeddingBatchSize,
		Providers:              providerDTOs,
	})
}

// handleUpdateLlmSettings updates only LLM global settings (active provider + tag scan schedule)
func (s *Server) handleUpdateLlmSettings(c *gin.Context) {
	var req dto.UpdateLlmSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	var settings domain.LlmSettings
	err := s.db.First(&settings).Error
	globalUpdates := make(map[string]interface{})
	if req.ActiveProvider != nil {
		globalUpdates["active_provider"] = *req.ActiveProvider
	}
	if req.TagScanEnabled != nil {
		globalUpdates["tag_scan_enabled"] = *req.TagScanEnabled
	}
	if req.TagScanStartHour != nil {
		globalUpdates["tag_scan_start_hour"] = *req.TagScanStartHour
	}
	if req.TagScanStartMinute != nil {
		globalUpdates["tag_scan_start_minute"] = *req.TagScanStartMinute
	}
	if req.TagScanEndHour != nil {
		globalUpdates["tag_scan_end_hour"] = *req.TagScanEndHour
	}
	if req.TagScanEndMinute != nil {
		globalUpdates["tag_scan_end_minute"] = *req.TagScanEndMinute
	}
	if req.TagScanTimezoneOffset != nil {
		globalUpdates["tag_scan_timezone_offset"] = *req.TagScanTimezoneOffset
	}
	if req.EmbeddingProviderAlias != nil {
		globalUpdates["embedding_provider_alias"] = *req.EmbeddingProviderAlias
	}
	if req.EmbeddingModel != nil {
		globalUpdates["embedding_model"] = *req.EmbeddingModel
	}
	if req.EmbeddingDimension != nil {
		globalUpdates["embedding_dimension"] = *req.EmbeddingDimension
	}
	if req.EmbeddingBatchSize != nil {
		globalUpdates["embedding_batch_size"] = *req.EmbeddingBatchSize
	}

	if err == gorm.ErrRecordNotFound {
		settings = domain.LlmSettings{
			ActiveProvider:     "ollama_1",
			TagScanEnabled:     true,
			TagScanStartHour:   22,
			TagScanStartMinute: 0,
			TagScanEndHour:     7,
			TagScanEndMinute:   0,
		}
		if req.ActiveProvider != nil {
			settings.ActiveProvider = *req.ActiveProvider
		}
		if req.TagScanEnabled != nil {
			settings.TagScanEnabled = *req.TagScanEnabled
		}
		if req.TagScanStartHour != nil {
			settings.TagScanStartHour = *req.TagScanStartHour
		}
		if req.TagScanStartMinute != nil {
			settings.TagScanStartMinute = *req.TagScanStartMinute
		}
		if req.TagScanEndHour != nil {
			settings.TagScanEndHour = *req.TagScanEndHour
		}
		if req.TagScanEndMinute != nil {
			settings.TagScanEndMinute = *req.TagScanEndMinute
		}
		if req.TagScanTimezoneOffset != nil {
			settings.TagScanTimezoneOffset = *req.TagScanTimezoneOffset
		}
		if req.EmbeddingProviderAlias != nil {
			settings.EmbeddingProviderAlias = *req.EmbeddingProviderAlias
		}
		if req.EmbeddingModel != nil {
			settings.EmbeddingModel = *req.EmbeddingModel
		}
		if req.EmbeddingDimension != nil {
			settings.EmbeddingDimension = *req.EmbeddingDimension
		}
		if req.EmbeddingBatchSize != nil {
			settings.EmbeddingBatchSize = *req.EmbeddingBatchSize
		}
		if err := s.db.Create(&settings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	} else {
		if len(globalUpdates) > 0 {
			s.db.Model(&settings).Updates(globalUpdates)
		}
	}

	// Reload settings to get updated values
	s.db.First(&settings)

	// Update tag scan manager if running
	if s.tagScanManager != nil && s.tagScanManager.IsRunning() {
		s.tagScanManager.UpdateSchedule(settings.TagScanEnabled, settings.TagScanStartHour, settings.TagScanStartMinute, settings.TagScanEndHour, settings.TagScanEndMinute, settings.TagScanTimezoneOffset)
	}

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleProbeEmbeddingDimension probes the embedding model to detect its vector dimension.
// Sends a single "dimension probe" text to the model and returns len(embeddings[0]).
func (s *Server) handleProbeEmbeddingDimension(c *gin.Context) {
	var req dto.ProbeEmbeddingDimensionRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Resolve provider by alias
	var provider domain.LlmProvider
	if err := s.db.Where("alias = ?", req.ProviderAlias).First(&provider).Error; err != nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgEmbeddingProviderNotFound)
		return
	}

	// Create embedding client
	embeddingClient, err := llm.NewEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, req.Model)
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgEmbeddingClientFailed)
		return
	}

	// Probe: send a single short text and measure output dimension
	probe, err := embeddingClient.Embed([]string{"dimension probe"})
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgEmbeddingProbeFailed)
		return
	}
	if len(probe) == 0 || len(probe[0]) == 0 {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgEmbeddingEmptyVector)
		return
	}

	dimension := len(probe[0])

	// Persist detected dimension into LlmSettings so downstream code uses it immediately
	var settings domain.LlmSettings
	if s.db.First(&settings).Error == nil {
		s.db.Model(&settings).Updates(map[string]interface{}{
			"embedding_dimension":      dimension,
			"embedding_model":          req.Model,
			"embedding_provider_alias": req.ProviderAlias,
		})
	}

	c.JSON(http.StatusOK, dto.ProbeEmbeddingDimensionResponse{Dimension: dimension})
}

// handleCreateLlmProvider creates a new LLM provider
func (s *Server) handleCreateLlmProvider(c *gin.Context) {
	var req dto.CreateLlmProviderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Check alias uniqueness
	var existing domain.LlmProvider
	if err := s.db.Where("alias = ?", req.Alias).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	provider := domain.LlmProvider{
		Name:   req.Name,
		Alias:  req.Alias,
		ApiUrl: req.ApiUrl,
		ApiKey: req.ApiKey,
		Model:  req.Model,
	}
	if provider.ApiUrl == "" {
		switch provider.Name {
		case "ollama":
			provider.ApiUrl = "http://localhost:11434"
		case "ollama_cloud":
			provider.ApiUrl = "https://ollama.com"
		case "openai":
			provider.ApiUrl = "https://api.openai.com"
		}
	}
	if provider.Model == "" {
		provider.Model = "minicpm-v"
	}

	if err := s.db.Create(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	}

	c.JSON(http.StatusCreated, dto.LlmProviderDTO{
		ID:     provider.ID,
		Alias:  provider.Alias,
		Name:   provider.Name,
		ApiUrl: provider.ApiUrl,
		Model:  provider.Model,
	})
}

// handleUpdateLlmProvider updates an existing LLM provider by alias
func (s *Server) handleUpdateLlmProvider(c *gin.Context) {
	alias := c.Param("alias")
	var req dto.UpdateLlmProviderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	var provider domain.LlmProvider
	if err := s.db.Where("alias = ?", alias).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return
	}

	updates := make(map[string]interface{})
	if req.ApiUrl != nil {
		updates["api_url"] = *req.ApiUrl
	}
	if req.ApiKey != nil {
		updates["api_key"] = *req.ApiKey
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.Alias != nil && *req.Alias != alias {
		// Check new alias uniqueness
		var existing domain.LlmProvider
		if err := s.db.Where("alias = ?", *req.Alias).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, i18n.CreateValidationError(i18n.ValidationError))
			return
		}
		updates["alias"] = *req.Alias
		// Update LlmSettings.ActiveProvider if it referenced the old alias
		var settings domain.LlmSettings
		if s.db.First(&settings).Error == nil && settings.ActiveProvider == alias {
			s.db.Model(&settings).Update("active_provider", *req.Alias)
		}
	}

	if len(updates) > 0 {
		if err := s.db.Model(&provider).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}

		// Invalidate model cache when connection config changes (URL or key)
		if req.ApiUrl != nil || req.ApiKey != nil {
			s.db.Where("provider_alias = ?", alias).Delete(&domain.LlmProviderModelCache{})
		}

		// Migrate model cache row when alias is renamed
		if req.Alias != nil && *req.Alias != alias {
			s.db.Model(&domain.LlmProviderModelCache{}).
				Where("provider_alias = ?", alias).
				Update("provider_alias", *req.Alias)
		}
	}

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleDeleteLlmProvider deletes an LLM provider by alias
func (s *Server) handleDeleteLlmProvider(c *gin.Context) {
	alias := c.Param("alias")

	var provider domain.LlmProvider
	if err := s.db.Where("alias = ?", alias).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return
	}

	// If this was the active provider, reset active provider to first available
	var settings domain.LlmSettings
	if s.db.First(&settings).Error == nil && settings.ActiveProvider == alias {
		var firstProvider domain.LlmProvider
		if s.db.Where("id != ?", provider.ID).First(&firstProvider).Error == nil {
			s.db.Model(&settings).Update("active_provider", firstProvider.Alias)
		}
	}

	if err := s.db.Delete(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	}

	// Clean up model cache for deleted provider
	s.db.Where("provider_alias = ?", alias).Delete(&domain.LlmProviderModelCache{})

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

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

// handleLlmRecognize starts LLM-based OCR recognition asynchronously
func (s *Server) handleLlmRecognize(c *gin.Context) {
	var req dto.LlmOcrRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Create LLM client (also validates settings and enabled state)
	llmClient, provider, ok := s.llmFactory.CreateClient(c)
	if !ok {
		return
	}

	// Get image file ID from path
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", req.ImagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	// Check if already recognized (return cached result unless force re-recognition)
	if !req.Force && s.llmOcrService != nil {
		existing, _ := s.llmOcrService.GetRecognition(imageFile.ID)
		if existing != nil && existing.Success {
			c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
				Status:           "completed",
				MarkdownContent:  existing.MarkdownContent,
				Language:         existing.Language,
				Provider:         existing.Provider,
				Model:            existing.Model,
				ProcessingTimeMs: existing.ProcessingTimeMs,
			})
			return
		}
	}

	// Start async recognition
	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return
	}

	_ = s.llmOcrService.StartRecognizeAsync(imageFile.ID, llmClient, provider)
	c.JSON(http.StatusAccepted, dto.LlmRecognizeStatusResponse{
		Status: "processing",
	})
}

// handleLlmRecognizeStatus returns the status of an async LLM recognition task
func (s *Server) handleLlmRecognizeStatus(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	// Get image file ID
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", imagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
		return
	}

	taskStatus := s.llmOcrService.GetRecognizeStatus(imageFile.ID)
	if taskStatus == nil {
		// No active task — check if there's a result in DB
		existing, _ := s.llmOcrService.GetRecognition(imageFile.ID)
		if existing != nil && existing.Success {
			c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
				Status:           "completed",
				MarkdownContent:  existing.MarkdownContent,
				Language:         existing.Language,
				Provider:         existing.Provider,
				Model:            existing.Model,
				ProcessingTimeMs: existing.ProcessingTimeMs,
			})
			return
		}
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
		return
	}

	switch taskStatus.Status {
	case "processing":
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "processing"})
	case "completed":
		resp := dto.LlmRecognizeStatusResponse{
			Status: "completed",
		}
		if taskStatus.Result != nil {
			resp.MarkdownContent = taskStatus.Result.MarkdownContent
			resp.Language = taskStatus.Result.Language
			resp.Provider = taskStatus.Result.Provider
			resp.Model = taskStatus.Result.Model
			resp.ProcessingTimeMs = taskStatus.Result.ProcessingTimeMs
		}
		c.JSON(http.StatusOK, resp)
	case "failed":
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
			Status: "failed",
			Error:  taskStatus.Error,
		})
	default:
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
	}
}

// handleGetLlmRecognition retrieves LLM OCR recognition for an image
func (s *Server) handleGetLlmRecognition(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	// Get image file ID
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", imagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	// Get recognition
	if s.llmOcrService == nil {
		c.JSON(http.StatusOK, dto.LlmOcrDataResponse{Found: false})
		return
	}

	recognition, err := s.llmOcrService.GetRecognition(imageFile.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return
	}

	if recognition == nil {
		c.JSON(http.StatusOK, dto.LlmOcrDataResponse{Found: false})
		return
	}

	c.JSON(http.StatusOK, dto.LlmOcrDataResponse{
		Found:            true,
		MarkdownContent:  recognition.MarkdownContent,
		Language:         recognition.Language,
		Provider:         recognition.Provider,
		Model:            recognition.Model,
		ProcessingTimeMs: recognition.ProcessingTimeMs,
		Success:          recognition.Success,
		Error:            recognition.Error,
		CreatedAt:        recognition.CreatedAt.Format(helpers.DateTimeFormat),
	})
}

// handleGetLlmModels returns a list of available LLM models from the configured server.
// Supports DB-backed caching: returns cached models unless force=true or no cache exists.
func (s *Server) handleGetLlmModels(c *gin.Context) {
	// Check for optional provider query parameter
	providerName := c.Query("provider")
	forceRefresh := c.Query("force") == "true"

	var provider domain.LlmProvider
	var found bool

	if providerName != "" {
		// Load specific provider
		provider, found = s.settingsLoader.LlmProvider(providerName)
	} else {
		// Get active provider from settings
		settings, settingsFound := s.settingsLoader.LlmSettingsIfExists()
		if !settingsFound {
			c.JSON(http.StatusNotFound, dto.LlmModelsResponse{
				Success:  false,
				Error:    "LLM settings not configured",
				Provider: "",
			})
			return
		}
		provider, found = s.settingsLoader.LlmProvider(settings.ActiveProvider)
	}

	if !found {
		c.JSON(http.StatusNotFound, dto.LlmModelsResponse{
			Success:  false,
			Error:    "Provider not configured",
			Provider: providerName,
		})
		return
	}

	// Check DB cache unless force-refresh requested
	if !forceRefresh {
		var cacheRow domain.LlmProviderModelCache
		if err := s.db.Where("provider_alias = ?", provider.Alias).First(&cacheRow).Error; err == nil {
			var models []dto.LlmModelDTO
			if err := json.Unmarshal([]byte(cacheRow.ModelsJSON), &models); err == nil && len(models) > 0 {
				c.JSON(http.StatusOK, dto.LlmModelsResponse{
					Success:  true,
					Models:   models,
					Provider: provider.Name,
				})
				return
			}
		}
	}

	// Create LLM client
	llmClient, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, provider.Model, s.config.LlmMaxImageMegapixels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.LlmModelsResponse{
			Success:  false,
			Error:    err.Error(),
			Provider: provider.Name,
		})
		return
	}

	// Fetch models from provider
	models, err := llmClient.ListModels()
	if err != nil {
		c.JSON(http.StatusOK, dto.LlmModelsResponse{
			Success:  false,
			Error:    err.Error(),
			Provider: provider.Name,
		})
		return
	}

	// Convert to DTO
	modelDTOs := make([]dto.LlmModelDTO, len(models))
	for i, m := range models {
		modelDTOs[i] = dto.LlmModelDTO{
			ID:            m.ID,
			Name:          m.Name,
			Size:          m.Size,
			ContextLength: m.ContextLength,
		}
	}

	// Upsert cache row (only on successful fetch; stale cache is better than no cache on failure)
	if len(modelDTOs) > 0 {
		modelsJSON, _ := json.Marshal(modelDTOs)
		var existing domain.LlmProviderModelCache
		if s.db.Where("provider_alias = ?", provider.Alias).First(&existing).Error == nil {
			s.db.Model(&existing).Updates(map[string]interface{}{
				"models_json": string(modelsJSON),
				"fetched_at":  time.Now(),
			})
		} else {
			s.db.Create(&domain.LlmProviderModelCache{
				ProviderAlias: provider.Alias,
				ModelsJSON:    string(modelsJSON),
				FetchedAt:     time.Now(),
			})
		}
	}

	c.JSON(http.StatusOK, dto.LlmModelsResponse{
		Success:  true,
		Models:   modelDTOs,
		Provider: provider.Name,
	})
}

// handleGetImageTags returns existing tags for an image (without generating new ones).
func (s *Server) handleGetImageTags(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is required"})
		return
	}

	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", path).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	var existingTags []domain.ImageTag
	s.db.Where("image_file_id = ?", imageFile.ID).Find(&existingTags)

	tags := make([]string, len(existingTags))
	for i, t := range existingTags {
		tags[i] = t.Tag
	}

	c.JSON(http.StatusOK, dto.ImageTagsResponse{Tags: tags})
}

// handleAiAction executes an AI action (describe, tags, recognizeText, askQuestion) asynchronously
func (s *Server) handleAiAction(c *gin.Context) {
	var req dto.AiActionRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Create LLM client (also validates settings and enabled state)
	llmClient, provider, ok := s.llmFactory.CreateClient(c)
	if !ok {
		// CreateClient already wrote the error response
		return
	}

	// Get image file ID from path
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", req.ImagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "Image not found",
		})
		return
	}

	// Validate action
	if req.Action == dto.AiActionAskQuestion && req.Question == "" {
		c.JSON(http.StatusBadRequest, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "Question is required for askQuestion action",
		})
		return
	}

	// Start async AI action
	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "AI service not available",
		})
		return
	}

	// Generate unique task ID
	taskID := uuid.New().String()

	// For "tags" action, check if tags already exist in DB (unless force regeneration requested)
	if req.Action == dto.AiActionTags && !req.Force {
		var existingTags []domain.ImageTag
		s.db.Where("image_file_id = ?", imageFile.ID).Find(&existingTags)
		if len(existingTags) > 0 {
			tags := make([]string, len(existingTags))
			for i, t := range existingTags {
				tags[i] = t.Tag
			}
			s.llmOcrService.StoreCachedTagsResult(taskID, tags)

			c.JSON(http.StatusAccepted, dto.AiActionStartResponse{
				TaskID: taskID,
				Action: req.Action,
				Status: "processing",
			})
			return
		}
	}

	// Start async processing
	s.llmOcrService.StartAiActionAsync(taskID, imageFile.ID, string(req.Action), req.Question, req.Language, llmClient, provider)

	// Return 202 Accepted with task ID
	c.JSON(http.StatusAccepted, dto.AiActionStartResponse{
		TaskID: taskID,
		Action: req.Action,
		Status: "processing",
	})
}

// handleAiActionStatus returns the status of an async AI action task
func (s *Server) handleAiActionStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "Task ID is required",
		})
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "AI service not available",
		})
		return
	}

	taskStatus := s.llmOcrService.GetAiActionStatus(taskID)
	if taskStatus == nil {
		c.JSON(http.StatusNotFound, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "Task not found or expired",
		})
		return
	}

	response := dto.AiActionStatusResponse{
		TaskID: taskID,
		Status: taskStatus.Status,
		Action: dto.AiActionType(taskStatus.Action),
	}

	if taskStatus.Status == "completed" && taskStatus.Result != nil {
		response.Provider = taskStatus.Result.Provider
		response.Model = taskStatus.Result.Model
		response.ProcessingTimeMs = taskStatus.Result.ProcessingTimeMs

		if taskStatus.Action == "tags" {
			response.Tags = taskStatus.Result.Tags
		} else {
			response.Result = taskStatus.Result.Result
		}
	} else if taskStatus.Status == "failed" {
		response.Error = taskStatus.Error
	}

	c.JSON(http.StatusOK, response)
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

// --- Geocode / GPS Handlers ---

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

	// Validate coordinates
	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsInvalidCoordinates)
		return
	}

	// Validate JPEG extension
	ext := strings.ToLower(filepath.Ext(req.Path))
	if ext != ".jpg" && ext != ".jpeg" {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsNotJpeg)
		return
	}

	// Look up ImageFile by path
	var imageFile domain.ImageFile
	if result := s.db.Where("path = ?", req.Path).First(&imageFile); result.Error != nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgImageNotFound)
		return
	}

	// Load existing metadata for EXIF restore in case of nuclear fallback
	var existingMeta domain.ImageMetadata
	s.db.Where("image_file_id = ?", imageFile.ID).First(&existingMeta)

	// Convert to OS path
	osPath := filepath.FromSlash(req.Path)

	// Write GPS to EXIF via EXIF service
	if err := s.exifClient.WriteGPS(context.Background(), osPath, req.Lat, req.Lng, s.settingsLoader.AppSettings().ExifBackupDir, &existingMeta); err != nil {
		log.Printf("UpdateGps: WriteGPS failed for %s: %v", req.Path, err)
		if strings.Contains(err.Error(), "backup") {
			s.respondError(c, http.StatusInternalServerError, i18n.MsgGpsBackupFailed)
		} else {
			s.respondError(c, http.StatusInternalServerError, i18n.MsgGpsUpdateFailed)
		}
		return
	}

	// Enrich any missing EXIF fields from the file into the DB record
	enriched, _ := s.exifClient.EnrichMissingMetadata(context.Background(), osPath, &existingMeta)

	// Reverse geocode via GeolocationService (cache + Nominatim)
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

	// Upsert ImageMetadata in DB (reuse existingMeta already loaded above)
	if existingMeta.ID == 0 {
		// No existing record — create new with all enriched fields
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
		s.db.Create(&newMeta)
	} else {
		// Update existing record with geolocation + any enriched fields
		updates := map[string]interface{}{"geolocation_ref": geoRef}
		for k, v := range enriched {
			updates[k] = v
		}
		s.db.Model(&existingMeta).Updates(updates)
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
// Requires "date" (YYYY-MM-DD) query param. Queries image_metadata + geolocation_caches only.
func (s *Server) handleGetLocationCandidates(c *gin.Context) {
	dateParam := c.Query("date")
	if dateParam == "" {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGeocodeDateRequired)
		return
	}

	// Validate date format
	targetDate, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsInvalidCoordinates)
		return
	}
	nextDay := targetDate.AddDate(0, 0, 1)

	// Query same-day photos with geolocation data via geolocation_caches (no image_files join)
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

	// Group by unique (lat, lng) pairs
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
		// Round to ~5km precision for grouping
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

	// Build candidates (limit 20, no thumbnails since image_files is not joined)
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

	// Sort by photo count descending
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

	// Validate coordinates
	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgGpsInvalidCoordinates)
		return
	}

	// Validate paths
	if len(req.Paths) == 0 {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgBatchGpsNoPaths)
		return
	}

	// Reverse geocode once for all photos via GeolocationService
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
		// Validate JPEG extension
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".jpg" && ext != ".jpeg" {
			failedCount++
			failedFiles = append(failedFiles, p)
			continue
		}

		// Look up ImageFile by path
		var imageFile domain.ImageFile
		if result := s.db.Where("path = ?", p).First(&imageFile); result.Error != nil {
			failedCount++
			failedFiles = append(failedFiles, p)
			continue
		}

		// Load existing metadata for EXIF restore in case of nuclear fallback
		var existingMeta domain.ImageMetadata
		s.db.Where("image_file_id = ?", imageFile.ID).First(&existingMeta)

		// Check if image already has geolocation - skip if it does
		if existingMeta.GeolocationRef != nil {
			skippedCount++
			continue
		}

		// Convert to OS path
		osPath := filepath.FromSlash(p)

		// Write GPS to EXIF via EXIF service
		if err := s.exifClient.WriteGPS(context.Background(), osPath, req.Lat, req.Lng, s.settingsLoader.AppSettings().ExifBackupDir, &existingMeta); err != nil {
			log.Printf("BatchUpdateGps: WriteGPS failed for %s: %v", p, err)
			failedCount++
			failedFiles = append(failedFiles, p)
			continue
		}

		// Enrich any missing EXIF fields from the file into the DB record
		enriched, _ := s.exifClient.EnrichMissingMetadata(context.Background(), osPath, &existingMeta)

		// Upsert ImageMetadata in DB (reuse existingMeta already loaded above)
		if existingMeta.ID == 0 {
			// No existing record — create new with all enriched fields
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
			s.db.Create(&newMeta)
		} else {
			// Update existing record with geolocation + any enriched fields
			updates := map[string]interface{}{"geolocation_ref": geoRef}
			for k, v := range enriched {
				updates[k] = v
			}
			s.db.Model(&existingMeta).Updates(updates)
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
