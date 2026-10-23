package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/application/imaging"
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/interfaces/handler/helpers"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// newLlmTestServer builds a minimal Server wired for LLM routing tests.
func newLlmTestServer(t *testing.T) (*Server, *gorm.DB, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)

	svc := &Server{
		db:             db,
		settingsLoader: helpers.NewSettingsLoader(db),
		llmFactory:     helpers.NewLLMFactory(db, 0),
		llmOcrService:  imaging.NewLlmOcrService(db),
		llmRepo:        repository.NewLlmRepository(db),
		imageFileRepo:  repository.NewImageFileRepository(db),
		imageTagRepo:   repository.NewImageTagRepository(db),
	}
	return svc, db, cleanup
}

func TestInstrumentTypeFromStringIncludesOCR(t *testing.T) {
	if got := instrumentTypeFromString("ocr"); got != domain.InstrumentOCR {
		t.Errorf(`instrumentTypeFromString("ocr") = %q, want %q`, got, domain.InstrumentOCR)
	}
}

// TestHandleAiAction_RoutesRecognizeTextToOCRInstrument verifies the recognizeText
// action resolves the ocr instrument while describe/tags/askQuestion keep using vl.
func TestHandleAiAction_RoutesRecognizeTextToOCRInstrument(t *testing.T) {
	svc, db, cleanup := newLlmTestServer(t)
	defer cleanup()
	defer svc.llmOcrService.Stop()

	testutil.SeedImageFile(t, db, "/gallery/photo.jpg", "hash-1", 100)

	// recognizeText succeeds (202) when the OCR instrument exists.
	w := serveHandler(svc.handleAiAction, http.MethodPost, "/api/ai/action", dto.AiActionRequest{
		ImagePath: "/gallery/photo.jpg",
		Action:    dto.AiActionRecognizeText,
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("recognizeText with OCR instrument: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	// Removing the VL instrument must NOT affect recognizeText (it uses OCR)...
	var vl domain.LlmInstrumentSettings
	if err := db.Where("type = ?", domain.InstrumentVL).First(&vl).Error; err != nil {
		t.Fatalf("failed to load VL instrument: %v", err)
	}
	if err := db.Delete(&vl).Error; err != nil {
		t.Fatalf("failed to delete VL instrument: %v", err)
	}

	w = serveHandler(svc.handleAiAction, http.MethodPost, "/api/ai/action", dto.AiActionRequest{
		ImagePath: "/gallery/photo.jpg",
		Action:    dto.AiActionRecognizeText,
	})
	if w.Code != http.StatusAccepted {
		t.Errorf("recognizeText without VL instrument: expected 202 (uses OCR), got %d: %s", w.Code, w.Body.String())
	}

	// ...but the VL-only actions must fail cleanly once the VL instrument is gone.
	w = serveHandler(svc.handleAiAction, http.MethodPost, "/api/ai/action", dto.AiActionRequest{
		ImagePath: "/gallery/photo.jpg",
		Action:    dto.AiActionDescribe,
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("describe without VL instrument: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Removing the OCR instrument makes recognizeText fail cleanly (no VL fallback).
	if err := db.Where("type = ?", domain.InstrumentOCR).Delete(&domain.LlmInstrumentSettings{}).Error; err != nil {
		t.Fatalf("failed to delete OCR instrument: %v", err)
	}
	w = serveHandler(svc.handleAiAction, http.MethodPost, "/api/ai/action", dto.AiActionRequest{
		ImagePath: "/gallery/photo.jpg",
		Action:    dto.AiActionRecognizeText,
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("recognizeText without OCR instrument: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Let background async tasks (which fail fast against the unreachable local
	// LLM endpoint) finish before cleanup closes the test DB.
	time.Sleep(200 * time.Millisecond)
}

// TestHandleLlmRecognize_UsesOCRInstrument verifies POST /api/llm/recognize
// resolves the ocr instrument instead of vl.
func TestHandleLlmRecognize_UsesOCRInstrument(t *testing.T) {
	svc, db, cleanup := newLlmTestServer(t)
	defer cleanup()
	defer svc.llmOcrService.Stop()

	testutil.SeedImageFile(t, db, "/gallery/photo.jpg", "hash-1", 100)

	w := serveHandler(svc.handleLlmRecognize, http.MethodPost, "/api/llm/recognize", dto.LlmOcrRequest{
		ImagePath: "/gallery/photo.jpg",
		Force:     true,
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("recognize with OCR instrument: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	// Without the OCR instrument the endpoint must fail cleanly (404), not fall back to VL.
	if err := db.Where("type = ?", domain.InstrumentOCR).Delete(&domain.LlmInstrumentSettings{}).Error; err != nil {
		t.Fatalf("failed to delete OCR instrument: %v", err)
	}

	w = serveHandler(svc.handleLlmRecognize, http.MethodPost, "/api/llm/recognize", dto.LlmOcrRequest{
		ImagePath: "/gallery/photo.jpg",
		Force:     true,
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("recognize without OCR instrument: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	time.Sleep(200 * time.Millisecond)
}

// getLlmSettings serves GET /api/llm-settings and decodes the response.
func getLlmSettings(t *testing.T, svc *Server) dto.LlmSettingsResponse {
	t.Helper()
	w := serveHandler(svc.handleGetLlmSettings, http.MethodGet, "/api/llm-settings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET llm settings: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp dto.LlmSettingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal llm settings response: %v", err)
	}
	return resp
}

// tag-scan-schedule.days-persisted
func TestUpdateLlmSettingsTagScanDaysPersisted(t *testing.T) {
	svc, db, cleanup := newLlmTestServer(t)
	defer cleanup()

	days := "1,3,5"
	w := serveHandler(svc.handleUpdateLlmSettings, http.MethodPut, "/api/llm-settings", dto.UpdateLlmSettingsRequest{
		TagScanDays: &days,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT llm settings: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stored domain.TagScanSettings
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored tag scan settings: %v", err)
	}
	if stored.Days != days {
		t.Errorf("stored tag scan days = %q, want %q", stored.Days, days)
	}

	if got := getLlmSettings(t, svc).TagScan.Days; got != days {
		t.Errorf("response tagScan.days = %q, want %q", got, days)
	}
}

// TestGetLlmSettingsDefaultsTagScanWeekdays covers the default weekday set
// used when no tag scan settings row exists yet.
func TestGetLlmSettingsDefaultsTagScanWeekdays(t *testing.T) {
	svc, db, cleanup := newLlmTestServer(t)
	defer cleanup()

	if err := db.Exec("DELETE FROM tag_scan_settings").Error; err != nil {
		t.Fatalf("failed to clear tag scan settings: %v", err)
	}

	if got := getLlmSettings(t, svc).TagScan.Days; got != "1,2,3,4,5" {
		t.Errorf("default tagScan.days = %q, want %q", got, "1,2,3,4,5")
	}
}
