package application

import (
	"errors"
	"fmt"
	"time"

	"exif/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- Query result types ---

// MissingImageRow represents an image missing EXIF data.
type MissingImageRow struct {
	ImageFileID uint
	MissingDate bool
	MissingGPS  bool
}

// CalendarRow represents a calendar gallery item with joined GPS data.
type CalendarRow struct {
	domain.ImageMetadata
	GPSLatitude  float64
	GPSLongitude float64
	NameLocal    string
	NameEng      string
}

// GeoPointRow represents a GPS point for clustering.
type GeoPointRow struct {
	ImageFileID  uint
	GPSLatitude  float64
	GPSLongitude float64
	NameLocal    string
	NameEng      string
}

// MetadataRepo provides CRUD operations for image_metadata and geolocation_caches tables.
type MetadataRepo struct {
	db *gorm.DB
}

// NewMetadataRepo creates a new MetadataRepo.
func NewMetadataRepo(db *gorm.DB) *MetadataRepo {
	return &MetadataRepo{db: db}
}

// --- ImageMetadata operations ---

// GetByImageID retrieves metadata for a specific image file ID.
func (r *MetadataRepo) GetByImageID(imageFileID uint) (*domain.ImageMetadata, error) {
	var meta domain.ImageMetadata
	if err := r.db.Where("image_file_id = ?", imageFileID).First(&meta).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query image_metadata: %w", err)
	}
	return &meta, nil
}

// Upsert inserts or updates metadata for a given image_file_id.
func (r *MetadataRepo) Upsert(meta *domain.ImageMetadata) (*domain.ImageMetadata, error) {
	result := &domain.ImageMetadata{}
	if err := r.db.Where("image_file_id = ?", meta.ImageFileID).Assign(meta).FirstOrCreate(result).Error; err != nil {
		return nil, fmt.Errorf("failed to upsert image_metadata: %w", err)
	}
	// Re-query to get the fully populated record
	if err := r.db.Where("image_file_id = ?", meta.ImageFileID).First(result).Error; err != nil {
		return nil, fmt.Errorf("failed to re-query image_metadata after upsert: %w", err)
	}
	return result, nil
}

// DeleteByImageID deletes metadata for a specific image file ID.
func (r *MetadataRepo) DeleteByImageID(imageFileID uint) error {
	if err := r.db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageMetadata{}).Error; err != nil {
		return fmt.Errorf("failed to delete image_metadata: %w", err)
	}
	return nil
}

// GetBatch retrieves metadata for multiple image file IDs.
func (r *MetadataRepo) GetBatch(imageFileIDs []uint) (map[uint]*domain.ImageMetadata, error) {
	var rows []domain.ImageMetadata
	if err := r.db.Where("image_file_id IN ?", imageFileIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to batch query image_metadata: %w", err)
	}
	result := make(map[uint]*domain.ImageMetadata, len(rows))
	for i := range rows {
		result[rows[i].ImageFileID] = &rows[i]
	}
	return result, nil
}

// GetMissingImages returns images missing date_taken or geolocation_ref, with pagination.
func (r *MetadataRepo) GetMissingImages(page, pageSize int) ([]MissingImageRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	type missingRow struct {
		ImageFileID uint
		MissingDate bool `gorm:"column:missing_date"`
		MissingGPS  bool `gorm:"column:missing_gps"`
	}

	var total int64
	if err := r.db.Model(&domain.ImageMetadata{}).
		Where("date_taken IS NULL OR geolocation_ref IS NULL").
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count missing images: %w", err)
	}

	var rows []missingRow
	if err := r.db.Model(&domain.ImageMetadata{}).
		Select("image_file_id, date_taken IS NULL as missing_date, geolocation_ref IS NULL as missing_gps").
		Where("date_taken IS NULL OR geolocation_ref IS NULL").
		Order("image_file_id ASC").
		Offset(offset).Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query missing images: %w", err)
	}

	results := make([]MissingImageRow, len(rows))
	for i := range rows {
		results[i] = MissingImageRow{
			ImageFileID: rows[i].ImageFileID,
			MissingDate: rows[i].MissingDate,
			MissingGPS:  rows[i].MissingGPS,
		}
	}
	return results, total, nil
}

