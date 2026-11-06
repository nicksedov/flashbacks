package handler

import (
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	recognition "github.com/flashbacks/core/internal/application/faces"
	"github.com/flashbacks/core/internal/application/people"
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
		if err != nil || photoCount == 0 {
			// A person whose faces were re-scanned away stays in the table but is
			// hidden from the listing: an empty tile helps nobody.
			continue
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
		LastError: status.Progress.LastError,
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

// handleGetImageFaces returns the faces detected on one gallery image, each with
// its box in the upright frame of that image and the person it was clustered
// into.
//
// The analyser reads the stored pixels as they are, so a stored box is in the raw
// frame. /api/image serves the untouched file and the browser rotates it upright
// (EXIF Orientation), which is the frame the lightbox draws over, so every box is
// mapped through the image's orientation here.
func (s *Server) handleGetImageFaces(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Same normalization and gallery check every other image endpoint applies.
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}
	normalizedPath := filepath.ToSlash(absPath)
	if !s.galleryAccess.VerifyGalleryAccess(c, normalizedPath) {
		return
	}

	// An image the scanner has not recorded yet has no faces either; answering
	// with an empty list keeps the lightbox tab on its normal "no faces" path
	// instead of showing a not-found error for a file the user just opened.
	imageFile, err := s.imageFileRepo.FindByPath(normalizedPath)
	if err != nil {
		c.JSON(http.StatusOK, dto.ImageFacesResponse{Faces: []dto.ImageFaceDTO{}})
		return
	}

	faces, err := s.faceRepo.FindByImageFileID(imageFile.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	frame := recognition.ReadImageFrame(normalizedPath)
	names := make(map[uint]string, len(faces))
	dtos := make([]dto.ImageFaceDTO, 0, len(faces))
	for _, face := range faces {
		x, y, width, height := frame.NormalizeBox(face.X, face.Y, face.Width, face.Height)
		dtos = append(dtos, dto.ImageFaceDTO{
			ID:         face.ID,
			PersonID:   face.PersonID,
			PersonName: s.personName(names, face.PersonID),
			X:          x,
			Y:          y,
			Width:      width,
			Height:     height,
			Confidence: face.Confidence,
		})
	}

	c.JSON(http.StatusOK, dto.ImageFacesResponse{Faces: dtos})
}

// handleRenamePerson sets or clears a person's display name.
func (s *Server) handleRenamePerson(c *gin.Context) {
	if !s.requirePeopleCatalog(c) {
		return
	}
	personID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.RenamePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgPeopleInvalidInput))
		return
	}

	person, err := s.peopleCatalog.Rename(personID, req.Name)
	if err != nil {
		s.respondCatalogError(c, err)
		return
	}
	c.JSON(http.StatusOK, s.personDTO(person))
}

// handleCreatePerson creates a person from previously unassigned faces.
func (s *Server) handleCreatePerson(c *gin.Context) {
	if !s.requirePeopleCatalog(c) {
		return
	}

	var req dto.CreatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgPeopleInvalidInput))
		return
	}

	person, err := s.peopleCatalog.CreateFromFaces(req.Name, req.FaceIDs)
	if err != nil {
		s.respondCatalogError(c, err)
		return
	}
	c.JSON(http.StatusCreated, s.personDTO(person))
}

// handleMergePeople merges the source people into the target person.
func (s *Server) handleMergePeople(c *gin.Context) {
	if !s.requirePeopleCatalog(c) {
		return
	}

	var req dto.MergePeopleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgPeopleInvalidInput))
		return
	}

	person, err := s.peopleCatalog.Merge(req.TargetPersonID, req.SourcePersonIDs, req.Name)
	if err != nil {
		s.respondCatalogError(c, err)
		return
	}
	c.JSON(http.StatusOK, s.personDTO(person))
}

// handleDisbandPerson returns a person's faces to unassigned and removes the
// person, keeping the photos.
func (s *Server) handleDisbandPerson(c *gin.Context) {
	if !s.requirePeopleCatalog(c) {
		return
	}
	personID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := s.peopleCatalog.Disband(personID); err != nil {
		s.respondCatalogError(c, err)
		return
	}
	s.respondSuccess(c, http.StatusOK, i18n.MsgPeopleDisbanded)
}

// handleAssignFace moves a face to a person, or detaches it when personId is 0.
func (s *Server) handleAssignFace(c *gin.Context) {
	if !s.requirePeopleCatalog(c) {
		return
	}
	faceID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.AssignFaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgPeopleInvalidInput))
		return
	}

	face, err := s.peopleCatalog.AssignFace(faceID, req.PersonID)
	if err != nil {
		s.respondCatalogError(c, err)
		return
	}

	// The client redraws this box straight over the served photo, so it has to be
	// in the same upright frame handleGetImageFaces reports.
	x, y, width, height := s.faceFrame(face).NormalizeBox(face.X, face.Y, face.Width, face.Height)
	names := make(map[uint]string, 1)
	c.JSON(http.StatusOK, dto.ImageFaceDTO{
		ID:         face.ID,
		PersonID:   face.PersonID,
		PersonName: s.personName(names, face.PersonID),
		X:          x,
		Y:          y,
		Width:      width,
		Height:     height,
		Confidence: face.Confidence,
	})
}

