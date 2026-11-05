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

// BatchResult is one entry of a batch analysis: the faces detected on one
// image, or the reason that image could not be analysed. Entries are
// positionally aligned with the images of the request.
type BatchResult struct {
	Faces []AnalyzedFace
	Error string
}

// Thresholds are the calibrated similarities the faces service applies. Core
// reads them from /faces/health instead of keeping its own copies.
type Thresholds struct {
	ClusterMinSimilarity     float64 `json:"clusterMinSimilarity"`
	RecognitionMinSimilarity float64 `json:"recognitionMinSimilarity"`
	RecognitionMargin        float64 `json:"recognitionMargin"`
	// Assignment of a face (or a new cluster) to an existing person.
	ClusterAssignSimilarity float64 `json:"clusterAssignSimilarity"`
	// Merge of two people whose medoids and cross support agree.
	ClusterMergeSimilarity        float64 `json:"clusterMergeSimilarity"`
	ClusterMergeSupport           int     `json:"clusterMergeSupport"`
	ClusterMergeSupportSimilarity float64 `json:"clusterMergeSupportSimilarity"`
}

// Health is the /faces/health payload.
type Health struct {
	Status             string     `json:"status"`
	ActiveProvider     string     `json:"activeProvider"`
	RequestedProviders []string   `json:"requestedProviders"`
	Thresholds         Thresholds `json:"thresholds"`
}

// FaceLink is one candidate-similarity edge between two detected faces.
type FaceLink struct {
	FaceID      uint    `json:"faceId"`
	CandidateID uint    `json:"candidateId"`
	Similarity  float64 `json:"similarity"`
}

// FaceFacts are the per-face facts the clustering service needs beside the
// candidate links, aligned with the submitted face ids. The service owns the
// semantics (photo diversity, density, quality); core only reports the facts.
type FaceFacts struct {
	PhotoIDs  []uint    `json:"photoIds,omitempty"`
	Widths    []float32 `json:"widths,omitempty"`
	Qualities []float32 `json:"qualities,omitempty"`
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
	// AnalyzeBatch detects and embeds every face of several images in one
	// request; the result is positionally aligned with images. A per-image
	// failure is reported in that entry's Error instead of failing the batch.
	AnalyzeBatch(ctx context.Context, images [][]byte) ([]BatchResult, error)
	// Embed runs ArcFace embedding on a full image and returns the 512-d vector
	// of one detected face; box selects which detected face to embed.
	Embed(ctx context.Context, image []byte, box *FaceBox) ([]float32, error)
	// Cluster groups embeddings into people and returns one label per embedding
	// (-1 for noise). soloFaces marks embeddings that are the only face of their
	// photo: such a portrait may anchor a person for a face from a group shot.
	Cluster(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error)
	// ClusterGraph groups faces from the candidate links of a vector-store kNN
	// query, returning one label per face id (-1 for unassigned). facts carries
	// the aligned photo/width/quality arrays the service uses for photo
	// diversity and density.
	ClusterGraph(ctx context.Context, faceIDs []uint, edges []FaceLink, soloFaces []bool, facts FaceFacts) ([]int, error)
	// Health reports the service liveness, active provider and thresholds.
	Health(ctx context.Context) (Health, error)
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
		// One clustering request can legitimately exceed a minute on a large
		// gallery (the core budget is FACES_CLUSTER_TIMEOUT, 10m); shorter calls
		// stay bounded by their own request contexts.
		httpClient: &http.Client{Timeout: 10 * time.Minute},
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

func (c *clientImpl) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
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

// AnalyzeBatch detects and embeds every face of several images in one request.
// The service answers one entry per submitted image, in request order.
func (c *clientImpl) AnalyzeBatch(ctx context.Context, images [][]byte) ([]BatchResult, error) {
	encoded := make([]string, len(images))
	for i, image := range images {
		encoded[i] = base64.StdEncoding.EncodeToString(image)
	}
	var resp struct {
		Results []struct {
			Faces []AnalyzedFace `json:"faces"`
			Error string         `json:"error"`
		} `json:"results"`
	}
	if err := c.post(ctx, "/faces/analyze-batch", map[string]any{"images": encoded}, &resp); err != nil {
		return nil, err
	}
	results := make([]BatchResult, len(resp.Results))
	for i, entry := range resp.Results {
		results[i] = BatchResult{Faces: entry.Faces, Error: entry.Error}
	}
	return results, nil
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

// ClusterGraph groups faces from candidate links answered by the vector store.
func (c *clientImpl) ClusterGraph(
	ctx context.Context,
	faceIDs []uint,
	edges []FaceLink,
	soloFaces []bool,
	facts FaceFacts,
) ([]int, error) {
	var resp struct {
		Labels []int `json:"labels"`
	}
	body := map[string]any{
		"faceIds": faceIDs,
		"edges":   edges,
	}
	if len(soloFaces) > 0 {
		body["soloFaces"] = soloFaces
	}
	if len(facts.PhotoIDs) > 0 {
		body["photoIds"] = facts.PhotoIDs
	}
	if len(facts.Widths) > 0 {
		body["widths"] = facts.Widths
	}
	if len(facts.Qualities) > 0 {
		body["qualities"] = facts.Qualities
	}
	if err := c.post(ctx, "/faces/cluster-graph", body, &resp); err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// Health reports the service liveness, active execution provider and the
// calibrated thresholds the clustering decision uses.
func (c *clientImpl) Health(ctx context.Context) (Health, error) {
	var health Health
	if err := c.get(ctx, "/faces/health", &health); err != nil {
		return Health{}, err
	}
	return health, nil
}
