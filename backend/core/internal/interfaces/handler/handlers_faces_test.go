package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	recognition "github.com/flashbacks/core/internal/application/faces"
	catalogpeople "github.com/flashbacks/core/internal/application/people"
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/interfaces/handler/helpers"
	"github.com/flashbacks/core/internal/interfaces/i18n"
	"github.com/flashbacks/core/internal/testutil"
	"github.com/flashbacks/core/internal/testutil/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newFacesTestServer builds a Server wired for the people/faces handlers.
func newFacesTestServer(t *testing.T) (*Server, *gorm.DB, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	i18nSvc, err := i18n.NewService()
	require.NoError(t, err)

	personRepo := repository.NewPersonRepository(db)
	faceRepo := repository.NewFaceRepository(db)
	imageRepo := repository.NewImageFileRepository(db)
	referenceRepo := repository.NewPersonReferenceRepository(db)

	svc := &Server{
		db:             db,
		i18n:           i18nSvc,
		personRepo:     personRepo,
		faceRepo:       faceRepo,
		imageFileRepo:  imageRepo,
		peopleCatalog:  catalogpeople.NewCatalog(db, personRepo, faceRepo, referenceRepo, imageRepo),
		settingsLoader: helpers.NewSettingsLoader(db),
		scheduleRepo:   repository.NewScheduleRepository(db),
	}
	return svc, db, cleanup
}

func serveHandlerWithParam(h gin.HandlerFunc, method, target string, paramName, paramValue string, body any) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic("failed to encode request body: " + err.Error())
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	if paramName != "" {
		ctx.Params = gin.Params{{Key: paramName, Value: paramValue}}
	}
	h(ctx)
	return w
}

// TestHandleGetPeople_ListsWithCoverAndCount guards the people listing endpoint.
func TestHandleGetPeople_ListsWithCoverAndCount(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	person := domain.Person{CoverFaceID: nil}
	require.NoError(t, svc.personRepo.Create(&person))
	img1 := testutil.SeedImageFile(t, db, "/gallery/p1.jpg", "h1", 10)
	img2 := testutil.SeedImageFile(t, db, "/gallery/p2.jpg", "h2", 20)
	face1 := domain.Face{ImageFileID: img1.ID, X: 1, Y: 2, Width: 3, Height: 4, Confidence: 0.9}
	face2 := domain.Face{ImageFileID: img2.ID, X: 5, Y: 6, Width: 7, Height: 8, Confidence: 0.8}
	require.NoError(t, svc.faceRepo.Create(&face1))
	require.NoError(t, svc.faceRepo.Create(&face2))
	require.NoError(t, svc.faceRepo.SetPersonID(face1.ID, person.ID))
	require.NoError(t, svc.faceRepo.SetPersonID(face2.ID, person.ID))
	require.NoError(t, svc.personRepo.Save(&domain.Person{ID: person.ID, CoverFaceID: &face1.ID}))

	w := serveHandler(svc.handleGetPeople, http.MethodGet, "/api/people", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp dto.PeopleResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.People, 1)
	require.Equal(t, 2, resp.People[0].PhotoCount)
	require.NotEmpty(t, resp.People[0].CoverThumbnail)
}

// TestHandleGetPersonPhotos_Paginated guards the person photos endpoint.
func TestHandleGetPersonPhotos_Paginated(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	person := domain.Person{}
	require.NoError(t, svc.personRepo.Create(&person))
	for i := 0; i < 3; i++ {
		img := testutil.SeedImageFile(t, db, "/gallery/p"+string(rune('a'+i))+".jpg", "hash"+string(rune('0'+i)), int64(i))
		face := domain.Face{ImageFileID: img.ID, X: 0, Y: 0, Width: 1, Height: 1, Confidence: 0.9}
		require.NoError(t, svc.faceRepo.Create(&face))
		require.NoError(t, svc.faceRepo.SetPersonID(face.ID, person.ID))
	}

	w := serveHandlerWithParam(svc.handleGetPersonPhotos, http.MethodGet, "/api/people/1/photos?page=1&pageSize=2", "id", "1", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp dto.PersonPhotosResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 3, resp.TotalImages)
	require.Len(t, resp.Images, 2)
	require.True(t, resp.HasNextPage)
}

