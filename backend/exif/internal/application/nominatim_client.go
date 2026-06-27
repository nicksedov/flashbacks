package application

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// NominatimResult represents a single result from the Nominatim geocoding API.
type NominatimResult struct {
	Lat         float64
	Lon         float64
	DisplayName string
	Type        string
}

// NominatimClient provides forward and reverse geocoding via Nominatim (OpenStreetMap) REST API.
type NominatimClient struct {
	httpClient *http.Client
	baseURL    string
}

// nominatimJSONResult is the raw JSON structure returned by Nominatim.
type nominatimJSONResult struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

// NewNominatimClient creates a NominatimClient with sensible defaults.
func NewNominatimClient(httpClient *http.Client, baseURL string) *NominatimClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://nominatim.openstreetmap.org"
	}
	return &NominatimClient{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// ReverseGeocodeResult holds the bilingual location names from a reverse geocode call.
type ReverseGeocodeResult struct {
	NameLocal string // Local-language place name
	NameEng   string // English display name
}

// nominatimReverseJSON is the raw JSON structure returned by Nominatim /reverse.
type nominatimReverseJSON struct {
	DisplayName string            `json:"display_name"`
	NameDetails map[string]string `json:"namedetails"`
	Address     struct {
		City         string `json:"city"`
		Town         string `json:"town"`
		Village      string `json:"village"`
		State        string `json:"state"`
		Country      string `json:"country"`
		Hamlet       string `json:"hamlet"`
		Municipality string `json:"municipality"`
	} `json:"address"`
}

// ReverseGeocode performs reverse geocoding for the given coordinates.
func (n *NominatimClient) ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error) {
	// First call: local language (no accept-language)
	localRaw, err := n.reverseCall(lat, lng, "")
	if err != nil {
		return nil, err
	}

	nameLocal := extractPlaceName(localRaw)

	// Brief pause between Nominatim requests to be polite
	time.Sleep(300 * time.Millisecond)

	// Second call: English language
	engRaw, err := n.reverseCall(lat, lng, "en")
	if err != nil {
		nameEng := localRaw.NameDetails["name:en"]
		if nameEng == "" {
			nameEng = localRaw.DisplayName
		}
		return &ReverseGeocodeResult{
			NameLocal: nameLocal,
			NameEng:   nameEng,
		}, nil
	}

	nameEng := extractPlaceName(engRaw)
	if nameEng == "" {
		nameEng = engRaw.NameDetails["name:en"]
	}
	if nameEng == "" {
		nameEng = engRaw.DisplayName
	}

	return &ReverseGeocodeResult{
		NameLocal: nameLocal,
		NameEng:   nameEng,
	}, nil
}

// reverseCall performs a single Nominatim /reverse request.
func (n *NominatimClient) reverseCall(lat, lng float64, acceptLang string) (*nominatimReverseJSON, error) {
	reverseURL := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json&zoom=10&addressdetails=1&namedetails=1",
		n.baseURL, lat, lng)

	req, err := http.NewRequest(http.MethodGet, reverseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create reverse request: %w", err)
	}
	req.Header.Set("User-Agent", "Flashbacks/1.0 (https://github.com/nicksedov/image-toolkit)")
	if acceptLang != "" {
		req.Header.Set("Accept-Language", acceptLang)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim reverse request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim reverse returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read nominatim reverse body: %w", err)
	}

	var raw nominatimReverseJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode nominatim reverse response: %w", err)
	}

	return &raw, nil
}

// extractPlaceName builds a place name from address details.
func extractPlaceName(raw *nominatimReverseJSON) string {
	if raw.Address.City != "" {
		return raw.Address.City
	}
	if raw.Address.Town != "" {
		return raw.Address.Town
	}
	if raw.Address.Village != "" {
		return raw.Address.Village
	}
	if raw.Address.Hamlet != "" {
		return raw.Address.Hamlet
	}
	if raw.Address.Municipality != "" {
		return raw.Address.Municipality
	}
	return raw.Address.State
}

// Search performs a forward geocoding search and returns up to 10 results.
func (n *NominatimClient) Search(query string) ([]NominatimResult, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&format=json&limit=10",
		n.baseURL, url.QueryEscape(query))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Flashbacks/1.0 (https://github.com/nicksedov/image-toolkit)")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	var rawResults []nominatimJSONResult
	if err := json.NewDecoder(resp.Body).Decode(&rawResults); err != nil {
		return nil, fmt.Errorf("failed to decode nominatim response: %w", err)
	}

	results := make([]NominatimResult, 0, len(rawResults))
	for _, raw := range rawResults {
		lat, err := strconv.ParseFloat(raw.Lat, 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(raw.Lon, 64)
		if err != nil {
			continue
		}
		results = append(results, NominatimResult{
			Lat:         lat,
			Lon:         lon,
			DisplayName: raw.DisplayName,
			Type:        raw.Type,
		})
	}

	return results, nil
}
