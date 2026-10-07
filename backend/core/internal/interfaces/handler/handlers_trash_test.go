package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newTrashTestServer builds a minimal Server wired for trash handler tests.
func newTrashTestServer(t *testing.T, trashDir string) (*Server, *gorm.DB, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)

	settings := domain.AppSettings{ID: 1, TrashDir: trashDir}
	if err := db.Save(&settings).Error; err != nil {
		t.Fatalf("failed to seed app settings: %v", err)
	}

	svc := &Server{
		db:             db,
		settingsLoader: helpers.NewSettingsLoader(db),
		thumbnailBatch: helpers.NewThumbnailBatch(nil),
	}
	return svc, db, cleanup
}

func makeTrashItem(t *testing.T, db *gorm.DB, fileName, trashPath, originalPath string, size int64, deletedAt time.Time) domain.TrashItem {
	t.Helper()
	item := domain.TrashItem{
		FileName:     fileName,
		TrashPath:    trashPath,
		OriginalPath: originalPath,
		Size:         size,
		DeletedAt:    deletedAt,
		OriginalHash: "hash-" + fileName,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("failed to create trash item: %v", err)
	}
	return item
}

// serveHandler runs a gin handler against an httptest request.
func serveHandler(h gin.HandlerFunc, method, target string, body any) *httptest.ResponseRecorder {
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
	h(ctx)
	return w
}

