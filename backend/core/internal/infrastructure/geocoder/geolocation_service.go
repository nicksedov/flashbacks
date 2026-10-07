package geocoder

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/flashbacks/api-service/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GeolocationService resolves GPS coordinates to location names via Nominatim,
// with a database-backed cache (geolocation_caches table).
// It rate-limits Nominatim calls to at most 1 request per second.
type GeolocationService struct {
	db        *gorm.DB
	nominatim *NominatimClient
	mu        sync.Mutex
	lastCall  time.Time
}

// NewGeolocationService creates a GeolocationService backed by the given DB and Nominatim client.
func NewGeolocationService(db *gorm.DB, nominatim *NominatimClient) *GeolocationService {
	return &GeolocationService{
		db:        db,
		nominatim: nominatim,
	}
}

// ResolveGeolocation returns the GeolocationCache entry for the given coordinates.
// It first checks the cache; on a miss, it calls Nominatim (rate-limited) and inserts the result.
func (gs *GeolocationService) ResolveGeolocation(lat, lng float64) (*domain.GeolocationCache, error) {
	// Check cache first (no lock needed for reads)
	var entry domain.GeolocationCache
	if err := gs.db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("geolocation cache lookup failed: %w", err)
		}
		// Record not found — fall through to Nominatim
	} else {
		return &entry, nil
	}

	// Cache miss: acquire mutex and rate-limit
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Double-check after acquiring lock (another goroutine may have inserted it)
	if err := gs.db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("geolocation cache double-check failed: %w", err)
		}
		// Still not found — proceed to Nominatim
	} else {
		return &entry, nil
	}

	// Rate-limit: ensure at least 1 second between Nominatim calls
	elapsed := time.Since(gs.lastCall)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	// Call Nominatim reverse geocode
	result, err := gs.nominatim.ReverseGeocode(lat, lng)
	gs.lastCall = time.Now()
	if err != nil {
		log.Printf("GeolocationService: Nominatim reverse geocode failed for (%f, %f): %v", lat, lng, err)
		return nil, fmt.Errorf("nominatim reverse geocode failed: %w", err)
	}

	// Insert into cache (handle unique conflict)
	entry = domain.GeolocationCache{
		GPSLatitude:  lat,
		GPSLongitude: lng,
		NameLocal:    result.NameLocal,
		NameEng:      result.NameEng,
	}

	// Use ON CONFLICT DO NOTHING + re-query to handle race conditions
	if err := gs.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
		// If insert failed due to conflict, re-query
		if err := gs.db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
			return nil, fmt.Errorf("failed to query geolocation cache after conflict: %w", err)
		}
		return &entry, nil
	}

	// If DoNothing was triggered, GORM may not populate entry.ID; re-query
	if entry.ID == 0 {
		if err := gs.db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
			return nil, fmt.Errorf("failed to re-query geolocation cache: %w", err)
		}
	}

	return &entry, nil
}
