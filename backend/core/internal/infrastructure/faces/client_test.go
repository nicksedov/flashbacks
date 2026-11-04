package faces

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAnalyzeBatch_SendsEveryImageAndKeepsPositions guards the batch contract:
// every image travels in one request and each response entry stays aligned with
// the image that produced it.
func TestAnalyzeBatch_SendsEveryImageAndKeepsPositions(t *testing.T) {
	var gotPath string
	var gotImages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body struct {
			Images []string `json:"images"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotImages = body.Images
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"faces":[{"x":1,"y":2,"width":3,"height":4,"confidence":0.9,"embedding":[0.1,0.2]}]},
			{"error":"invalid image payload"},
			{"faces":[]}
		]}`))
	}))
	defer server.Close()

	images := [][]byte{[]byte("first"), []byte("second"), []byte("third")}
	results, err := NewClient(server.URL).AnalyzeBatch(context.Background(), images)
	if err != nil {
		t.Fatalf("AnalyzeBatch: %v", err)
	}

	if gotPath != "/faces/analyze-batch" {
		t.Errorf("path = %q, want /faces/analyze-batch", gotPath)
	}
	if len(gotImages) != len(images) {
		t.Fatalf("request carried %d images, want %d", len(gotImages), len(images))
	}
	for i, image := range images {
		if want := base64.StdEncoding.EncodeToString(image); gotImages[i] != want {
			t.Errorf("image %d = %q, want %q", i, gotImages[i], want)
		}
	}

	if len(results) != len(images) {
		t.Fatalf("results = %d, want %d", len(results), len(images))
	}
	if len(results[0].Faces) != 1 || results[0].Faces[0].Width != 3 {
		t.Errorf("entry 0 faces = %+v, want the detected face", results[0].Faces)
	}
	if results[1].Error != "invalid image payload" || len(results[1].Faces) != 0 {
		t.Errorf("entry 1 = %+v, want only the server-side error", results[1])
	}
	if results[2].Error != "" || len(results[2].Faces) != 0 {
		t.Errorf("entry 2 = %+v, want an empty successful entry", results[2])
	}
}

// TestAnalyzeBatch_RequestFailureReturnsError guards that a batch-level failure
// surfaces as an error so the caller can retry the whole batch.
func TestAnalyzeBatch_RequestFailureReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL).AnalyzeBatch(context.Background(), [][]byte{[]byte("one")})
	if err == nil {
		t.Fatal("expected a request-level failure to be returned as an error")
	}
}
