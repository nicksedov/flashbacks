package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// GalleryFolderRepository provides data access for gallery_folder records.
type GalleryFolderRepository interface {
	FindAll() ([]domain.GalleryFolder, error)
	FindByID(id uint) (*domain.GalleryFolder, error)
	Create(folder *domain.GalleryFolder) error
	Delete(id uint) error
}

type gormGalleryFolderRepo struct {
	db *gorm.DB
}

// NewGalleryFolderRepository creates a GORM-backed GalleryFolderRepository.
func NewGalleryFolderRepository(db *gorm.DB) GalleryFolderRepository {
	return &gormGalleryFolderRepo{db: db}
}

func (r *gormGalleryFolderRepo) FindAll() ([]domain.GalleryFolder, error) {
	var folders []domain.GalleryFolder
	err := r.db.Order("created_at").Find(&folders).Error
	return folders, err
}

func (r *gormGalleryFolderRepo) FindByID(id uint) (*domain.GalleryFolder, error) {
	var f domain.GalleryFolder
	if err := r.db.First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *gormGalleryFolderRepo) Create(folder *domain.GalleryFolder) error {
	return r.db.Create(folder).Error
}

func (r *gormGalleryFolderRepo) Delete(id uint) error {
	return r.db.Delete(&domain.GalleryFolder{}, id).Error
}
