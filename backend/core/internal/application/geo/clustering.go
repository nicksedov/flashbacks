package geo

import (
	"fmt"
	"log"
	"math"
	"sync"

	"github.com/flashbacks/api-service/internal/interfaces/dto"

	goclusterlib "github.com/MadAppGang/gocluster"
	"gorm.io/gorm"
)

// ClusterStorage stores cluster image paths in memory for later retrieval
type ClusterStorage struct {
	mu       sync.RWMutex
	clusters map[string][]string // clusterID -> imagePaths
}

// NewClusterStorage creates a new cluster storage
func NewClusterStorage() *ClusterStorage {
	return &ClusterStorage{
		clusters: make(map[string][]string),
	}
}

// StoreClusters saves cluster image paths to memory
func (s *ClusterStorage) StoreClusters(clusters []dto.GeoCluster) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear old data
	s.clusters = make(map[string][]string)

	for _, c := range clusters {
		if len(c.ImagePaths) > 0 {
			s.clusters[c.ID] = c.ImagePaths
		}
	}

	log.Printf("[geo] Stored %d clusters in memory", len(s.clusters))
}

// GetClusterImagePaths retrieves image paths for a specific cluster
func (s *ClusterStorage) GetClusterImagePaths(clusterID string) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paths, ok := s.clusters[clusterID]
	return paths, ok
}

// Clear removes all stored cluster data
func (s *ClusterStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusters = make(map[string][]string)
}

// ClusterParams holds the viewport parameters for clustering
type ClusterParams struct {
	MinLat, MaxLat, MinLng, MaxLng float64
	Zoom                           int
	ViewportWidth, ViewportHeight  int
}

// imageGeoPoint implements goclusterlib.GeoPoint interface
type imageGeoPoint struct {
	lat, lng float64
	path     string
	index    int
}

func (p *imageGeoPoint) GetCoordinates() goclusterlib.GeoCoordinates {
	return goclusterlib.GeoCoordinates{Lon: p.lng, Lat: p.lat}
}

// ComputeClusters performs server-side clustering using goclusterlib library.
// It uses hierarchical grid-based clustering with KD-tree indexing for fast queries.
func ComputeClusters(db *gorm.DB, params ClusterParams) ([]dto.GeoCluster, int, error) {
	// If bounds are not set, fetch all GPS images
	hasBounds := params.MinLat != 0 || params.MaxLat != 0 || params.MinLng != 0 || params.MaxLng != 0

	// Query all images with GPS coordinates
	type imageWithGPS struct {
		Path         string
		GPSLatitude  float64
		GPSLongitude float64
	}

	var images []imageWithGPS
	query := db.Table("image_files").
		Select("image_files.path, geolocation_caches.gps_latitude, geolocation_caches.gps_longitude").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Joins("INNER JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref")

	if hasBounds {
		minLat, maxLat := params.MinLat, params.MaxLat
		minLng, maxLng := params.MinLng, params.MaxLng
		if minLat > maxLat {
			minLat, maxLat = maxLat, minLat
		}
		if minLng > maxLng {
			minLng, maxLng = maxLng, minLng
		}

		query = query.Where("geolocation_caches.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
			Where("geolocation_caches.gps_longitude BETWEEN ? AND ?", minLng, maxLng)

		log.Printf("[geo] Bounds: lat=[%.4f, %.4f], lng=[%.4f, %.4f]", minLat, maxLat, minLng, maxLng)
	} else {
		log.Printf("[geo] No bounds, querying all GPS images")
	}

	if err := query.Find(&images).Error; err != nil {
		return nil, 0, err
	}

	log.Printf("[geo] Found %d images", len(images))

	totalImages := len(images)
	if totalImages == 0 {
		return []dto.GeoCluster{}, 0, nil
	}

	// Convert to goclusterlib.GeoPoint slice
	points := make([]goclusterlib.GeoPoint, len(images))
	for i, img := range images {
		points[i] = &imageGeoPoint{
			lat:   img.GPSLatitude,
			lng:   img.GPSLongitude,
			path:  img.Path,
			index: i,
		}
	}

	// Build cluster index
	cl := goclusterlib.NewCluster()

	// Viewport-aware point size: adapt clustering density to viewport dimensions
	// Smaller viewports get tighter clusters, larger viewports get more spread
	basePointSize := 40.0
	if params.ViewportWidth > 0 && params.ViewportHeight > 0 {
		minDimension := math.Min(float64(params.ViewportWidth), float64(params.ViewportHeight))
		scaleFactor := minDimension / 800.0 // 800px is baseline
		basePointSize = 40.0 * scaleFactor
	}
	// Clamp point size to reasonable range [20, 80]
	cl.PointSize = int(math.Max(20, math.Min(80, basePointSize)))
	cl.MinZoom = 0
	cl.MaxZoom = 18

	if err := cl.ClusterPoints(points); err != nil {
		return nil, 0, err
	}

	// Use AllClusters (GetClusters has issues with world bounds)
	clusterPoints := cl.AllClusters(params.Zoom)
	log.Printf("[geo] AllClusters returned %d points", len(clusterPoints))

	// Convert to DTO
	clusters := make([]dto.GeoCluster, 0, len(clusterPoints))

	for _, cp := range clusterPoints {
		// IMPORTANT: Coordinates() returns (x, y) from Mercator = (lng, lat)
		// NOT (lat, lng)! The library's Coordinates() returns the reverse Mercator
		// projection which gives us (longitude, latitude).
		y, x := cp.Coordinates() // y is actually longitude, x is latitude
		lat, lng := x, y         // swap to get correct lat/lng

		// Skip individual points at low zoom levels
		if cp.NumPoints == 1 && params.Zoom < 15 {
			continue
		}

		// Generate stable cluster ID based on geographic center + zoom level
		// Round to 4 decimal places (~11m precision) for stability across requests
		clusterID := fmt.Sprintf("cluster_%d_%.4f_%.4f", params.Zoom, lat, lng)

		// Get image paths for storage (not returned to frontend)
		var imagePaths []string
		if cp.NumPoints > 1 {
			// For clusters, query nearby images
			radius := calculateClusterRadius(cp.NumPoints, params.Zoom)
			var paths []string
			db.Table("image_files").
				Select("image_files.path").
				Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
				Joins("INNER JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
				Where("geolocation_caches.gps_latitude BETWEEN ? AND ?", lat-radius, lat+radius).
				Where("geolocation_caches.gps_longitude BETWEEN ? AND ?", lng-radius, lng+radius).
				Limit(500).
				Pluck("path", &paths)
			imagePaths = paths
			log.Printf("[geo] Cluster %s: %d images within %.4f° radius", clusterID, len(paths), radius)
		} else {
			// For single points, get path from original data
			if cp.Id >= 0 && cp.Id < len(images) {
				imagePaths = []string{images[cp.Id].Path}
			}
		}

		clusters = append(clusters, dto.GeoCluster{
			ID:         clusterID,
			Latitude:   lat,
			Longitude:  lng,
			Count:      cp.NumPoints,
			ImagePaths: imagePaths, // Will be cleared before sending to frontend
		})
	}

	log.Printf("[geo] Returning %d clusters", len(clusters))
	return clusters, totalImages, nil
}

// calculateClusterRadius estimates the geographic radius for a cluster based on zoom level
func calculateClusterRadius(numPoints int, zoom int) float64 {
	baseRadius := 10.0 / math.Pow(2, float64(zoom))
	if numPoints > 10 {
		baseRadius *= 1.5
	}
	return baseRadius
}
