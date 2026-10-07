package geo

import (
	"sync"
	"testing"

	"github.com/flashbacks/api-service/internal/interfaces/dto"

	"github.com/stretchr/testify/assert"
)

func TestClusterStorage_StoreClusters(t *testing.T) {
	cs := NewClusterStorage()

	clusters := []dto.GeoCluster{
		{ID: "cluster_1", ImagePaths: []string{"/a.jpg", "/b.jpg"}},
		{ID: "cluster_2", ImagePaths: []string{"/c.jpg"}},
		{ID: "cluster_3", ImagePaths: []string{}}, // empty paths, should be skipped
	}

	cs.StoreClusters(clusters)

	paths, ok := cs.GetClusterImagePaths("cluster_1")
	assert.True(t, ok)
	assert.Equal(t, []string{"/a.jpg", "/b.jpg"}, paths)

	paths, ok = cs.GetClusterImagePaths("cluster_2")
	assert.True(t, ok)
	assert.Equal(t, []string{"/c.jpg"}, paths)

	// cluster_3 had empty paths, should not be stored
	_, ok = cs.GetClusterImagePaths("cluster_3")
	assert.False(t, ok)
}

func TestClusterStorage_GetClusterImagePaths_Missing(t *testing.T) {
	cs := NewClusterStorage()

	_, ok := cs.GetClusterImagePaths("nonexistent")
	assert.False(t, ok)
}

func TestClusterStorage_Clear(t *testing.T) {
	cs := NewClusterStorage()

	cs.StoreClusters([]dto.GeoCluster{
		{ID: "cluster_1", ImagePaths: []string{"/a.jpg"}},
	})

	cs.Clear()

	_, ok := cs.GetClusterImagePaths("cluster_1")
	assert.False(t, ok)
}

func TestClusterStorage_ConcurrentAccess(t *testing.T) {
	cs := NewClusterStorage()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cs.StoreClusters([]dto.GeoCluster{
				{ID: "concurrent", ImagePaths: []string{"/img.jpg"}},
			})
		}(i)
	}

	wg.Wait()

	// Should not panic - verify readable
	paths, ok := cs.GetClusterImagePaths("concurrent")
	assert.True(t, ok)
	assert.NotNil(t, paths)
}

func TestCalculateClusterRadius_LowZoom(t *testing.T) {
	// At zoom 0, radius should be 10.0
	radius := calculateClusterRadius(1, 0)
	assert.InDelta(t, 10.0, radius, 0.0001)
}

func TestCalculateClusterRadius_HighZoom(t *testing.T) {
	// At zoom 10, radius = 10 / 2^10 = 10 / 1024 ≈ 0.00977
	radius := calculateClusterRadius(1, 10)
	assert.InDelta(t, 0.00977, radius, 0.001)
}

func TestCalculateClusterRadius_ManyPoints(t *testing.T) {
	// More than 10 points scales radius by 1.5
	radiusFew := calculateClusterRadius(5, 0)
	radiusMany := calculateClusterRadius(15, 0)

	assert.Greater(t, radiusMany, radiusFew, "more points should have larger radius")
	assert.InDelta(t, radiusFew*1.5, radiusMany, 0.0001)
}

func TestImageGeoPoint_GetCoordinates(t *testing.T) {
	p := &imageGeoPoint{lat: 55.7558, lng: 37.6173}
	coords := p.GetCoordinates()

	assert.InDelta(t, 55.7558, coords.Lat, 0.0001)
	assert.InDelta(t, 37.6173, coords.Lon, 0.0001)
}

func TestClusterStorage_OverwriteClusters(t *testing.T) {
	cs := NewClusterStorage()

	// Store initial clusters
	cs.StoreClusters([]dto.GeoCluster{
		{ID: "old", ImagePaths: []string{"/old.jpg"}},
	})

	// Overwrite with new
	cs.StoreClusters([]dto.GeoCluster{
		{ID: "new", ImagePaths: []string{"/new.jpg"}},
	})

	// Old cluster should be gone
	_, ok := cs.GetClusterImagePaths("old")
	assert.False(t, ok)

	// New cluster should exist
	paths, ok := cs.GetClusterImagePaths("new")
	assert.True(t, ok)
	assert.Equal(t, []string{"/new.jpg"}, paths)
}
