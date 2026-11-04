package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/interfaces/handler/helpers"
	"github.com/flashbacks/core/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleGetPeople returns the people discovered by face recognition.
func (s *Server) handleGetPeople(c *gin.Context) {
	people, err := s.personRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	dtos := make([]dto.PersonDTO, 0, len(people))
	for _, person := range people {
		photoCount, err := s.faceRepo.CountImageFilesByPersonID(person.ID)
		if err != nil {
			photoCount = 0
		}
		dtos = append(dtos, dto.PersonDTO{
			ID:             person.ID,
			Name:           person.Name,
			PhotoCount:     int(photoCount),
			CoverThumbnail: s.coverThumbnail(person.CoverFaceID),
		})
	}

	c.JSON(http.StatusOK, dto.PeopleResponse{People: dtos})
}

// coverThumbnail resolves a person's cover face to a thumbnail URL reference.
func (s *Server) coverThumbnail(coverFaceID *uint) string {
	if coverFaceID == nil {
		return ""
	}
	face, err := s.faceRepo.FindByID(*coverFaceID)
	if err != nil {
		return ""
	}
	imageFile, err := s.imageFileRepo.FindByID(face.ImageFileID)
	if err != nil {
		return ""
	}
	return "/api/thumbnail?path=" + url.QueryEscape(imageFile.Path)
}

// handleGetPersonPhotos returns the paginated photos of a person.
func (s *Server) handleGetPersonPhotos(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	if _, err := s.personRepo.FindByID(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgAuthUserNotFound))
		return
	}

	params := helpers.ParsePagination(c, helpers.ModeFlexible)
	total, err := s.faceRepo.CountImageFilesByPersonID(uint(id))
	if err != nil {
		total = 0
	}
	pag := helpers.CalcPagination(params.Page, params.PageSize, total)

	files, err := s.faceRepo.ListImageFilesByPersonID(uint(id), pag.PageSize*(pag.Page-1), pag.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	images := helpers.BuildGalleryImageDTOs(files)
	c.JSON(http.StatusOK, dto.PersonPhotosResponse{
		Images:      images,
		TotalImages: int(total),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleGetFacesStatus returns the face recognition job status.
func (s *Server) handleGetFacesStatus(c *gin.Context) {
	if s.faceRecognition == nil {
		c.JSON(http.StatusServiceUnavailable, dto.FaceStatusResponse{})
		return
	}
	status := s.faceRecognition.GetStatus()
	c.JSON(http.StatusOK, dto.FaceStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}

// handleStartFaces launches the face recognition job.
func (s *Server) handleStartFaces(c *gin.Context) {
	if s.faceRecognition == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgFacesManagerNotAvailable)
		return
	}
	schedule := s.settingsLoader.Schedule(domain.ScheduleFaceRecognition)
	s.faceRecognition.Start(schedule)
	s.respondSuccess(c, http.StatusOK, i18n.MsgFacesStarted)
}

// handleStopFaces stops the face recognition job.
func (s *Server) handleStopFaces(c *gin.Context) {
	if s.faceRecognition == nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgFacesManagerNotAvailable)
		return
	}
	s.faceRecognition.Stop()
	s.respondSuccess(c, http.StatusOK, i18n.MsgFacesStopped)
}
