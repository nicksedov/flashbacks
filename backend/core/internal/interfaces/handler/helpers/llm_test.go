package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"github.com/gin-gonic/gin"
)

func TestCreateOCRClientResolvesOCRInstrument(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	factory := NewLLMFactory(db, 0)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	client, provider, instrument, ok := factory.CreateOCRClient(c)
	if !ok {
		t.Fatalf("CreateOCRClient failed: %s", w.Body.String())
	}
	if client == nil {
		t.Fatal("expected a non-nil client")
	}
	if instrument.Type != domain.InstrumentOCR {
		t.Errorf("instrument.Type = %q, want %q", instrument.Type, domain.InstrumentOCR)
	}
	if instrument.Model != "minicpm-v" {
		t.Errorf("instrument.Model = %q, want %q", instrument.Model, "minicpm-v")
	}
	if provider.Name != "ollama" {
		t.Errorf("provider.Name = %q, want %q", provider.Name, "ollama")
	}
}

func TestCreateOCRClientMissingInstrumentReturns404(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Simulate an install that has no OCR instrument configured yet.
	if err := db.Where("type = ?", domain.InstrumentOCR).Delete(&domain.LlmInstrumentSettings{}).Error; err != nil {
		t.Fatalf("failed to delete OCR instrument: %v", err)
	}

	factory := NewLLMFactory(db, 0)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	_, _, _, ok := factory.CreateOCRClient(c)
	if ok {
		t.Fatal("expected CreateOCRClient to fail when no OCR instrument exists")
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("response code = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCreateOCRClientIsIndependentFromVL(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Give the OCR instrument a model distinct from VL to prove independence.
	if err := db.Model(&domain.LlmInstrumentSettings{}).
		Where("type = ?", domain.InstrumentOCR).
		Update("model", "ocr-specialized").Error; err != nil {
		t.Fatalf("failed to update OCR instrument model: %v", err)
	}

	factory := NewLLMFactory(db, 0)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	_, _, ocrInstrument, ok := factory.CreateOCRClient(c)
	if !ok {
		t.Fatalf("CreateOCRClient failed: %s", w.Body.String())
	}
	if ocrInstrument.Model != "ocr-specialized" {
		t.Errorf("OCR instrument model = %q, want %q (must not follow VL)", ocrInstrument.Model, "ocr-specialized")
	}

	// The VL instrument must remain untouched.
	var vlInstrument domain.LlmInstrumentSettings
	if err := db.Where("type = ?", domain.InstrumentVL).First(&vlInstrument).Error; err != nil {
		t.Fatalf("failed to load VL instrument: %v", err)
	}
	if vlInstrument.Model == "ocr-specialized" {
		t.Error("VL instrument model was unexpectedly changed to the OCR model")
	}
}
