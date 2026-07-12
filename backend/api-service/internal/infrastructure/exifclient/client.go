package exifclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// --- Existing methods ---

// ExtractMetadata reads EXIF metadata from an image file via the EXIF service.
func (c *HTTPExifClient) ExtractMetadata(ctx context.Context, filePath string) (*domain.ImageMetadata, error) {
	u := fmt.Sprintf("%s/exif/metadata?path=%s", c.baseURL, url.PathEscape(filePath))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
	u := fmt.Sprintf("%s/exif/metadata?path=%s", c.baseURL, url.PathEscape(filePath))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
func (c *HTTPExifClient) WriteGPS(ctx context.Context, filePath string, lat, lng float64, backupDir string, meta *domain.ImageMetadata) error {
	u := fmt.Sprintf("%s/exif/gps", c.baseURL)

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
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, strings.NewReader(string(bodyBytes)))
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
func (c *HTTPExifClient) EnrichMissingMetadata(ctx context.Context, filePath string, meta *domain.ImageMetadata) (map[string]interface{}, error) {
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
	u := fmt.Sprintf("%s/exif/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

// --- Phase 3: New metadata CRUD methods ---

// GetMetadataByImageID retrieves metadata from the EXIF service by image file ID.
func (c *HTTPExifClient) GetMetadataByImageID(ctx context.Context, imageFileID uint) (*domain.ImageMetadata, error) {
	u := fmt.Sprintf("%s/exif/metadata?imageFileId=%d", c.baseURL, imageFileID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
		ImageFileID:  imageFileID,
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

// UpsertMetadata creates or updates metadata via the EXIF service.
func (c *HTTPExifClient) UpsertMetadata(ctx context.Context, meta *domain.ImageMetadata) (*domain.ImageMetadata, error) {
	u := fmt.Sprintf("%s/exif/metadata", c.baseURL)

	var dateTakenStr *string
	if meta.DateTaken != nil {
		formatted := meta.DateTaken.Format(time.RFC3339)
		dateTakenStr = &formatted
	}

	reqBody := struct {
		ImageFileID    uint    `json:"imageFileId"`
		Width          int     `json:"width"`
		Height         int     `json:"height"`
		CameraModel    string  `json:"cameraModel"`
		LensModel      string  `json:"lensModel"`
		ISO            int     `json:"iso"`
		Aperture       string  `json:"aperture"`
		ShutterSpeed   string  `json:"shutterSpeed"`
		FocalLength    string  `json:"focalLength"`
		DateTaken      *string `json:"dateTaken"`
		Orientation    int     `json:"orientation"`
		ColorSpace     string  `json:"colorSpace"`
		Software       string  `json:"software"`
		GeolocationRef *uint   `json:"geolocationRef"`
	}{
		ImageFileID:    meta.ImageFileID,
		Width:          meta.Width,
		Height:         meta.Height,
		CameraModel:    meta.CameraModel,
		LensModel:      meta.LensModel,
		ISO:            meta.ISO,
		Aperture:       meta.Aperture,
		ShutterSpeed:   meta.ShutterSpeed,
		FocalLength:    meta.FocalLength,
		DateTaken:      dateTakenStr,
		Orientation:    meta.Orientation,
		ColorSpace:     meta.ColorSpace,
		Software:       meta.Software,
		GeolocationRef: meta.GeolocationRef,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("EXIF service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("EXIF service returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID             uint    `json:"id"`
		GeolocationRef *uint   `json:"geolocationRef"`
		DateTaken      *string `json:"dateTaken"`
		ImageFileID    uint    `json:"imageFileId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	meta.ID = result.ID
	meta.GeolocationRef = result.GeolocationRef
	if result.DateTaken != nil {
		if t, err := time.Parse(time.RFC3339, *result.DateTaken); err == nil {
			meta.DateTaken = &t
		}
	}

	return meta, nil
}

// DeleteMetadata deletes metadata for an image via the EXIF service.
func (c *HTTPExifClient) DeleteMetadata(ctx context.Context, imageFileID uint) error {
	u := fmt.Sprintf("%s/exif/metadata/%d", c.baseURL, imageFileID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

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

// GetMetadataBatch retrieves metadata for multiple image IDs via the EXIF service.
func (c *HTTPExifClient) GetMetadataBatch(ctx context.Context, imageFileIDs []uint) (map[uint]*domain.ImageMetadata, error) {
	if len(imageFileIDs) == 0 {
		return map[uint]*domain.ImageMetadata{}, nil
	}

	idStrs := make([]string, len(imageFileIDs))
	for i, id := range imageFileIDs {
		idStrs[i] = strconv.FormatUint(uint64(id), 10)
	}
	u := fmt.Sprintf("%s/exif/metadata/batch?ids=%s", c.baseURL, strings.Join(idStrs, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

	var batchResp struct {
		Metadata []struct {
			ImageFileID  uint    `json:"imageFileId"`
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
		} `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make(map[uint]*domain.ImageMetadata, len(batchResp.Metadata))
	for _, item := range batchResp.Metadata {
		meta := &domain.ImageMetadata{
			ImageFileID:  item.ImageFileID,
			Width:        item.Width,
			Height:       item.Height,
			CameraModel:  item.CameraModel,
			LensModel:    item.LensModel,
			ISO:          item.ISO,
			Aperture:     item.Aperture,
			ShutterSpeed: item.ShutterSpeed,
			FocalLength:  item.FocalLength,
			Orientation:  item.Orientation,
			ColorSpace:   item.ColorSpace,
			Software:     item.Software,
		}
		if item.DateTaken != nil {
			if t, err := time.Parse(time.RFC3339, *item.DateTaken); err == nil {
				meta.DateTaken = &t
			}
		}
		result[meta.ImageFileID] = meta
	}
	return result, nil
}

// GetCalendarItems fetches calendar gallery items via the EXIF service.
func (c *HTTPExifClient) GetCalendarItems(ctx context.Context, params domain.CalendarParams) (*domain.CalendarResult, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/exif/metadata/calendar", c.baseURL))
	q := u.Query()
	if params.StartDate != nil {
		q.Set("startDate", params.StartDate.Format("2006-01-02"))
	}
	if params.EndDate != nil {
		q.Set("endDate", params.EndDate.Format("2006-01-02"))
	}
	if params.Cursor != nil && params.CursorID != nil {
		raw := fmt.Sprintf("%s|%d", params.Cursor.Format(time.RFC3339), *params.CursorID)
		q.Set("cursor", base64.StdEncoding.EncodeToString([]byte(raw)))
	}
	if params.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(params.PageSize))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
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

	var calResp struct {
		Items []struct {
			ImageFileID    uint    `json:"imageFileId"`
			DateTaken      string  `json:"dateTaken"`
			GeolocationRef *uint   `json:"geolocationRef"`
			GPSLatitude    float64 `json:"gpsLatitude"`
			GPSLongitude   float64 `json:"gpsLongitude"`
			NameLocal      string  `json:"nameLocal"`
			NameEng        string  `json:"nameEng"`
		} `json:"items"`
		NextCursor    string `json:"nextCursor"`
		MinDate       string `json:"minDate"`
		MaxDate       string `json:"maxDate"`
		TotalWithDate int64  `json:"totalWithDate"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&calResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &domain.CalendarResult{
		NextCursor:    calResp.NextCursor,
		MinDate:       calResp.MinDate,
		MaxDate:       calResp.MaxDate,
		TotalWithDate: calResp.TotalWithDate,
	}
	for _, item := range calResp.Items {
		result.Items = append(result.Items, domain.CalendarItem{
			ImageFileID:    item.ImageFileID,
			DateTaken:      item.DateTaken,
			GeolocationRef: item.GeolocationRef,
			GPSLatitude:    item.GPSLatitude,
			GPSLongitude:   item.GPSLongitude,
			NameLocal:      item.NameLocal,
			NameEng:        item.NameEng,
		})
	}
	return result, nil
}

// GetGeoPoints fetches GPS points for clustering via the EXIF service.
func (c *HTTPExifClient) GetGeoPoints(ctx context.Context, bounds domain.GeoBounds) ([]domain.GeoPoint, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/exif/metadata/geo-points", c.baseURL))
	q := u.Query()
	if bounds.MinLat != 0 || bounds.MaxLat != 0 {
		q.Set("minLat", fmt.Sprintf("%f", bounds.MinLat))
		q.Set("maxLat", fmt.Sprintf("%f", bounds.MaxLat))
		q.Set("minLng", fmt.Sprintf("%f", bounds.MinLng))
		q.Set("maxLng", fmt.Sprintf("%f", bounds.MaxLng))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
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

	var geoResp struct {
		Points []struct {
			ImageFileID  uint    `json:"imageFileId"`
			GPSLatitude  float64 `json:"gpsLatitude"`
			GPSLongitude float64 `json:"gpsLongitude"`
			NameLocal    string  `json:"nameLocal"`
			NameEng      string  `json:"nameEng"`
		} `json:"points"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	points := make([]domain.GeoPoint, 0, len(geoResp.Points))
	for _, p := range geoResp.Points {
		points = append(points, domain.GeoPoint{
			ImageFileID:  p.ImageFileID,
			GPSLatitude:  p.GPSLatitude,
			GPSLongitude: p.GPSLongitude,
			NameLocal:    p.NameLocal,
			NameEng:      p.NameEng,
		})
	}
	return points, nil
}

// GetMissingImages fetches images missing EXIF data via the EXIF service.
func (c *HTTPExifClient) GetMissingImages(ctx context.Context, page, pageSize int) (*domain.MissingImagesResult, error) {
	u := fmt.Sprintf("%s/exif/missing?page=%d&pageSize=%d", c.baseURL, page, pageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

	var missResp struct {
		Images []struct {
			ImageFileID uint   `json:"imageFileId"`
			Path        string `json:"path"`
			MissingDate bool   `json:"missingDate"`
			MissingGPS  bool   `json:"missingGps"`
		} `json:"images"`
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&missResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &domain.MissingImagesResult{
		Total:    missResp.Total,
		Page:     missResp.Page,
		PageSize: missResp.PageSize,
	}
	for _, img := range missResp.Images {
		result.Images = append(result.Images, domain.MissingImageItem{
			ImageFileID: img.ImageFileID,
			Path:        img.Path,
			MissingDate: img.MissingDate,
			MissingGPS:  img.MissingGPS,
		})
	}
	return result, nil
}

// GetLocationCandidates fetches location candidates via the EXIF service.
func (c *HTTPExifClient) GetLocationCandidates(ctx context.Context, date string) ([]domain.LocationCandidate, error) {
	u := fmt.Sprintf("%s/exif/location-candidates?date=%s", c.baseURL, url.QueryEscape(date))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

	var locResp struct {
		Candidates []struct {
			Lat        float64 `json:"lat"`
			Lng        float64 `json:"lng"`
			NameLocal  string  `json:"nameLocal"`
			NameEng    string  `json:"nameEng"`
			PhotoCount int     `json:"photoCount"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&locResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	candidates := make([]domain.LocationCandidate, 0, len(locResp.Candidates))
	for _, c := range locResp.Candidates {
		candidates = append(candidates, domain.LocationCandidate{
			Lat:        c.Lat,
			Lng:        c.Lng,
			NameLocal:  c.NameLocal,
			NameEng:    c.NameEng,
			PhotoCount: c.PhotoCount,
		})
	}
	return candidates, nil
}

// ResolveGeolocation resolves GPS coordinates to location names via the EXIF service.
func (c *HTTPExifClient) ResolveGeolocation(ctx context.Context, lat, lng float64) (*domain.GeolocationCache, error) {
	u := fmt.Sprintf("%s/exif/geolocation?lat=%f&lng=%f", c.baseURL, lat, lng)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

	var geoResp struct {
		ID           uint    `json:"id"`
		GPSLatitude  float64 `json:"gpsLatitude"`
		GPSLongitude float64 `json:"gpsLongitude"`
		NameLocal    string  `json:"nameLocal"`
		NameEng      string  `json:"nameEng"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &domain.GeolocationCache{
		ID:           geoResp.ID,
		GPSLatitude:  geoResp.GPSLatitude,
		GPSLongitude: geoResp.GPSLongitude,
		NameLocal:    geoResp.NameLocal,
		NameEng:      geoResp.NameEng,
	}, nil
}

// ExtractPreview fetches an embedded preview/thumbnail image from a file
// via the EXIF service. Used as a fallback when Go's image.Decode cannot
// handle large/panoramic JPEGs. Returns raw JPEG bytes.
func (c *HTTPExifClient) ExtractPreview(ctx context.Context, filePath string) ([]byte, error) {
	u := fmt.Sprintf("%s/exif/preview?path=%s", c.baseURL, url.PathEscape(filePath))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("EXIF service preview request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("EXIF service returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}
