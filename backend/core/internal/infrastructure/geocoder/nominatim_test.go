package geocoder

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNominatimSearch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("expected User-Agent header to be set")
		}
		if q := r.URL.Query().Get("q"); q != "Paris" {
			t.Errorf("expected query 'Paris', got %q", q)
		}
		if format := r.URL.Query().Get("format"); format != "json" {
			t.Errorf("expected format 'json', got %q", format)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"lat": "48.8566", "lon": "2.3522", "display_name": "Paris, France", "type": "city"},
			{"lat": "48.8600", "lon": "2.3400", "display_name": "Paris 1er, Paris", "type": "suburb"}
		]`))
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	results, err := client.Search("Paris")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Lat != 48.8566 {
		t.Errorf("expected lat 48.8566, got %f", results[0].Lat)
	}
	if results[0].Lon != 2.3522 {
		t.Errorf("expected lon 2.3522, got %f", results[0].Lon)
	}
	if results[0].DisplayName != "Paris, France" {
		t.Errorf("expected display name 'Paris, France', got %q", results[0].DisplayName)
	}
	if results[0].Type != "city" {
		t.Errorf("expected type 'city', got %q", results[0].Type)
	}
}

func TestNominatimSearch_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	results, err := client.Search("nonexistentplace12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestNominatimSearch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	_, err := client.Search("test")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestNominatimSearch_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	_, err := client.Search("test")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestNominatimReverseGeocode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reverse" {
			t.Errorf("expected path /reverse, got %q", r.URL.Path)
		}
		if namedetails := r.URL.Query().Get("namedetails"); namedetails != "1" {
			t.Errorf("expected namedetails '1', got %q", namedetails)
		}

		w.Header().Set("Content-Type", "application/json")

		// Respond based on Accept-Language header
		lang := r.Header.Get("Accept-Language")
		if lang == "en" {
			w.Write([]byte(`{
				"display_name": "Moscow, Russia",
				"namedetails": {"name": "Москва", "name:en": "Moscow", "name:ru": "Москва"},
				"address": {"city": "Moscow", "state": "Moscow", "country": "Russia"}
			}`))
		} else {
			w.Write([]byte(`{
				"display_name": "Москва, Россия",
				"namedetails": {"name": "Москва", "name:en": "Moscow", "name:ru": "Москва"},
				"address": {"city": "Москва", "state": "Москва", "country": "Россия"}
			}`))
		}
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	result, err := client.ReverseGeocode(55.7558, 37.6173)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NameLocal != "Москва" {
		t.Errorf("expected NameLocal 'Москва', got %q", result.NameLocal)
	}
	if result.NameEng != "Moscow" {
		t.Errorf("expected NameEng 'Moscow', got %q", result.NameEng)
	}
}

func TestNominatimReverseGeocode_NoAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		lang := r.Header.Get("Accept-Language")
		if lang == "en" {
			w.Write([]byte(`{
				"display_name": "Middle of Nowhere",
				"namedetails": {"name": "Middle of Nowhere"},
				"address": {}
			}`))
		} else {
			w.Write([]byte(`{
				"display_name": "Посреди нигде",
				"namedetails": {"name": "Посреди нигде"},
				"address": {}
			}`))
		}
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	result, err := client.ReverseGeocode(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NameLocal != "" {
		t.Errorf("expected empty NameLocal, got %q", result.NameLocal)
	}
	// Falls back to display_name when no address and no name:en in namedetails
	if result.NameEng != "Middle of Nowhere" {
		t.Errorf("expected NameEng 'Middle of Nowhere', got %q", result.NameEng)
	}
}

func TestNominatimReverseGeocode_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	_, err := client.ReverseGeocode(55.7558, 37.6173)
	if err == nil {
		t.Fatal("expected error for HTTP 503, got nil")
	}
}

func TestNominatimReverseGeocode_FallbackToTown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		lang := r.Header.Get("Accept-Language")
		if lang == "en" {
			w.Write([]byte(`{
				"display_name": "Small Town, Province, Country",
				"namedetails": {"name": "Small Town", "name:en": "Small Town"},
				"address": {"town": "Small Town", "state": "Province", "country": "Country"}
			}`))
		} else {
			w.Write([]byte(`{
				"display_name": "Малый Город, Провинция, Страна",
				"namedetails": {"name": "Малый Город", "name:en": "Small Town"},
				"address": {"town": "Малый Город", "state": "Провинция", "country": "Страна"}
			}`))
		}
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	result, err := client.ReverseGeocode(45.0, 10.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NameLocal != "Малый Город" {
		t.Errorf("expected NameLocal 'Малый Город', got %q", result.NameLocal)
	}
	if result.NameEng != "Small Town" {
		t.Errorf("expected NameEng 'Small Town', got %q", result.NameEng)
	}
}

func TestNominatimReverseGeocode_EnglishCallFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "application/json")
		if lang == "en" {
			// Second (English) call fails
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			// First (local) call succeeds
			w.Write([]byte(`{
				"display_name": "Москва, Россия",
				"namedetails": {"name": "Москва", "name:en": "Moscow"},
				"address": {"city": "Москва", "country": "Россия"}
			}`))
		}
	}))
	defer server.Close()

	client := NewNominatimClient(server.Client(), server.URL)
	result, err := client.ReverseGeocode(55.7558, 37.6173)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NameLocal != "Москва" {
		t.Errorf("expected NameLocal 'Москва', got %q", result.NameLocal)
	}
	// Falls back to namedetails["name:en"] from the local call
	if result.NameEng != "Moscow" {
		t.Errorf("expected NameEng 'Moscow' (fallback), got %q", result.NameEng)
	}
}
