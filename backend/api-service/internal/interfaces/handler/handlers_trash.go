package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// trashDefaultPageSize is the page size used for the grouped trash list.
const trashDefaultPageSize = 50

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
		TotalSizeHuman: helpers.FormatSize(totalSize),
	})
}

// handleCleanTrash removes all files from the trash directory and clears trash_items rows
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

	// Clear the persisted trash metadata.
	if err := s.db.Where("1 = 1").Delete(&domain.TrashItem{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashCleanFailed))
		return
	}

	c.JSON(http.StatusOK, dto.CleanTrashResponse{
		Deleted: deleted,
		Failed:  failed,
	})
}

// handleGetTrash returns trash items grouped by deletion date, newest → oldest,
// cursor-paginated with boundary-date overflow handling.
func (s *Server) handleGetTrash(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, dto.TrashListResponse{
			Groups:      []dto.TrashDateGroup{},
			TotalItems:  0,
			TotalGroups: 0,
			HasMore:     false,
			NextCursor:  nil,
		})
		return
	}

	// Reconcile the trash directory with trash_items (backfill/prune).
	s.reconcileTrash(settings.TrashDir)

	pageSize := trashDefaultPageSize
	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 200 {
			pageSize = parsed
		}
	}

	var totalItems int64
	s.db.Model(&domain.TrashItem{}).Count(&totalItems)

	baseQuery := s.db.Model(&domain.TrashItem{})

	cursorParam := c.Query("cursor")
	var rows []domain.TrashItem
	var nextCursor *string

	if cursorParam != "" {
		decodedDate, decodedID, err := helpers.DecodeCursor(cursorParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashInvalidCursor))
			return
		}

		var cursorDate time.Time
		if len(decodedDate) > 10 {
			cursorDate, err = time.Parse(helpers.DateTimeFormat, decodedDate)
		} else {
			cursorDate, err = time.Parse(helpers.DateOnlyFormat, decodedDate)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashInvalidCursor))
			return
		}

		orderClause := "deleted_at DESC, id DESC"
		cursorQuery := baseQuery
		query := cursorQuery.Order(orderClause).Limit(pageSize+1).
			Where("(deleted_at < ?) OR (deleted_at = ? AND id < ?)", cursorDate, cursorDate, decodedID)
		query.Find(&rows)

		if len(rows) > pageSize {
			overflowItem := rows[pageSize]
			lastKept := rows[pageSize-1]

			if sameCalendarDay(overflowItem.DeletedAt, lastKept.DeletedAt) {
				var extra []domain.TrashItem
				s.db.Model(&domain.TrashItem{}).
					Where("DATE(deleted_at) = DATE(?) AND id < ?", lastKept.DeletedAt, lastKept.ID).
					Order(orderClause).Find(&extra)
				rows = append(rows[:pageSize], extra...)
				lastResult := rows[len(rows)-1]
				cursorStr := helpers.EncodeCursor(lastResult.DeletedAt.Format(helpers.DateTimeFormat), lastResult.ID)
				nextCursor = &cursorStr
			} else {
				cursorStr := helpers.EncodeCursor(lastKept.DeletedAt.Format(helpers.DateTimeFormat), lastKept.ID)
				nextCursor = &cursorStr
				rows = rows[:pageSize]
			}
		}
	} else {
		orderClause := "deleted_at DESC, id DESC"
		baseQuery.Order(orderClause).Limit(pageSize + 1).Find(&rows)

		if len(rows) > pageSize {
			overflowItem := rows[pageSize]
			lastKept := rows[pageSize-1]

			if sameCalendarDay(overflowItem.DeletedAt, lastKept.DeletedAt) {
				var extra []domain.TrashItem
				s.db.Model(&domain.TrashItem{}).
					Where("DATE(deleted_at) = DATE(?) AND id < ?", lastKept.DeletedAt, lastKept.ID).
					Order(orderClause).Find(&extra)
				rows = append(rows[:pageSize], extra...)
				lastResult := rows[len(rows)-1]
				cursorStr := helpers.EncodeCursor(lastResult.DeletedAt.Format(helpers.DateTimeFormat), lastResult.ID)
				nextCursor = &cursorStr
			} else {
				cursorStr := helpers.EncodeCursor(lastKept.DeletedAt.Format(helpers.DateTimeFormat), lastKept.ID)
				nextCursor = &cursorStr
				rows = rows[:pageSize]
			}
		}
	}

	// Group by deletion calendar day in the order encountered.
	type dateGroup struct {
		date  time.Time
		items []domain.TrashItem
	}
	groupsMap := make(map[string]*dateGroup)
	var dateOrder []string

	for _, r := range rows {
		dateStr := r.DeletedAt.Format(helpers.DateOnlyFormat)
		if _, ok := groupsMap[dateStr]; !ok {
			groupsMap[dateStr] = &dateGroup{date: r.DeletedAt}
			dateOrder = append(dateOrder, dateStr)
		}
		groupsMap[dateStr].items = append(groupsMap[dateStr].items, r)
	}

	groupDTOs := make([]dto.TrashDateGroup, 0, len(dateOrder))
	for _, dateStr := range dateOrder {
		g := groupsMap[dateStr]
		itemDTOs := make([]dto.TrashItemDTO, len(g.items))
		for i, r := range g.items {
			itemDTOs[i] = dto.TrashItemDTO{
				ID:           r.ID,
				FileName:     r.FileName,
				TrashPath:    r.TrashPath,
				OriginalPath: r.OriginalPath,
				Size:         r.Size,
				SizeHuman:    helpers.FormatSize(r.Size),
				DeletedAt:    r.DeletedAt.Format(helpers.DateTimeFormat),
			}
		}

		if len(g.items) > 0 {
			paths := make([]string, len(g.items))
			for i, r := range g.items {
				paths[i] = r.TrashPath
			}
			s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
				itemDTOs[idx].Thumbnail = thumb
			})
		}

		label := g.date.Format("Monday, January 2, 2006")

		groupDTOs = append(groupDTOs, dto.TrashDateGroup{
			Date:      dateStr,
			Label:     label,
			ItemCount: len(g.items),
			Items:     itemDTOs,
		})
	}

	hasMore := nextCursor != nil

	c.JSON(http.StatusOK, dto.TrashListResponse{
		Groups:      groupDTOs,
		TotalItems:  int(totalItems),
		TotalGroups: len(groupDTOs),
		HasMore:     hasMore,
		NextCursor:  nextCursor,
	})
}

