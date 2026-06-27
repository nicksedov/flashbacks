package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	shareddomain "github.com/flashbacks/shared/domain"
	"github.com/flashbacks/ocr/internal/service"
	"github.com/otiai10/gosseract/v2"
)

// ClassifyHandler handles image classification requests.
type ClassifyHandler struct {
	classifier *service.Classifier
}

// NewClassifyHandler creates a new ClassifyHandler instance.
func NewClassifyHandler() *ClassifyHandler {
	return &ClassifyHandler{
		classifier: service.NewClassifier(),
	}
}

// ErrorResponse represents an error response in JSON format.
type ErrorResponse struct {
	Error string `json:"error"`
}

// parsePageIteratorLevel parses a string level name to gosseract PageIteratorLevel constant.
// Accepts names like "RIL_BLOCK", "RIL_PARA", "RIL_TEXTLINE", "RIL_WORD", "RIL_SYMBOL".
func parsePageIteratorLevel(level string) (gosseract.PageIteratorLevel, error) {
	switch level {
	case "RIL_BLOCK", "block":
		return gosseract.RIL_BLOCK, nil
	case "RIL_PARA", "para":
		return gosseract.RIL_PARA, nil
	case "RIL_TEXTLINE", "textline":
		return gosseract.RIL_TEXTLINE, nil
	case "RIL_WORD", "word":
		return gosseract.RIL_WORD, nil
	case "RIL_SYMBOL", "symbol":
		return gosseract.RIL_SYMBOL, nil
	default:
		return 0, fmt.Errorf("invalid level: %s", level)
	}
}

// Classify processes image classification requests.
// It accepts POST requests with image/jpeg or image/png content type.
// Optional query parameters: confidence_threshold (0-1), min_token_count (positive integer),
// lang (comma-separated language codes, default: "eng+rus"), level (PageIteratorLevel name).
func (h *ClassifyHandler) Classify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{Error: "method not allowed, use POST"}); err != nil {
			fmt.Fprintf(w, `{"error":"method not allowed, use POST"}`)
		}
		return
	}

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{Error: "content-type must be image/jpeg or image/png"}); err != nil {
			fmt.Fprintf(w, `{"error":"content-type must be image/jpeg or image/png"}`)
		}
		return
	}

	// Read image data
	imageData, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{Error: "failed to read image data"}); err != nil {
			fmt.Fprintf(w, `{"error":"failed to read image data"}`)
		}
		return
	}
	defer r.Body.Close()

	if len(imageData) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{Error: "empty image data"}); err != nil {
			fmt.Fprintf(w, `{"error":"empty image data"}`)
		}
		return
	}

	// Parse query parameters with defaults
	decisionRule := service.GetDefaultDecisionRule()

	// Parse lang from URL parameter (default: "eng+rus")
	if lang := r.URL.Query().Get("lang"); lang != "" {
		decisionRule.Language = lang
	}

	// Parse level from URL parameter (default: RIL_WORD)
	if level := r.URL.Query().Get("level"); level != "" {
		if levelInt, err := parsePageIteratorLevel(level); err == nil {
			decisionRule.Level = &levelInt
		}
	}

	// Parse confidence_threshold from URL parameter
	if thresholdStr := r.URL.Query().Get("confidence_threshold"); thresholdStr != "" {
		if val, err := strconv.ParseFloat(thresholdStr, 64); err == nil && val > 0 && val <= 1 {
			decisionRule.MinConfidence = val
		}
	}

	// Parse min_token_count from URL parameter
	if tokenCountStr := r.URL.Query().Get("min_token_count"); tokenCountStr != "" {
		if val, err := strconv.Atoi(tokenCountStr); err == nil && val > 0 {
			decisionRule.MinTokenCount = val
		}
	}

	// Perform classification
	result, err := h.classifier.DetectText(imageData, decisionRule)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(ErrorResponse{Error: "failed to process image"}); err != nil {
			fmt.Fprintf(w, `{"error":"failed to process image"}`)
		}
		return
	}

	// Convert internal result to shared ClassifyResponse for JSON encoding
	response := toSharedClassifyResponse(result)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(w, `{"error":"failed to encode response"}`)
	}
}

// toSharedClassifyResponse converts an internal ClassifierResult to the shared ClassifyResponse.
func toSharedClassifyResponse(result *service.ClassifierResult) shareddomain.ClassifyResponse {
	response := shareddomain.ClassifyResponse{
		MeanConfidence:     result.MeanConfidence,
		WeightedConfidence: result.WeightedConfidence,
		TokenCount:         result.TokenCount,
		Boxes:              make([]shareddomain.BoundingBox, len(result.Boxes)),
		Angle:              result.Angle,
		ScaleFactor:        result.ScaleFactor,
		IsTextDocument:     result.IsTextDocument,
		BoundingBoxWidth:   result.BoundingBoxWidth,
		BoundingBoxHeight:  result.BoundingBoxHeight,
	}
	for i, b := range result.Boxes {
		response.Boxes[i] = shareddomain.BoundingBox{
			X:          b.X,
			Y:          b.Y,
			Width:      b.Width,
			Height:     b.Height,
			Word:       b.Word,
			Confidence: b.Confidence,
		}
	}
	return response
}
