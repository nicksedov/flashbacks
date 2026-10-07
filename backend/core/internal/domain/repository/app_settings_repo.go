package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// AppSettingsRepository provides data access for the app_settings singleton record.
type AppSettingsRepository interface {
	Get() (*domain.AppSettings, error)
	Save(settings *domain.AppSettings) error
	Update(updates map[string]interface{}) error
}

type gormAppSettingsRepo struct {
	db *gorm.DB
}

// NewAppSettingsRepository creates a GORM-backed AppSettingsRepository.
func NewAppSettingsRepository(db *gorm.DB) AppSettingsRepository {
	return &gormAppSettingsRepo{db: db}
}

func (r *gormAppSettingsRepo) Get() (*domain.AppSettings, error) {
	var s domain.AppSettings
	if err := r.db.First(&s, 1).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormAppSettingsRepo) Save(settings *domain.AppSettings) error {
	return r.db.Save(settings).Error
}

func (r *gormAppSettingsRepo) Update(updates map[string]interface{}) error {
	var s domain.AppSettings
	if err := r.db.First(&s, 1).Error; err != nil {
		return err
	}
	return r.db.Model(&s).Updates(updates).Error
}
