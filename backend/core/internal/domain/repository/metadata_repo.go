package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// MetadataRepository provides data access for image_metadata and geolocation_cache records.
type MetadataRepository interface {
	FindByImageFileID(imageFileID uint) (*domain.ImageMetadata, error)
	Create(meta *domain.ImageMetadata) error
	Update(imageFileID uint, updates map[string]interface{}) error
	FindGeolocationCacheByID(id uint) (*domain.GeolocationCache, error)
}

type gormMetadataRepo struct {
	db *gorm.DB
}

// NewMetadataRepository creates a GORM-backed MetadataRepository.
func NewMetadataRepository(db *gorm.DB) MetadataRepository {
	return &gormMetadataRepo{db: db}
}

func (r *gormMetadataRepo) FindByImageFileID(imageFileID uint) (*domain.ImageMetadata, error) {
	var m domain.ImageMetadata
	if err := r.db.Where("image_file_id = ?", imageFileID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *gormMetadataRepo) Create(meta *domain.ImageMetadata) error {
	return r.db.Create(meta).Error
}

func (r *gormMetadataRepo) Update(imageFileID uint, updates map[string]interface{}) error {
	var m domain.ImageMetadata
	if err := r.db.Where("image_file_id = ?", imageFileID).First(&m).Error; err != nil {
		return err
	}
	return r.db.Model(&m).Updates(updates).Error
}

func (r *gormMetadataRepo) FindGeolocationCacheByID(id uint) (*domain.GeolocationCache, error) {
	var g domain.GeolocationCache
	if err := r.db.First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}
