package repository

import (
	"github.com/flashbacks/core/internal/domain"
	"gorm.io/gorm"
)

// PersonRepository provides data access for the people table.
type PersonRepository interface {
	// Create inserts a new person.
	Create(person *domain.Person) error
	// FindByID returns a person by ID or gorm.ErrRecordNotFound.
	FindByID(id uint) (*domain.Person, error)
	// Save persists changes to an existing person.
	Save(person *domain.Person) error
	// List returns every person ordered by ID.
	List() ([]domain.Person, error)
	// Delete removes a person row. Deleting an unknown id is not an error.
	Delete(id uint) error
}

type gormPersonRepo struct {
	db *gorm.DB
}

// NewPersonRepository creates a GORM-backed PersonRepository.
func NewPersonRepository(db *gorm.DB) PersonRepository {
	return &gormPersonRepo{db: db}
}

func (r *gormPersonRepo) Create(person *domain.Person) error {
	return r.db.Create(person).Error
}

func (r *gormPersonRepo) FindByID(id uint) (*domain.Person, error) {
	var person domain.Person
	if err := r.db.First(&person, id).Error; err != nil {
		return nil, err
	}
	return &person, nil
}

func (r *gormPersonRepo) Save(person *domain.Person) error {
	return r.db.Save(person).Error
}

func (r *gormPersonRepo) List() ([]domain.Person, error) {
	var people []domain.Person
	if err := r.db.Order("id").Find(&people).Error; err != nil {
		return nil, err
	}
	return people, nil
}

func (r *gormPersonRepo) Delete(id uint) error {
	return r.db.Where("id = ?", id).Delete(&domain.Person{}).Error
}
