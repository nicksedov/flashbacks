package mocks

import "fmt"

// MockGeocoder is a mock implementation of the geocoder for testing.
type MockGeocoder struct {
	Results   map[string]struct{ Country, City string }
	CallCount int
}

// NewMockGeocoder creates a new mock geocoder with predefined test fixtures.
func NewMockGeocoder() *MockGeocoder {
	mg := &MockGeocoder{
		Results: make(map[string]struct{ Country, City string }),
	}
	// Add common test fixtures
	mg.AddResult(55.7558, 37.6173, "Russia", "Moscow")
	mg.AddResult(48.8566, 2.3522, "France", "Paris")
	mg.AddResult(40.7128, -74.0060, "United States", "New York")
	mg.AddResult(51.5074, -0.1278, "United Kingdom", "London")
	mg.AddResult(35.6762, 139.6503, "Japan", "Tokyo")
	mg.AddResult(0.0, 0.0, "", "") // null-island edge case
	return mg
}

// AddResult adds a geocoding result for the given coordinates.
func (m *MockGeocoder) AddResult(lat, lng float64, country, city string) {
	key := formatKey(lat, lng)
	m.Results[key] = struct{ Country, City string }{Country: country, City: city}
}

// ReverseGeocode implements the geocoder interface.
func (m *MockGeocoder) ReverseGeocode(lat, lng float64) (country, city string) {
	m.CallCount++
	key := formatKey(lat, lng)
	if result, ok := m.Results[key]; ok {
		return result.Country, result.City
	}
	return "", ""
}

func formatKey(lat, lng float64) string {
	return fmt.Sprintf("%.4f,%.4f", lat, lng)
}
