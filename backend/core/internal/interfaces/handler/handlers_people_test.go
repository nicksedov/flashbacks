package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedFace stores a face on a seeded gallery photo.
func seedFace(t *testing.T, db *gorm.DB, imageID, personID uint, confidence float32) domain.Face {
	t.Helper()
	face := domain.Face{
		ImageFileID: imageID,
		PersonID:    personID,
		X:           1,
		Y:           2,
		Width:       30,
		Height:      40,
		Confidence:  confidence,
	}
	require.NoError(t, db.Create(&face).Error)
	return face
}

// TestHandleRenamePerson_RenamesAndValidates guards PATCH /api/people/:id.
func TestHandleRenamePerson_RenamesAndValidates(t *testing.T) {
	svc, _, cleanup := newFacesTestServer(t)
	defer cleanup()

	person := domain.Person{}
	require.NoError(t, svc.personRepo.Create(&person))

	w := serveHandlerWithParam(svc.handleRenamePerson, http.MethodPatch, "/api/people/1",
		"id", "1", dto.RenamePersonRequest{Name: "  Anna  "})
	require.Equal(t, http.StatusOK, w.Code)

	var renamed dto.PersonDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &renamed))
	require.Equal(t, "Anna", renamed.Name)

	stored, err := svc.personRepo.FindByID(person.ID)
	require.NoError(t, err)
	require.True(t, stored.Manual)

	// An empty name clears it.
	w = serveHandlerWithParam(svc.handleRenamePerson, http.MethodPatch, "/api/people/1",
		"id", "1", dto.RenamePersonRequest{Name: "   "})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &renamed))
	require.Equal(t, "", renamed.Name)

	// An over-long name is rejected and leaves the row alone.
	w = serveHandlerWithParam(svc.handleRenamePerson, http.MethodPatch, "/api/people/1",
		"id", "1", dto.RenamePersonRequest{Name: strings.Repeat("а", 201)})
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Unknown id.
	w = serveHandlerWithParam(svc.handleRenamePerson, http.MethodPatch, "/api/people/999",
		"id", "999", dto.RenamePersonRequest{Name: "Anna"})
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleCreatePerson_StatusCodes guards POST /api/people.
func TestHandleCreatePerson_StatusCodes(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	image := testutil.SeedImageFile(t, db, "/gallery/a.jpg", "h1", 10)
	first := seedFace(t, db, image.ID, 0, 0.9)
	second := seedFace(t, db, image.ID, 0, 0.8)

	w := serveHandler(svc.handleCreatePerson, http.MethodPost, "/api/people",
		dto.CreatePersonRequest{Name: "Василиса", FaceIDs: []uint{first.ID, second.ID}})
	require.Equal(t, http.StatusCreated, w.Code)

	var created dto.PersonDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "Василиса", created.Name)
	require.Equal(t, 1, created.PhotoCount)

	// The faces now belong to the created person.
	for _, faceID := range []uint{first.ID, second.ID} {
		face, err := svc.faceRepo.FindByID(faceID)
		require.NoError(t, err)
		require.Equal(t, created.ID, face.PersonID)
	}

	// An empty face list is a validation error.
	w = serveHandler(svc.handleCreatePerson, http.MethodPost, "/api/people",
		dto.CreatePersonRequest{Name: "Anna"})
	require.Equal(t, http.StatusBadRequest, w.Code)

	// An unknown face is a 404.
	w = serveHandler(svc.handleCreatePerson, http.MethodPost, "/api/people",
		dto.CreatePersonRequest{FaceIDs: []uint{999}})
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleMergePeople_StatusCodes guards POST /api/people/merge.
func TestHandleMergePeople_StatusCodes(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	image := testutil.SeedImageFile(t, db, "/gallery/a.jpg", "h1", 10)
	target := domain.Person{}
	source := domain.Person{}
	require.NoError(t, svc.personRepo.Create(&target))
	require.NoError(t, svc.personRepo.Create(&source))
	seedFace(t, db, image.ID, target.ID, 0.9)
	seedFace(t, db, image.ID, source.ID, 0.8)

	name := "Василиса"
	w := serveHandler(svc.handleMergePeople, http.MethodPost, "/api/people/merge",
		dto.MergePeopleRequest{TargetPersonID: target.ID, SourcePersonIDs: []uint{source.ID}, Name: &name})
	require.Equal(t, http.StatusOK, w.Code)

	var merged dto.PersonDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &merged))
	require.Equal(t, "Василиса", merged.Name)
	require.Equal(t, 1, merged.PhotoCount)

	_, err := svc.personRepo.FindByID(source.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// No sources is a validation error.
	w = serveHandler(svc.handleMergePeople, http.MethodPost, "/api/people/merge",
		dto.MergePeopleRequest{TargetPersonID: target.ID})
	require.Equal(t, http.StatusBadRequest, w.Code)

	// The target among the sources is a validation error.
	w = serveHandler(svc.handleMergePeople, http.MethodPost, "/api/people/merge",
		dto.MergePeopleRequest{TargetPersonID: target.ID, SourcePersonIDs: []uint{target.ID}})
	require.Equal(t, http.StatusBadRequest, w.Code)

	// An unknown source is a 404.
	w = serveHandler(svc.handleMergePeople, http.MethodPost, "/api/people/merge",
		dto.MergePeopleRequest{TargetPersonID: target.ID, SourcePersonIDs: []uint{999}})
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleDisbandPerson_StatusCodes guards DELETE /api/people/:id.
func TestHandleDisbandPerson_StatusCodes(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	image := testutil.SeedImageFile(t, db, "/gallery/a.jpg", "h1", 10)
	person := domain.Person{Name: "Василиса"}
	require.NoError(t, svc.personRepo.Create(&person))
	face := seedFace(t, db, image.ID, person.ID, 0.9)

	w := serveHandlerWithParam(svc.handleDisbandPerson, http.MethodDelete, "/api/people/1",
		"id", "1", nil)
	require.Equal(t, http.StatusOK, w.Code)

	_, err := svc.personRepo.FindByID(person.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	storedFace, err := svc.faceRepo.FindByID(face.ID)
	require.NoError(t, err)
	require.Equal(t, uint(0), storedFace.PersonID)

	// The photo survives.
	_, err = svc.imageFileRepo.FindByID(image.ID)
	require.NoError(t, err)

	w = serveHandlerWithParam(svc.handleDisbandPerson, http.MethodDelete, "/api/people/999",
		"id", "999", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleAssignFace_StatusCodes guards PATCH /api/faces/:id.
func TestHandleAssignFace_StatusCodes(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	image := testutil.SeedImageFile(t, db, "/gallery/a.jpg", "h1", 10)
	person := domain.Person{Name: "Василиса"}
	require.NoError(t, svc.personRepo.Create(&person))
	face := seedFace(t, db, image.ID, 0, 0.9)

	w := serveHandlerWithParam(svc.handleAssignFace, http.MethodPatch, "/api/faces/1",
		"id", "1", dto.AssignFaceRequest{PersonID: person.ID})
	require.Equal(t, http.StatusOK, w.Code)

	var assigned dto.ImageFaceDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &assigned))
	require.Equal(t, person.ID, assigned.PersonID)
	require.Equal(t, "Василиса", assigned.PersonName)
	require.Equal(t, float32(0.9), assigned.Confidence)

	// A personId of 0 detaches the face.
	w = serveHandlerWithParam(svc.handleAssignFace, http.MethodPatch, "/api/faces/1",
		"id", "1", dto.AssignFaceRequest{PersonID: 0})
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &assigned))
	require.Equal(t, uint(0), assigned.PersonID)

	_ = face

	// An unknown person is a 404.
	w = serveHandlerWithParam(svc.handleAssignFace, http.MethodPatch, "/api/faces/1",
		"id", "1", dto.AssignFaceRequest{PersonID: 999})
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleGetUnassignedFaces_Pagination guards GET /api/faces/unassigned.
func TestHandleGetUnassignedFaces_Pagination(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	image := testutil.SeedImageFile(t, db, "/gallery/a.jpg", "h1", 10)
	person := domain.Person{}
	require.NoError(t, svc.personRepo.Create(&person))
	for i := 0; i < 3; i++ {
		seedFace(t, db, image.ID, 0, 0.9)
	}
	seedFace(t, db, image.ID, person.ID, 0.9)

	w := serveHandler(svc.handleGetUnassignedFaces, http.MethodGet,
		"/api/faces/unassigned?page=1&pageSize=2", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp dto.UnassignedFacesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 3, resp.TotalFaces)
	require.Len(t, resp.Faces, 2)
	require.True(t, resp.HasNextPage)
	require.Equal(t, 2, resp.TotalPages)
	require.Equal(t, "/gallery/a.jpg", resp.Faces[0].Path)
	require.Equal(t, "a.jpg", resp.Faces[0].FileName)
	require.Equal(t, float32(30), resp.Faces[0].Width)
	require.Equal(t, float32(0.9), resp.Faces[0].Confidence)
}
