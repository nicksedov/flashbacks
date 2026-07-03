// Package repository defines data access interfaces and GORM implementations,
// decoupling HTTP handlers from direct database access (Layer Isolation).
package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// ImageFileRepository provides data access for image_file records.
type ImageFileRepository interface {
	FindByPath(path string) (*domain.ImageFile, error)
	FindByID(id uint) (*domain.ImageFile, error)
	CountByPathPrefix(prefix string) (int64, error)
	DeleteByPathPrefix(prefix string) (int64, error)
}

type gormImageFileRepo struct {
	db *gorm.DB
}

// NewImageFileRepository creates a GORM-backed ImageFileRepository.
func NewImageFileRepository(db *gorm.DB) ImageFileRepository {
	return &gormImageFileRepo{db: db}
}

func (r *gormImageFileRepo) FindByPath(path string) (*domain.ImageFile, error) {
	var f domain.ImageFile
	if err := r.db.Where("path = ?", path).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *gormImageFileRepo) FindByID(id uint) (*domain.ImageFile, error) {
	var f domain.ImageFile
	if err := r.db.First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *gormImageFileRepo) CountByPathPrefix(prefix string) (int64, error) {
	var count int64
	err := r.db.Model(&domain.ImageFile{}).Where("path LIKE ?", prefix+"%").Count(&count).Error
	return count, err
}

func (r *gormImageFileRepo) DeleteByPathPrefix(prefix string) (int64, error) {
	result := r.db.Where("path LIKE ?", prefix+"%").Delete(&domain.ImageFile{})
	return result.RowsAffected, result.Error
}
