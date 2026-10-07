package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/infrastructure/healthcheck"
	"github.com/flashbacks/api-service/internal/infrastructure/retry"
	shareddomain "github.com/flashbacks/shared/domain"
)

// Client is an interface for OCR classifier service
type Client interface {
	// CheckHealth checks if OCR service is available
	CheckHealth(ctx context.Context) error
	// GetStatus returns the current OCR health status
	GetStatus() healthcheck.HealthStatus
	// StartHealthCheck starts the periodic health check in background
	StartHealthCheck(intervalSeconds int)
	// StopHealthCheck stops the periodic health check
	StopHealthCheck()
	// Classify sends an image to the OCR classifier and returns results
	Classify(ctx context.Context, image io.Reader, contentType string, params *ClassifyParams) (*shareddomain.ClassifyResponse, error)
}

// ClassifyParams holds query parameters for the classify endpoint
type ClassifyParams struct {
	ConfidenceThreshold float32
	Level               string
	MinTokenCount       int
	Lang                string
}

// DefaultClassifyParams returns default parameters matching openapi.yaml spec
func DefaultClassifyParams() *ClassifyParams {
	return &ClassifyParams{
		ConfidenceThreshold: 0.65,
		Level:               "RIL_TEXTLINE",
		MinTokenCount:       40,
		Lang:                "eng+rus",
	}
}

type clientImpl struct {
	baseURL    string
	httpClient *http.Client
	checker    *healthcheck.PeriodicChecker
}

// NewClient creates a new OCR client with unified healthcheck.
func NewClient(serviceURL string) Client {
	baseURL := fmt.Sprintf("%s/ocr/api", strings.TrimRight(serviceURL, "/"))
	c := &clientImpl{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	c.checker = healthcheck.NewPeriodicChecker(c, 30*time.Second, 2*time.Second)
	return c
}

// Check implements healthcheck.Checker by delegating to CheckHealth.
func (c *clientImpl) Check(ctx context.Context) error {
	return c.CheckHealth(ctx)
}

// CheckHealth checks if OCR service is available.
func (c *clientImpl) CheckHealth(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result["status"] != "ok" {
		return fmt.Errorf("health check returned non-OK status")
	}

	return nil
}

// GetStatus returns the current OCR health status.
func (c *clientImpl) GetStatus() healthcheck.HealthStatus {
	return c.checker.GetStatus()
}

// StartHealthCheck starts the periodic health check in background.
func (c *clientImpl) StartHealthCheck(intervalSeconds int) {
	c.checker.Start(time.Duration(intervalSeconds) * time.Second)
}

// StopHealthCheck stops the periodic health check.
func (c *clientImpl) StopHealthCheck() {
	c.checker.Stop()
}

// Classify sends an image to the OCR classifier and returns results.
// Uses unified retry with exponential backoff.
func (c *clientImpl) Classify(ctx context.Context, image io.Reader, contentType string, params *ClassifyParams) (*shareddomain.ClassifyResponse, error) {
	imgData, err := io.ReadAll(image)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	if contentType == "" {
		contentType = "image/jpeg"
	}

	queryParams := url.Values{}
	if params != nil {
		queryParams.Set("confidence_threshold", fmt.Sprintf("%.2f", params.ConfidenceThreshold))
		if params.Level != "" {
			queryParams.Set("level", params.Level)
		}
		if params.MinTokenCount > 0 {
			queryParams.Set("min_token_count", fmt.Sprintf("%d", params.MinTokenCount))
		}
		if params.Lang != "" {
			queryParams.Set("lang", params.Lang)
		}
	} else {
		defaults := DefaultClassifyParams()
		queryParams.Set("confidence_threshold", fmt.Sprintf("%.2f", defaults.ConfidenceThreshold))
		queryParams.Set("level", defaults.Level)
		queryParams.Set("min_token_count", fmt.Sprintf("%d", defaults.MinTokenCount))
		queryParams.Set("lang", defaults.Lang)
	}

	classifyURL := fmt.Sprintf("%s/v1/classify?%s", c.baseURL, queryParams.Encode())

	cfg := retry.Config{
		MaxAttempts: 3,
		Delay:       2 * time.Second,
		MaxDelay:    30 * time.Second,
		Backoff:     retry.BackoffExponential,
	}

	return retry.WithRetry(ctx, cfg, func(ctx context.Context) (*shareddomain.ClassifyResponse, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, classifyURL, bytes.NewReader(imgData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)

		client := &http.Client{Timeout: 180 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("OCR API error (status %d): %s", resp.StatusCode, string(body))
		}

		var result shareddomain.ClassifyResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to parse OCR response: %w", err)
		}

		return &result, nil
	})
}
