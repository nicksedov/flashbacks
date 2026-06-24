package exifclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
)

// HTTPExifClient implements ExifClient by calling the EXIF microservice over HTTP.
type HTTPExifClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPExifClient creates a new HTTP-based EXIF client.
func NewHTTPExifClient(serviceURL string) *HTTPExifClient {
	return &HTTPExifClient{
		baseURL: strings.TrimRight(serviceURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExtractMetadata reads EXIF metadata from an image file via the EXIF service.
func (c *HTTPExifClient) ExtractMetadata(ctx context.Context, filePath string) (*domain.ImageMetadata, error) {
	url := fmt.Sprintf("%s/exif/metadata?path=%s", c.baseURL, filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("EXIF service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("EXIF service returned %d: %s", resp.StatusCode, string(body))
	}

	var metaResp struct {
		Width        int     `json:"width"`
		Height       int     `json:"height"`
		CameraModel  string  `json:"cameraModel"`
		LensModel    string  `json:"lensModel"`
		ISO          int     `json:"iso"`
		Aperture     string  `json:"aperture"`
		ShutterSpeed string  `json:"shutterSpeed"`
		FocalLength  string  `json:"focalLength"`
		DateTaken    *string `json:"dateTaken"`
		Orientation  int     `json:"orientation"`
		ColorSpace   string  `json:"colorSpace"`
		Software     string  `json:"software"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	meta := &domain.ImageMetadata{
		Width:        metaResp.Width,
		Height:       metaResp.Height,
		CameraModel:  metaResp.CameraModel,
		LensModel:    metaResp.LensModel,
		ISO:          metaResp.ISO,
		Aperture:     metaResp.Aperture,
		ShutterSpeed: metaResp.ShutterSpeed,
		FocalLength:  metaResp.FocalLength,
		Orientation:  metaResp.Orientation,
		ColorSpace:   metaResp.ColorSpace,
		Software:     metaResp.Software,
	}

	if metaResp.DateTaken != nil {
		if t, err := time.Parse(time.RFC3339, *metaResp.DateTaken); err == nil {
			meta.DateTaken = &t
		}
	}

	return meta, nil
}

// ExtractGPS reads GPS coordinates from an image file's EXIF via the EXIF service.
func (c *HTTPExifClient) ExtractGPS(ctx context.Context, filePath string) (lat, lng float64, ok bool, err error) {
	url := fmt.Sprintf("%s/exif/metadata?path=%s", c.baseURL, filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, false, fmt.Errorf("EXIF service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, false, nil
	}

	var metaResp struct {
		GPSLatitude  *float64 `json:"gpsLatitude"`
		GPSLongitude *float64 `json:"gpsLongitude"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metaResp); err != nil {
		return 0, 0, false, fmt.Errorf("failed to decode response: %w", err)
	}

	if metaResp.GPSLatitude != nil && metaResp.GPSLongitude != nil {
		return *metaResp.GPSLatitude, *metaResp.GPSLongitude, true, nil
	}

	return 0, 0, false, nil
}

// WriteGPS writes GPS coordinates to an image file's EXIF via the EXIF service.
// backupDir is sent to the EXIF service so it can store a backup of the original file before modification.
func (c *HTTPExifClient) WriteGPS(ctx context.Context, filePath string, lat, lng float64, backupDir string, meta *domain.ImageMetadata) error {
	url := fmt.Sprintf("%s/exif/gps", c.baseURL)

	reqBody := struct {
		Path      string  `json:"path"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		BackupDir string  `json:"backupDir"`
	}{
		Path:      filePath,
		Latitude:  lat,
		Longitude: lng,
		BackupDir: backupDir,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("EXIF service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("EXIF service returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// EnrichMissingMetadata fills empty fields in existing metadata via the EXIF service.
// Since the EXIF service doesn't have a dedicated enrich endpoint, we use ExtractMetadata
// and compare locally.
func (c *HTTPExifClient) EnrichMissingMetadata(ctx context.Context, filePath string, meta *domain.ImageMetadata) (map[string]interface{}, error) {
	// The EXIF service doesn't expose a dedicated enrich endpoint.
	// We extract full metadata and compute the diff locally.
	fileMeta, err := c.ExtractMetadata(ctx, filePath)
	if err != nil {
		return nil, err
	}

	enriched := make(map[string]interface{})

	if meta.CameraModel == "" && fileMeta.CameraModel != "" {
		meta.CameraModel = fileMeta.CameraModel
		enriched["camera_model"] = meta.CameraModel
	}
	if meta.LensModel == "" && fileMeta.LensModel != "" {
		meta.LensModel = fileMeta.LensModel
		enriched["lens_model"] = meta.LensModel
	}
	if meta.ISO == 0 && fileMeta.ISO != 0 {
		meta.ISO = fileMeta.ISO
		enriched["iso"] = meta.ISO
	}
	if meta.Aperture == "" && fileMeta.Aperture != "" {
		meta.Aperture = fileMeta.Aperture
		enriched["aperture"] = meta.Aperture
	}
	if meta.ShutterSpeed == "" && fileMeta.ShutterSpeed != "" {
		meta.ShutterSpeed = fileMeta.ShutterSpeed
		enriched["shutter_speed"] = meta.ShutterSpeed
	}
	if meta.FocalLength == "" && fileMeta.FocalLength != "" {
		meta.FocalLength = fileMeta.FocalLength
		enriched["focal_length"] = meta.FocalLength
	}
	if meta.DateTaken == nil && fileMeta.DateTaken != nil {
		meta.DateTaken = fileMeta.DateTaken
		enriched["date_taken"] = meta.DateTaken
	}
	if meta.Orientation == 0 && fileMeta.Orientation != 0 {
		meta.Orientation = fileMeta.Orientation
		enriched["orientation"] = meta.Orientation
	}
	if meta.ColorSpace == "" && fileMeta.ColorSpace != "" {
		meta.ColorSpace = fileMeta.ColorSpace
		enriched["color_space"] = meta.ColorSpace
	}
	if meta.Software == "" && fileMeta.Software != "" {
		meta.Software = fileMeta.Software
		enriched["software"] = meta.Software
	}

	if len(enriched) == 0 {
		return nil, nil
	}
	return enriched, nil
}

// Health checks the EXIF service health status.
func (c *HTTPExifClient) Health(ctx context.Context) (*domain.ExifHealthStatus, error) {
	url := fmt.Sprintf("%s/exif/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("EXIF service health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EXIF service returned %d", resp.StatusCode)
	}

	var status domain.ExifHealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &status, nil
}