// TestHandleFacesSettingsPersisted guards that face recognition settings are
// persisted through the LLM settings update path.
func TestHandleFacesSettingsPersisted(t *testing.T) {
	svc, _, cleanup := newFacesTestServer(t)
	defer cleanup()

	enabled := true
	days := "1,3"
	startHour := 22
	endHour := 7
	w := serveHandler(svc.handleUpdateLlmSettings, http.MethodPut, "/api/llm/settings", dto.UpdateLlmSettingsRequest{
		FaceRecognitionEnabled:   &enabled,
		FaceRecognitionDays:      &days,
		FaceRecognitionStartHour: &startHour,
		FaceRecognitionEndHour:   &endHour,
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	schedule := svc.settingsLoader.Schedule(domain.ScheduleFaceRecognition)
	require.True(t, schedule.Enabled)
	require.Equal(t, "1,3", schedule.Days)
	require.Equal(t, 22, schedule.StartHour)
	require.Equal(t, 7, schedule.EndHour)
}

// newImageFacesServer builds the faces test server with gallery access, so the
// per-image faces handler can verify the requested path.
func newImageFacesServer(t *testing.T) (*Server, *gorm.DB, func()) {
	t.Helper()
	svc, db, cleanup := newFacesTestServer(t)
	svc.galleryAccess = helpers.NewGalleryAccess(db)
	testutil.SeedGalleryFolder(t, db, "/gallery")
	return svc, db, cleanup
}

// TestHandleGetImageFaces_ListsDetectedFaces guards that the endpoint answers one
// entry per detected face on the requested image.
func TestHandleGetImageFaces_ListsDetectedFaces(t *testing.T) {
	svc, db, cleanup := newImageFacesServer(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/gallery/group.jpg", "h1", 10)
	first := domain.Face{ImageFileID: img.ID, X: 1, Y: 2, Width: 3, Height: 4, Confidence: 0.9}
	second := domain.Face{ImageFileID: img.ID, X: 5, Y: 6, Width: 7, Height: 8, Confidence: 0.8}
	require.NoError(t, svc.faceRepo.Create(&first))
	require.NoError(t, svc.faceRepo.Create(&second))

	w := serveHandler(svc.handleGetImageFaces, http.MethodGet, "/api/image-faces?path=/gallery/group.jpg", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp dto.ImageFacesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Faces, 2)
	require.Equal(t, first.ID, resp.Faces[0].ID)
	require.Equal(t, second.ID, resp.Faces[1].ID)
}

// TestHandleGetImageFaces_ReportsPersonName guards that a clustered face reports
// the person it belongs to, name included.
func TestHandleGetImageFaces_ReportsPersonName(t *testing.T) {
	svc, db, cleanup := newImageFacesServer(t)
	defer cleanup()

	person := domain.Person{Name: "Anna"}
	require.NoError(t, svc.personRepo.Create(&person))
	img := testutil.SeedImageFile(t, db, "/gallery/anna.jpg", "h2", 20)
	face := domain.Face{ImageFileID: img.ID, X: 1, Y: 1, Width: 2, Height: 2, Confidence: 0.95}
	require.NoError(t, svc.faceRepo.Create(&face))
	require.NoError(t, svc.faceRepo.SetPersonID(face.ID, person.ID))

	w := serveHandler(svc.handleGetImageFaces, http.MethodGet, "/api/image-faces?path=/gallery/anna.jpg", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp dto.ImageFacesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Faces, 1)
	require.Equal(t, person.ID, resp.Faces[0].PersonID)
	require.Equal(t, "Anna", resp.Faces[0].PersonName)
}

// TestHandleGetImageFaces_UnassignedFaceHasNoPerson guards the noise case: a face
// no person claims reports person id 0, which the client draws unnumbered.
func TestHandleGetImageFaces_UnassignedFaceHasNoPerson(t *testing.T) {
	svc, db, cleanup := newImageFacesServer(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/gallery/crowd.jpg", "h3", 30)
	face := domain.Face{ImageFileID: img.ID, X: 1, Y: 1, Width: 2, Height: 2, Confidence: 0.7}
	require.NoError(t, svc.faceRepo.Create(&face))

	w := serveHandler(svc.handleGetImageFaces, http.MethodGet, "/api/image-faces?path=/gallery/crowd.jpg", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp dto.ImageFacesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Faces, 1)
	require.Zero(t, resp.Faces[0].PersonID)
	require.Empty(t, resp.Faces[0].PersonName)
}

// TestHandleGetImageFaces_EmptyWithoutFaces guards that an image the analyser
// found no faces on answers an empty list rather than an error.
func TestHandleGetImageFaces_EmptyWithoutFaces(t *testing.T) {
	svc, db, cleanup := newImageFacesServer(t)
	defer cleanup()

	testutil.SeedImageFile(t, db, "/gallery/landscape.jpg", "h4", 40)

	w := serveHandler(svc.handleGetImageFaces, http.MethodGet, "/api/image-faces?path=/gallery/landscape.jpg", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp dto.ImageFacesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Empty(t, resp.Faces)
}

// TestHandleFacesStart_JobStarted guards that launching face recognition
// reports the job as running.
func TestHandleFacesStart_JobStarted(t *testing.T) {
	svc, db, cleanup := newFacesTestServer(t)
	defer cleanup()

	manager := recognition.NewManager(db, &mocks.MockFacesClient{})
	svc.faceRecognition = manager
	defer manager.Stop()

	w := serveHandler(svc.handleStartFaces, http.MethodPost, "/api/faces/start", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	w = serveHandler(svc.handleGetFacesStatus, http.MethodGet, "/api/faces/status", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var status dto.FaceStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	require.True(t, status.Running)
}
