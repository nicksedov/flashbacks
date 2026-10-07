package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// ImageTagRepository provides data access for image_tag records.
type ImageTagRepository interface {
	FindByImageFileID(imageFileID uint) ([]domain.ImageTag, error)
}

type gormImageTagRepo struct {
	db *gorm.DB
}

// NewImageTagRepository creates a GORM-backed ImageTagRepository.
func NewImageTagRepository(db *gorm.DB) ImageTagRepository {
	return &gormImageTagRepo{db: db}
}

func (r *gormImageTagRepo) FindByImageFileID(imageFileID uint) ([]domain.ImageTag, error) {
	var tags []domain.ImageTag
	err := r.db.Where("image_file_id = ?", imageFileID).Find(&tags).Error
	return tags, err
}