// reconcileTrash backfills trash_items rows for files present on disk but missing
// a row (legacy entries: originalPath="", deletedAt=mod time) and prunes rows
// whose file no longer exists.
func (s *Server) reconcileTrash(trashDir string) {
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return
	}

	onDisk := make(map[string]os.FileInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if fi, err := entry.Info(); err == nil {
			onDisk[filepath.ToSlash(filepath.Join(trashDir, entry.Name()))] = fi
		}
	}

	var rows []domain.TrashItem
	s.db.Find(&rows)

	// Backfill files present on disk without a row.
	for trashPath, fi := range onDisk {
		var count int64
		s.db.Model(&domain.TrashItem{}).Where("trash_path = ?", trashPath).Count(&count)
		if count == 0 {
			s.db.Create(&domain.TrashItem{
				FileName:     filepath.Base(trashPath),
				TrashPath:    trashPath,
				OriginalPath: "",
				Size:         fi.Size(),
				DeletedAt:    fi.ModTime(),
			})
		}
	}

	// Prune rows whose file is gone.
	for _, r := range rows {
		if _, ok := onDisk[r.TrashPath]; !ok {
			s.db.Delete(&domain.TrashItem{}, r.ID)
		}
	}
}

// sameCalendarDay reports whether two times fall on the same calendar day.
func sameCalendarDay(a, b time.Time) bool {
	return a.Format(helpers.DateOnlyFormat) == b.Format(helpers.DateOnlyFormat)
}

// handleRestoreTrashFile restores a trashed file to its original path by id
func (s *Server) handleRestoreTrashFile(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if !helpers.BindJSON(c, &req) || req.ID == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashIdRequired))
		return
	}

	var item domain.TrashItem
	if err := s.db.First(&item, req.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashItemNotFound))
		return
	}

	if item.OriginalPath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashRestoreNoOriginalPath))
		return
	}

	restoredPath, err := helpers.RestoreFile(filepath.Dir(item.TrashPath), filepath.Base(item.TrashPath), item.OriginalPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashRestoreFailed))
		return
	}

	// Re-insert the image_files row so the restored file reappears in the gallery.
	hash := item.OriginalHash
	modTime := item.OriginalModTime
	if hash == "" || modTime.IsZero() {
		if computed, err := imaging.CalculateFileHash(restoredPath); err == nil && hash == "" {
			hash = computed
		}
		if fi, err := os.Stat(restoredPath); err == nil && modTime.IsZero() {
			modTime = fi.ModTime()
		}
	}
	s.db.Create(&domain.ImageFile{
		Path:    filepath.ToSlash(restoredPath),
		Size:    item.Size,
		Hash:    hash,
		ModTime: modTime,
	})

	// Remove the trash metadata row.
	s.db.Delete(&domain.TrashItem{}, item.ID)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashRestored, "restoredPath": restoredPath})
}

// handleDeleteTrashFile permanently deletes a trashed file by id
func (s *Server) handleDeleteTrashFile(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if !helpers.BindJSON(c, &req) || req.ID == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashIdRequired))
		return
	}

	var item domain.TrashItem
	if err := s.db.First(&item, req.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashItemNotFound))
		return
	}

	if _, err := os.Stat(item.TrashPath); err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	if err := os.Remove(item.TrashPath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashDeleteFailed))
		return
	}

	s.db.Delete(&domain.TrashItem{}, item.ID)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashFileDeleted})
}

// handleServeTrashImage serves a full-size image from the trash directory for the lightbox
func (s *Server) handleServeTrashImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
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

	// Security: verify the resolved path is inside the configured trash directory
	trashAbs, err := filepath.Abs(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashReadFailed))
		return
	}
	trashNorm := filepath.ToSlash(filepath.Clean(trashAbs))
	if !isInsidePath(normalizedPath, trashNorm) {
		c.JSON(http.StatusForbidden, i18n.ErrorResponse(i18n.MsgTrashAccessDenied))
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		return
	}

	c.File(absPath)
}

// isInsidePath reports whether path equals root or is nested inside root.
func isInsidePath(path, root string) bool {
	if path == root {
		return false
	}
	return strings.HasPrefix(path, root+"/")
}
