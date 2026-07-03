package handler

import (
	"net/http"
	"path/filepath"
	"slices"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
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

	// Prepare group DTOs with async thumbnail generation.
	// Cached thumbnails are returned immediately ("generated"), uncached items
	// are marked "pending" and generated in background goroutines.
	groupDTOs := make([]dto.DuplicateGroupDTO, len(groups))
	pageFiles := 0

	for _, g := range groups {
		pageFiles += len(g.Files)
	}

	// Collect paths for thumbnail generation
	paths := make([]string, len(groups))
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
			Index:           offset + i + 1,
			Hash:            g.Hash,
			Size:            g.Size,
			SizeHuman:       helpers.FormatSize(g.Size),
			Files:           fileDTOs,
			ThumbnailStatus: "pending",
		}

		if len(g.Files) > 0 {
			paths[i] = g.Files[0].Path
		}
	}

	// Fast path: return cached thumbnails immediately without generation.
	hasPending := false
	for i, path := range paths {
		if path == "" {
			continue
		}
		if thumb, ok := s.thumbnailBatch.TryGetCached(path); ok {
			groupDTOs[i].Thumbnail = thumb
			groupDTOs[i].ThumbnailStatus = "generated"
		} else {
			hasPending = true
		}
	}

	// Launch async generation for pending items — does not block the response.
	if hasPending {
		s.thumbnailBatch.GenerateParallelAsync(paths, func(idx int, thumb string) {
			// Thumbnail is now cached; subsequent requests will find it via TryGetCached.
			// We don't update the in-flight response — the client polls for it.
		})
	}

	// Get scanned dirs from gallery folders
	galleryFolders, _ := s.galleryFolderRepo.FindAll()
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