func TestHandleGetTrash_GroupingAndOrder(t *testing.T) {
	trashDir := fixtures.CreateTempDir(t)
	svc, db, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	// Three items: two on the same (newest) day, one on an older day.
	now := time.Now()
	older := now.AddDate(0, 0, -2)
	itemA := makeTrashItem(t, db, "a.jpg", filepath.ToSlash(filepath.Join(trashDir, "a.jpg")), "/gallery/a.jpg", 100, now.Add(-time.Hour))
	itemB := makeTrashItem(t, db, "b.jpg", filepath.ToSlash(filepath.Join(trashDir, "b.jpg")), "/gallery/b.jpg", 200, now.Add(-2*time.Hour))
	itemC := makeTrashItem(t, db, "c.jpg", filepath.ToSlash(filepath.Join(trashDir, "c.jpg")), "/gallery/c.jpg", 300, older)

	// Create the files on disk so reconciliation keeps them.
	for _, p := range []string{itemA.TrashPath, itemB.TrashPath, itemC.TrashPath} {
		fixtures.CreateTestFile(t, filepath.Dir(p), filepath.Base(p), []byte("x"))
	}

	w := serveHandler(svc.handleGetTrash, http.MethodGet, "/api/trash", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.TrashListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.TotalItems != 3 {
		t.Errorf("TotalItems = %d, want 3", resp.TotalItems)
	}
	if len(resp.Groups) != 2 {
		t.Fatalf("TotalGroups = %d, want 2 (got groups %+v)", len(resp.Groups), resp.Groups)
	}

	// Newest group first, items ordered by deletedAt descending.
	first := resp.Groups[0]
	second := resp.Groups[1]
	if first.Items[0].ID != itemA.ID || first.Items[1].ID != itemB.ID {
		t.Errorf("newest group items not ordered by deletedAt desc: %+v", first.Items)
	}
	if second.Items[0].ID != itemC.ID {
		t.Errorf("older group should contain item C: %+v", second.Items)
	}
	if first.Date == second.Date {
		t.Errorf("expected two distinct dates, got %q and %q", first.Date, second.Date)
	}
	if first.ItemCount != 2 {
		t.Errorf("newest group ItemCount = %d, want 2", first.ItemCount)
	}
}

func TestHandleRestoreTrashFile_RestoresToOriginalPath(t *testing.T) {
	srcDir := fixtures.CreateTempDir(t)
	trashDir := fixtures.CreateTempDir(t)
	svc, db, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	originalPath := filepath.ToSlash(filepath.Join(srcDir, "photo.jpg"))

	// Create the source file, then move it into trash via the FileMover (records the row).
	fixtures.CreateTestFile(t, srcDir, "photo.jpg", []byte("jpeg-bytes"))
	testutil.SeedImageFile(t, db, originalPath, "hash-abc", 10)
	fm := helpers.NewFileMover(db)
	if err := fm.MoveToTrashOrDelete(filepath.FromSlash(originalPath), trashDir); err != nil {
		t.Fatalf("failed to move to trash: %v", err)
	}

	var item domain.TrashItem
	if err := db.First(&item).Error; err != nil {
		t.Fatalf("expected TrashItem after move: %v", err)
	}

	w := serveHandler(svc.handleRestoreTrashFile, http.MethodPost, "/api/trash-restore", map[string]any{"id": item.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// File should be back at the original path.
	if _, err := os.Stat(originalPath); err != nil {
		t.Fatalf("expected file restored to original path %q: %v", originalPath, err)
	}

	// Row should be removed.
	var count int64
	db.Model(&domain.TrashItem{}).Count(&count)
	if count != 0 {
		t.Errorf("expected TrashItem row removed after restore, got %d", count)
	}

	// image_files row should be re-inserted.
	var imgCount int64
	db.Model(&domain.ImageFile{}).Where("path = ?", originalPath).Count(&imgCount)
	if imgCount != 1 {
		t.Errorf("expected image_files row re-inserted for %q, got %d", originalPath, imgCount)
	}
}

func TestHandleRestoreTrashFile_UnknownOriginalPath(t *testing.T) {
	trashDir := fixtures.CreateTempDir(t)
	svc, db, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	item := makeTrashItem(t, db, "legacy.jpg", filepath.ToSlash(filepath.Join(trashDir, "legacy.jpg")), "", 10, time.Now())
	fixtures.CreateTestFile(t, trashDir, "legacy.jpg", []byte("x"))

	w := serveHandler(svc.handleRestoreTrashFile, http.MethodPost, "/api/trash-restore", map[string]any{"id": item.ID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown original path, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteTrashFile_PermanentDelete(t *testing.T) {
	trashDir := fixtures.CreateTempDir(t)
	svc, db, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	item := makeTrashItem(t, db, "del.jpg", filepath.ToSlash(filepath.Join(trashDir, "del.jpg")), "/gallery/del.jpg", 10, time.Now())
	trashPath := fixtures.CreateTestFile(t, trashDir, "del.jpg", []byte("x"))

	w := serveHandler(svc.handleDeleteTrashFile, http.MethodPost, "/api/trash-delete", map[string]any{"id": item.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(trashPath); !os.IsNotExist(err) {
		t.Errorf("expected file removed from trash dir, stat err = %v", err)
	}

	var count int64
	db.Model(&domain.TrashItem{}).Count(&count)
	if count != 0 {
		t.Errorf("expected TrashItem row removed after delete, got %d", count)
	}
}

func TestHandleCleanTrash_ClearsFilesAndMetadata(t *testing.T) {
	trashDir := fixtures.CreateTempDir(t)
	svc, db, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	makeTrashItem(t, db, "one.jpg", filepath.ToSlash(filepath.Join(trashDir, "one.jpg")), "/gallery/one.jpg", 10, time.Now())
	makeTrashItem(t, db, "two.jpg", filepath.ToSlash(filepath.Join(trashDir, "two.jpg")), "/gallery/two.jpg", 20, time.Now())
	fixtures.CreateTestFile(t, trashDir, "one.jpg", []byte("1"))
	fixtures.CreateTestFile(t, trashDir, "two.jpg", []byte("22"))

	w := serveHandler(svc.handleCleanTrash, http.MethodPost, "/api/trash-clean", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.CleanTrashResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Deleted != 2 {
		t.Errorf("Deleted = %d, want 2", resp.Deleted)
	}

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("failed to read trash dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty trash dir, got %d entries", len(entries))
	}

	var count int64
	db.Model(&domain.TrashItem{}).Count(&count)
	if count != 0 {
		t.Errorf("expected all TrashItem rows cleared after clean, got %d", count)
	}
}

func TestHandleServeTrashImage_OutsideTrashRejected(t *testing.T) {
	trashDir := fixtures.CreateTempDir(t)
	outsideDir := fixtures.CreateTempDir(t)
	svc, _, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	outsideFile := fixtures.CreateTestFile(t, outsideDir, "secret.jpg", []byte("secret"))

	reqPath := "/api/trash/image?path=" + url.QueryEscape(outsideFile)
	w := serveHandler(svc.handleServeTrashImage, http.MethodGet, reqPath, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for path outside trash, got %d", w.Code)
	}
}

func TestHandleServeTrashImage_InsideTrashServed(t *testing.T) {
	trashDir := fixtures.CreateTempDir(t)
	svc, _, cleanup := newTrashTestServer(t, trashDir)
	defer cleanup()

	trashFile := fixtures.CreateTestFile(t, trashDir, "photo.jpg", []byte("data"))

	reqPath := "/api/trash/image?path=" + url.QueryEscape(trashFile)
	w := serveHandler(svc.handleServeTrashImage, http.MethodGet, reqPath, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for path inside trash, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "data" {
		t.Errorf("expected file contents, got %q", w.Body.String())
	}
}
