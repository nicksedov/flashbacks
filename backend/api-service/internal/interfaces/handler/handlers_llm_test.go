package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/domain/repository"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/testutil"

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
