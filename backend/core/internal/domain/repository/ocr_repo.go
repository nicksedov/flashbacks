package repository

import (
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// OcrDocumentRow holds the flattened result of joining ocr_classifications with image_files.
type OcrDocumentRow struct {
	ID                 uint
	ImageFileID        uint
	Path               string
	Size               int64
	Hash               string
	ModTime            time.Time
	MeanConfidence     float32
	WeightedConfidence float32
	TokenCount         int
	Angle              int
	ScaleFactor        float32
}

// OcrRepository provides data access for OCR classification data.
type OcrRepository interface {
	CountDocuments() (int64, error)
	FindDocumentsPaginated(offset, limit int) ([]OcrDocumentRow, error)
	FindClassificationByPath(imagePath string) (*domain.OcrClassification, error)
	FindBoundingBoxesByClassificationID(classificationID uint) ([]domain.OcrBoundingBox, error)
}

type gormOcrRepo struct {
	db *gorm.DB
}

// NewOcrRepository creates a GORM-backed OcrRepository.
func NewOcrRepository(db *gorm.DB) OcrRepository {
	return &gormOcrRepo{db: db}
}

func (r *gormOcrRepo) CountDocuments() (int64, error) {
	var total int64
	err := r.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("ocr_classifications.is_text_document = true").
		Count(&total).Error
	return total, err
}

func (r *gormOcrRepo) FindDocumentsPaginated(offset, limit int) ([]OcrDocumentRow, error) {
	var results []OcrDocumentRow
	err := r.db.Table("ocr_classifications").
		Select("image_files.id, image_files.path, image_files.size, image_files.hash, image_files.mod_time, ocr_classifications.image_file_id, ocr_classifications.mean_confidence, ocr_classifications.weighted_confidence, ocr_classifications.token_count, ocr_classifications.angle, ocr_classifications.scale_factor").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("ocr_classifications.is_text_document = true").
		Order("image_files.id").
		Offset(offset).
		Limit(limit).
		Find(&results).Error
	return results, err
}

func (r *gormOcrRepo) FindClassificationByPath(imagePath string) (*domain.OcrClassification, error) {
	var c domain.OcrClassification
	err := r.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("image_files.path = ?", imagePath).
		First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *gormOcrRepo) FindBoundingBoxesByClassificationID(classificationID uint) ([]domain.OcrBoundingBox, error) {
	var boxes []domain.OcrBoundingBox
	err := r.db.Where("classification_id = ?", classificationID).Find(&boxes).Error
	return boxes, err
}