// faceFrame resolves the pixel frame of the image a face was detected on. An
// unknown image degrades to the identity frame, which leaves the box untouched.
func (s *Server) faceFrame(face *domain.Face) recognition.ImageFrame {
	imageFile, err := s.imageFileRepo.FindByID(face.ImageFileID)
	if err != nil {
		return recognition.ImageFrame{Orientation: 1}
	}
	return recognition.ReadImageFrame(imageFile.Path)
}

// handleGetUnassignedFaces lists the faces that belong to no person.
func (s *Server) handleGetUnassignedFaces(c *gin.Context) {
	if !s.requirePeopleCatalog(c) {
		return
	}

	params := helpers.ParsePagination(c, helpers.ModeFlexible)
	page, err := s.peopleCatalog.ListUnassigned(params.Page, params.PageSize)
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgPeopleListFailed)
		return
	}

	// The unassigned grid draws each box over the served photo, so the boxes are
	// mapped into the upright frame too. Several faces of one photo share a probe.
	frames := make(map[string]recognition.ImageFrame, len(page.Faces))
	faces := make([]dto.UnassignedFaceDTO, 0, len(page.Faces))
	for _, entry := range page.Faces {
		frame, probed := frames[entry.Path]
		if !probed {
			frame = recognition.ReadImageFrame(entry.Path)
			frames[entry.Path] = frame
		}
		x, y, width, height := frame.NormalizeBox(entry.Face.X, entry.Face.Y, entry.Face.Width, entry.Face.Height)
		faces = append(faces, dto.UnassignedFaceDTO{
			ID:         entry.Face.ID,
			PersonID:   entry.Face.PersonID,
			Path:       entry.Path,
			FileName:   entry.FileName,
			X:          x,
			Y:          y,
			Width:      width,
			Height:     height,
			Confidence: entry.Face.Confidence,
		})
	}

	c.JSON(http.StatusOK, dto.UnassignedFacesResponse{
		Faces:       faces,
		TotalFaces:  page.TotalFaces,
		CurrentPage: page.CurrentPage,
		PageSize:    page.PageSize,
		TotalPages:  page.TotalPages,
		HasNextPage: page.HasNextPage,
	})
}

// personDTO renders one person with its photo count and cover thumbnail.
func (s *Server) personDTO(person *domain.Person) dto.PersonDTO {
	photoCount, err := s.faceRepo.CountImageFilesByPersonID(person.ID)
	if err != nil {
		photoCount = 0
	}
	return dto.PersonDTO{
		ID:             person.ID,
		Name:           person.Name,
		PhotoCount:     int(photoCount),
		CoverThumbnail: s.coverThumbnail(person.CoverFaceID),
	}
}

// parseUintParam reads a numeric path parameter or answers 400.
func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgPeopleInvalidInput))
		return 0, false
	}
	return uint(value), true
}

// requirePeopleCatalog guards the catalog handlers when the service is absent
// (a partially wired server), answering 503 instead of panicking.
func (s *Server) requirePeopleCatalog(c *gin.Context) bool {
	if s.peopleCatalog != nil {
		return true
	}
	s.respondError(c, http.StatusServiceUnavailable, i18n.MsgPeopleOperationFailed)
	return false
}

// respondCatalogError maps a catalog error to the documented status code.
func (s *Server) respondCatalogError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, people.ErrPersonNotFound):
		s.respondError(c, http.StatusNotFound, i18n.MsgPeoplePersonNotFound)
	case errors.Is(err, people.ErrFaceNotFound):
		s.respondError(c, http.StatusNotFound, i18n.MsgPeopleFaceNotFound)
	case errors.Is(err, people.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.MsgPeopleInvalidInput))
	default:
		s.respondError(c, http.StatusInternalServerError, i18n.MsgPeopleOperationFailed)
	}
}

// personName resolves a person's display name once per request: a group photo
// repeats the same people across several faces, and an unnamed person reports an
// empty name so the client can render its localized placeholder.
func (s *Server) personName(cache map[uint]string, personID uint) string {
	if personID == 0 {
		return ""
	}
	if name, cached := cache[personID]; cached {
		return name
	}

	name := ""
	if person, err := s.personRepo.FindByID(personID); err == nil {
		name = person.Name
	}
	cache[personID] = name
	return name
}