// GetCalendarItems returns a flat list of metadata items with GPS data for the calendar gallery.
func (r *MetadataRepo) GetCalendarItems(startDate, endDate *time.Time, cursor *time.Time, cursorID *uint, pageSize int) ([]CalendarRow, *time.Time, *uint, error) {
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}

	type calendarRow struct {
		domain.ImageMetadata
		GPSLatitude  float64 `gorm:"column:gps_latitude"`
		GPSLongitude float64 `gorm:"column:gps_longitude"`
		NameLocal    string  `gorm:"column:name_local"`
		NameEng      string  `gorm:"column:name_eng"`
	}

	query := r.db.Table("image_metadata").
		Select(`image_metadata.*, 
			COALESCE(geolocation_caches.gps_latitude, 0) as gps_latitude,
			COALESCE(geolocation_caches.gps_longitude, 0) as gps_longitude,
			COALESCE(geolocation_caches.name_local, '') as name_local,
			COALESCE(geolocation_caches.name_eng, '') as name_eng`).
		Joins("LEFT JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("image_metadata.date_taken IS NOT NULL")

	if startDate != nil {
		query = query.Where("image_metadata.date_taken >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("image_metadata.date_taken < ?", *endDate)
	}

	// Cursor pagination: (date_taken, image_file_id)
	if cursor != nil && cursorID != nil {
		query = query.Where("(image_metadata.date_taken, image_metadata.image_file_id) > (?, ?)", *cursor, *cursorID)
	}

	var rows []calendarRow
	if err := query.Order("image_metadata.date_taken ASC, image_metadata.image_file_id ASC").
		Limit(pageSize + 1).
		Find(&rows).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query calendar items: %w", err)
	}

	// Check if there are more results
	var nextCursor *time.Time
	var nextCursorID *uint
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		lastRow := rows[len(rows)-1]
		nextCursor = lastRow.DateTaken
		nextCursorID = &lastRow.ImageFileID
	}

	results := make([]CalendarRow, len(rows))
	for i := range rows {
		results[i] = CalendarRow{
			ImageMetadata: rows[i].ImageMetadata,
			GPSLatitude:   rows[i].GPSLatitude,
			GPSLongitude:  rows[i].GPSLongitude,
			NameLocal:     rows[i].NameLocal,
			NameEng:       rows[i].NameEng,
		}
	}
	return results, nextCursor, nextCursorID, nil
}

// GetCalendarDateRange returns the min and max date_taken values.
func (r *MetadataRepo) GetCalendarDateRange() (*time.Time, *time.Time, error) {
	type dateRange struct {
		MinDate *time.Time
		MaxDate *time.Time
	}
	var dr dateRange
	if err := r.db.Model(&domain.ImageMetadata{}).
		Select("MIN(date_taken) as min_date, MAX(date_taken) as max_date").
		Where("date_taken IS NOT NULL").
		Find(&dr).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query calendar date range: %w", err)
	}
	return dr.MinDate, dr.MaxDate, nil
}

// GetCalendarDayCount returns the total number of images with a non-null date_taken.
func (r *MetadataRepo) GetCalendarDayCount() (int64, error) {
	var count int64
	if err := r.db.Model(&domain.ImageMetadata{}).
		Where("date_taken IS NOT NULL").
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count dated images: %w", err)
	}
	return count, nil
}

// GetGeoPoints returns GPS points within the given bounding box for clustering.
func (r *MetadataRepo) GetGeoPoints(minLat, maxLat, minLng, maxLng float64) ([]GeoPointRow, error) {
	type geoRow struct {
		ImageFileID  uint    `gorm:"column:image_file_id"`
		GPSLatitude  float64 `gorm:"column:gps_latitude"`
		GPSLongitude float64 `gorm:"column:gps_longitude"`
		NameLocal    string  `gorm:"column:name_local"`
		NameEng      string  `gorm:"column:name_eng"`
	}

	query := r.db.Table("image_metadata").
		Select("image_metadata.image_file_id, geolocation_caches.gps_latitude, geolocation_caches.gps_longitude, geolocation_caches.name_local, geolocation_caches.name_eng").
		Joins("JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("image_metadata.geolocation_ref IS NOT NULL")

	if minLat != 0 || maxLat != 0 || minLng != 0 || maxLng != 0 {
		query = query.Where("geolocation_caches.gps_latitude >= ? AND geolocation_caches.gps_latitude <= ?", minLat, maxLat).
			Where("geolocation_caches.gps_longitude >= ? AND geolocation_caches.gps_longitude <= ?", minLng, maxLng)
	}

	var rows []geoRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query geo points: %w", err)
	}

	results := make([]GeoPointRow, len(rows))
	for i := range rows {
		results[i] = GeoPointRow{
			ImageFileID:  rows[i].ImageFileID,
			GPSLatitude:  rows[i].GPSLatitude,
			GPSLongitude: rows[i].GPSLongitude,
			NameLocal:    rows[i].NameLocal,
			NameEng:      rows[i].NameEng,
		}
	}
	return results, nil
}

// --- GeolocationCache operations ---

// GetGeolocationByCoords retrieves a cached geolocation entry by GPS coordinates.
func (r *MetadataRepo) GetGeolocationByCoords(lat, lng float64) (*domain.GeolocationCache, error) {
	var entry domain.GeolocationCache
	if err := r.db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query geolocation_caches: %w", err)
	}
	return &entry, nil
}

// CreateGeolocation inserts a new geolocation cache entry (handles unique conflicts).
func (r *MetadataRepo) CreateGeolocation(entry *domain.GeolocationCache) (*domain.GeolocationCache, error) {
	result := *entry
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&result).Error; err != nil {
		// Conflict — re-query
		if err := r.db.Where("gps_latitude = ? AND gps_longitude = ?", entry.GPSLatitude, entry.GPSLongitude).First(&result).Error; err != nil {
			return nil, fmt.Errorf("failed to query geolocation cache after conflict: %w", err)
		}
		return &result, nil
	}
	// If DoNothing was triggered, GORM may not populate ID
	if result.ID == 0 {
		if err := r.db.Where("gps_latitude = ? AND gps_longitude = ?", entry.GPSLatitude, entry.GPSLongitude).First(&result).Error; err != nil {
			return nil, fmt.Errorf("failed to re-query geolocation cache: %w", err)
		}
	}
	return &result, nil
}
