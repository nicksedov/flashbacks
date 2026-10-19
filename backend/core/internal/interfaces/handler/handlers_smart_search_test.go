package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/interfaces/i18n"
	"github.com/flashbacks/core/internal/testutil"
)

// newSmartSearchTestServer builds a minimal Server wired for smart search tests.
func newSmartSearchTestServer(t *testing.T) *Server {
	t.Helper()

	db, cleanup := testutil.NewTestDB(t)
	t.Cleanup(cleanup)

	svc, err := i18n.NewService()
	if err != nil {
		t.Fatalf("failed to create i18n service: %v", err)
	}

	return &Server{db: db, i18n: svc}
}

// decodeErrorMessage returns the resolved "error" field of an error response.
func decodeErrorMessage(t *testing.T, body []byte) string {
	t.Helper()

	var resp map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return resp["error"]
}

func TestHandleSmartSearch_NotIndexed(t *testing.T) {
	srv := newSmartSearchTestServer(t)

	w := serveHandler(srv.handleSmartSearch, http.MethodGet, "/api/gallery/smart-search?q=sunset", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	msg := decodeErrorMessage(t, w.Body.Bytes())
	if msg == string(i18n.MsgSmartSearchNotIndexed) {
		t.Errorf("error message was not translated: %q", msg)
	}
	if !strings.Contains(msg, "not been indexed") {
		t.Errorf("expected a human-readable indexing hint, got %q", msg)
	}
}

func TestHandleSmartSearch_NotConfigured(t *testing.T) {
	srv := newSmartSearchTestServer(t)
	if err := srv.db.Where("type = ?", domain.InstrumentEmbedding).
		Delete(&domain.LlmInstrumentSettings{}).Error; err != nil {
		t.Fatalf("failed to remove embedding instrument: %v", err)
	}

	w := serveHandler(srv.handleSmartSearch, http.MethodGet, "/api/gallery/smart-search?q=sunset", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	msg := decodeErrorMessage(t, w.Body.Bytes())
	if msg == string(i18n.MsgSmartSearchNotConfigured) {
		t.Errorf("error message was not translated: %q", msg)
	}
	if !strings.Contains(msg, "not configured") {
		t.Errorf("expected a human-readable configuration hint, got %q", msg)
	}
}

func TestHandleSmartSearch_QueryRequired(t *testing.T) {
	srv := newSmartSearchTestServer(t)

	w := serveHandler(srv.handleSmartSearch, http.MethodGet, "/api/gallery/smart-search", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	msg := decodeErrorMessage(t, w.Body.Bytes())
	if msg == "" {
		t.Fatal("expected an error message in the response body")
	}
	if msg == string(i18n.MsgSmartQueryRequired) {
		t.Errorf("error message was not translated: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "query") {
		t.Errorf("expected a human-readable query hint, got %q", msg)
	}
}
