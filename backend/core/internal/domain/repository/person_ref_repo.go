package repository

import (
	"fmt"
	"math"

	"github.com/flashbacks/core/internal/domain"

	"gorm.io/gorm"
)

// PersonReferenceRepository stores a bounded set of exemplar vectors per person.
//
// Replacing the whole set on a reconcile pass keeps the storage small and the
// medoid current, without a row-by-row diff of a person's thousands of faces.
type PersonReferenceRepository interface {
	// Replace stores the exemplars of a person, dropping the previous set.
	Replace(personID uint, references []domain.PersonReference) error
	// List returns the exemplars of one person, ordered by ID.
	List(personID uint) ([]domain.PersonReference, error)
	// ListAll returns every exemplar grouped by person id.
	ListAll() (map[uint][]domain.PersonReference, error)
	// DeleteByPersonIDs removes the exemplars of several people.
	DeleteByPersonIDs(personIDs []uint) error
	// Medoid returns the exemplar closest to the set's centroid, or
	// gorm.ErrRecordNotFound when the person has no reference.
	Medoid(personID uint) (*domain.PersonReference, error)
}

type gormPersonReferenceRepo struct {
	db *gorm.DB
}

// NewPersonReferenceRepository creates a GORM-backed PersonReferenceRepository.
func NewPersonReferenceRepository(db *gorm.DB) PersonReferenceRepository {
	return &gormPersonReferenceRepo{db: db}
}

func (r *gormPersonReferenceRepo) Replace(personID uint, references []domain.PersonReference) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("person_id = ?", personID).Delete(&domain.PersonReference{}).Error; err != nil {
			return fmt.Errorf("clear references of person %d: %w", personID, err)
		}
		if len(references) == 0 {
			return nil
		}
		for index := range references {
			references[index].PersonID = personID
		}
		if err := tx.Create(&references).Error; err != nil {
			return fmt.Errorf("store references of person %d: %w", personID, err)
		}
		return nil
	})
}

func (r *gormPersonReferenceRepo) List(personID uint) ([]domain.PersonReference, error) {
	var references []domain.PersonReference
	if err := r.db.Where("person_id = ?", personID).
		Order("id").Find(&references).Error; err != nil {
		return nil, fmt.Errorf("list references of person %d: %w", personID, err)
	}
	return references, nil
}

func (r *gormPersonReferenceRepo) ListAll() (map[uint][]domain.PersonReference, error) {
	var references []domain.PersonReference
	if err := r.db.Order("person_id, id").Find(&references).Error; err != nil {
		return nil, fmt.Errorf("list person references: %w", err)
	}
	grouped := make(map[uint][]domain.PersonReference, len(references))
	for _, reference := range references {
		grouped[reference.PersonID] = append(grouped[reference.PersonID], reference)
	}
	return grouped, nil
}

func (r *gormPersonReferenceRepo) DeleteByPersonIDs(personIDs []uint) error {
	if len(personIDs) == 0 {
		return nil
	}
	if err := r.db.Where("person_id IN ?", personIDs).
		Delete(&domain.PersonReference{}).Error; err != nil {
		return fmt.Errorf("delete person references: %w", err)
	}
	return nil
}

func (r *gormPersonReferenceRepo) Medoid(personID uint) (*domain.PersonReference, error) {
	references, err := r.List(personID)
	if err != nil {
		return nil, err
	}
	if len(references) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	vectors := make([][]float32, len(references))
	dimension := 0
	for index, reference := range references {
		vector, err := domain.ParseEmbedding(reference.Embedding)
		if err != nil {
			return nil, fmt.Errorf("parse reference %d: %w", reference.ID, err)
		}
		vectors[index] = vector
		if len(vector) > dimension {
			dimension = len(vector)
		}
	}
	if dimension == 0 {
		return &references[0], nil
	}

	centroid := make([]float32, dimension)
	for _, vector := range vectors {
		for index, value := range vector {
			centroid[index] += value
		}
	}

	best := 0
	bestScore := math.Inf(-1)
	for index, vector := range vectors {
		if len(vector) == 0 {
			continue
		}
		score := cosineSimilarity(centroid, vector)
		if score > bestScore {
			bestScore, best = score, index
		}
	}
	return &references[best], nil
}

// cosineSimilarity returns the cosine similarity of two vectors, or 0 when
// either vector is empty.
func cosineSimilarity(left, right []float32) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	length := len(left)
	if len(right) < length {
		length = len(right)
	}
	var dot, leftNorm, rightNorm float64
	for index := 0; index < length; index++ {
		a, b := float64(left[index]), float64(right[index])
		dot += a * b
		leftNorm += a * a
		rightNorm += b * b
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
