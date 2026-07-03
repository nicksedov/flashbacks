package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// UserSettings is the domain type for user-level settings (defined in domain package).
// We re-declare the struct reference to avoid circular imports.

// UserSettingsRepository provides data access for user_settings records.
type UserSettingsRepository interface {
	FindOrCreateByUserID(userID uint) (*domain.UserSettings, error)
	Save(settings *domain.UserSettings) error
}

type gormUserSettingsRepo struct {
	db *gorm.DB
}

// NewUserSettingsRepository creates a GORM-backed UserSettingsRepository.
func NewUserSettingsRepository(db *gorm.DB) UserSettingsRepository {
	return &gormUserSettingsRepo{db: db}
}

func (r *gormUserSettingsRepo) FindOrCreateByUserID(userID uint) (*domain.UserSettings, error) {
	var s domain.UserSettings
	if err := r.db.FirstOrCreate(&s, domain.UserSettings{UserID: userID}).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormUserSettingsRepo) Save(settings *domain.UserSettings) error {
	return r.db.Save(settings).Error
}
