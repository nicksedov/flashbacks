// Package faces provides a typed HTTP client for the faces microservice.
package faces

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FaceBox is a detected face bounding box returned by /faces/detect.
type FaceBox struct {
	X          float32 `json:"x"`
	Y          float32 `json:"y"`
	Width      float32 `json:"width"`
	Height     float32 `json:"height"`
	Confidence float32 `json:"confidence"`
}

// AnalyzedFace is one face returned by /faces/analyze: its box, detection
// confidence and embedding, all computed in a single model pass.
type AnalyzedFace struct {
	FaceBox
	Embedding []float32 `json:"embedding"`
}

// Client is the typed REST interface of the faces microservice.
type Client interface {
	// ServiceURL returns the peer base URL this client currently targets.
	ServiceURL() string
	// Detect runs SCRFD-10G face detection and returns the detected boxes.
	Detect(ctx context.Context, image []byte) ([]FaceBox, error)
	// Analyze detects and embeds every face of an image in a single model pass.
	// It is the call the recognition job uses: re-detecting the whole image once
	// per face costs a full inference each and made group photos exceed the
	// request budget, dropping most of their faces.
	Analyze(ctx context.Context, image []byte) ([]AnalyzedFace, error)
	// Embed runs ArcFace embedding on a full image and returns the 512-d vector
	// of one detected face; box selects which detected face to embed.
	Embed(ctx context.Context, image []byte, box *FaceBox) ([]float32, error)
	// Cluster groups embeddings into people and returns one label per embedding
	// (-1 for noise). soloFaces marks embeddings that are the only face of their
	// photo: such a portrait may anchor a person for a face from a group shot.
	Cluster(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error)
}

type clientImpl struct {
	urlProvider func() string
	httpClient  *http.Client
}

// NewClient creates a faces client with a static service URL.
func NewClient(serviceURL string) Client {
	return NewClientWithProvider(func() string { return serviceURL })
}

// NewClientWithProvider creates a client whose base URL can change at runtime,
// for example as registry discovery updates peer locations.
func NewClientWithProvider(provider func() string) Client {
	if provider == nil {
		provider = func() string { return "" }
	}
	return &clientImpl{
		urlProvider: provider,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *clientImpl) base() string {
	return strings.TrimRight(c.urlProvider(), "/")
}

// ServiceURL returns the base URL currently used to reach the faces service.
func (c *clientImpl) ServiceURL() string {
	return c.base()
}

func (c *clientImpl) post(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("faces service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("faces service returned %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Detect runs face detection over a base64-encoded image.
func (c *clientImpl) Detect(ctx context.Context, image []byte) ([]FaceBox, error) {
	var resp struct {
		Boxes []FaceBox `json:"boxes"`
	}
	body := map[string]string{"image": base64.StdEncoding.EncodeToString(image)}
	if err := c.post(ctx, "/faces/detect", body, &resp); err != nil {
		return nil, err
	}
	return resp.Boxes, nil
}

// Analyze detects and embeds every face of an image in one service call.
func (c *clientImpl) Analyze(ctx context.Context, image []byte) ([]AnalyzedFace, error) {
	var resp struct {
		Faces []AnalyzedFace `json:"faces"`
	}
	body := map[string]string{"image": base64.StdEncoding.EncodeToString(image)}
	if err := c.post(ctx, "/faces/analyze", body, &resp); err != nil {
		return nil, err
	}
	return resp.Faces, nil
}

// Embed runs ArcFace embedding for one detected face of a full image: the
// optional box selects the detected face to embed (highest IoU), otherwise the
// largest detected face is embedded.
func (c *clientImpl) Embed(ctx context.Context, image []byte, box *FaceBox) ([]float32, error) {
	var resp struct {
		Embedding []float32 `json:"embedding"`
	}
	body := map[string]any{"image": base64.StdEncoding.EncodeToString(image)}
	if box != nil {
		body["box"] = box
	}
	if err := c.post(ctx, "/faces/embed", body, &resp); err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}

// Cluster groups a batch of embeddings into people (one label per embedding).
func (c *clientImpl) Cluster(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error) {
	var resp struct {
		Labels []int `json:"labels"`
	}
	body := map[string]any{
		"embeddings":     embeddings,
		"minClusterSize": minClusterSize,
	}
	if len(soloFaces) > 0 {
		body["soloFaces"] = soloFaces
	}
	if err := c.post(ctx, "/faces/cluster", body, &resp); err != nil {
		return nil, err
	}
	return resp.Labels, nil
}
