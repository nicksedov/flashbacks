package repository

import (
	"github.com/flashbacks/core/internal/domain"
	"gorm.io/gorm"
)

// FaceRepository provides data access for detected faces and their embeddings.
type FaceRepository interface {
	// Create inserts a detected face.
	Create(face *domain.Face) error
	// CreateEmbedding inserts a face embedding.
	CreateEmbedding(embedding *domain.FaceEmbedding) error
	// FindByID returns a face by ID or gorm.ErrRecordNotFound.
	FindByID(id uint) (*domain.Face, error)
	// FindByImageFileID returns the faces detected on an image.
	FindByImageFileID(imageFileID uint) ([]domain.Face, error)
	// ListImageFilesByPersonID returns the distinct image files a person appears
	// in, paginated and ordered by image file ID.
	ListImageFilesByPersonID(personID uint, offset, limit int) ([]domain.ImageFile, error)
	// CountImageFilesByPersonID counts the distinct photos of a person.
	CountImageFilesByPersonID(personID uint) (int64, error)
	// ListUnassigned returns the faces that belong to no person, ordered by ID
	// and paginated. Faces the user detached are included: they are unassigned
	// until the user assigns them again.
	ListUnassigned(offset, limit int) ([]domain.Face, error)
	// CountUnassigned counts the faces that belong to no person.
	CountUnassigned() (int64, error)
	// SetPersonID assigns a face to a person.
	SetPersonID(faceID, personID uint) error
}

type gormFaceRepo struct {
	db *gorm.DB
}

// NewFaceRepository creates a GORM-backed FaceRepository.
func NewFaceRepository(db *gorm.DB) FaceRepository {
	return &gormFaceRepo{db: db}
}

func (r *gormFaceRepo) Create(face *domain.Face) error {
	return r.db.Create(face).Error
}

func (r *gormFaceRepo) CreateEmbedding(embedding *domain.FaceEmbedding) error {
	return r.db.Create(embedding).Error
}

func (r *gormFaceRepo) FindByID(id uint) (*domain.Face, error) {
	var face domain.Face
	if err := r.db.First(&face, id).Error; err != nil {
		return nil, err
	}
	return &face, nil
}

func (r *gormFaceRepo) FindByImageFileID(imageFileID uint) ([]domain.Face, error) {
	var faces []domain.Face
	if err := r.db.Where("image_file_id = ?", imageFileID).Order("id").Find(&faces).Error; err != nil {
		return nil, err
	}
	return faces, nil
}

func (r *gormFaceRepo) ListImageFilesByPersonID(personID uint, offset, limit int) ([]domain.ImageFile, error) {
	var files []domain.ImageFile
	err := r.db.Table("image_files").
		Select("DISTINCT image_files.*").
		Joins("INNER JOIN faces ON faces.image_file_id = image_files.id").
		Where("faces.person_id = ?", personID).
		Order("image_files.id ASC").
		Offset(offset).
		Limit(limit).
		Find(&files).Error
	return files, err
}

func (r *gormFaceRepo) CountImageFilesByPersonID(personID uint) (int64, error) {
	var count int64
	err := r.db.Table("image_files").
		Joins("INNER JOIN faces ON faces.image_file_id = image_files.id").
		Where("faces.person_id = ?", personID).
		Distinct("image_files.id").
		Count(&count).Error
	return count, err
}

func (r *gormFaceRepo) ListUnassigned(offset, limit int) ([]domain.Face, error) {
	var faces []domain.Face
	if err := r.db.Where("person_id = ?", 0).
		Order("id").
		Offset(offset).
		Limit(limit).
		Find(&faces).Error; err != nil {
		return nil, err
	}
	return faces, nil
}

func (r *gormFaceRepo) CountUnassigned() (int64, error) {
	var count int64
	err := r.db.Model(&domain.Face{}).Where("person_id = ?", 0).Count(&count).Error
	return count, err
}

func (r *gormFaceRepo) SetPersonID(faceID, personID uint) error {
	return r.db.Model(&domain.Face{}).Where("id = ?", faceID).Update("person_id", personID).Error
}
