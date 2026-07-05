package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleGetFolders returns all gallery folders
func (s *Server) handleGetFolders(c *gin.Context) {
	folders, err := s.galleryFolderRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgFolderAddFailed))
		return
	}

	folderDTOs := make([]dto.GalleryFolderDTO, len(folders))
	for i, f := range folders {
		count, _ := s.imageFileRepo.CountByPathPrefix(f.Path)
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
	if err := s.galleryFolderRepo.Create(&folder); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgFolderAlreadyInGallery))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgFolderAddFailed))
		return
	}

	// Invalidate gallery access cache so new folder takes effect immediately
	s.galleryAccess.Invalidate()

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

// handleCreateSubfolder creates a new directory inside a gallery folder
func (s *Server) handleCreateSubfolder(c *gin.Context) {
	var req dto.CreateFolderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Validate the parent path is within gallery
	if !s.galleryAccess.IsPathInGallery(req.ParentPath) {
		c.JSON(http.StatusForbidden, i18n.ErrorResponse(i18n.MsgImageAccessDenied))
		return
	}

	// Sanitize folder name (no path separators)
	if strings.Contains(req.FolderName, "/") || strings.Contains(req.FolderName, "\\") {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgFolderNameRequired))
		return
	}

	newPath := filepath.Join(req.ParentPath, req.FolderName)

	// Check if already exists
	if _, err := os.Stat(newPath); err == nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgFolderAlreadyExists))
		return
	}

	// Create the directory
	if err := os.MkdirAll(newPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgFolderCreateFailed))
		return
	}

	c.JSON(http.StatusOK, dto.CreateFolderResponse{
		Message: string(i18n.MsgFolderCreated),
		Path:    filepath.ToSlash(newPath),
	})
}

// handleListSubdirs lists subdirectories of a given path within gallery
func (s *Server) handleListSubdirs(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgFolderParentPathRequired))
		return
	}

	// Validate the path is within gallery
	if !s.galleryAccess.IsPathInGallery(path) {
		c.JSON(http.StatusForbidden, i18n.ErrorResponse(i18n.MsgImageAccessDenied))
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgFolderSubdirsReadFailed))
		return
	}

	var subdirs []dto.SubdirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(path, entry.Name())
			subdirs = append(subdirs, dto.SubdirEntry{
				Name: entry.Name(),
				Path: filepath.ToSlash(subPath),
			})
		}
	}

	if subdirs == nil {
		subdirs = []dto.SubdirEntry{}
	}

	c.JSON(http.StatusOK, dto.SubdirsResponse{
		Subdirs: subdirs,
		Path:    path,
	})
}

// handleRemoveFolder removes a gallery folder and its files from the database
func (s *Server) handleRemoveFolder(c *gin.Context) {
	id := c.Param("id")

	var folderID uint
	if _, err := fmt.Sscanf(id, "%d", &folderID); err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgFolderNotFound))
		return
	}
	folder, err := s.galleryFolderRepo.FindByID(folderID)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgFolderNotFound))
		return
	}

	// Delete all image files under this folder
	prefix := folder.Path + "/"
	filesRemoved, _ := s.imageFileRepo.DeleteByPathPrefix(prefix)

	// Delete the folder record
	s.galleryFolderRepo.Delete(folder.ID)

	// Invalidate gallery access cache so removal takes effect immediately
	s.galleryAccess.Invalidate()

	c.JSON(http.StatusOK, dto.RemoveFolderResponse{
		Message:      string(i18n.MsgFolderRemoved),
		FilesRemoved: int(filesRemoved),
	})
}
