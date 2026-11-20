package repository

import (
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/infrastructure/cache"
	"gorm.io/gorm"
)

// UserSettingsRepository provides data access for user_settings records.
type UserSettingsRepository interface {
	FindOrCreateByUserID(userID uint) (*domain.UserSettings, error)
	Save(settings *domain.UserSettings) error
}

type gormUserSettingsRepo struct {
	db *gorm.DB
	// cache may be nil, which disables caching.
	cache *cache.TTLCache[uint, *domain.UserSettings]
}

// NewUserSettingsRepository creates a GORM-backed UserSettingsRepository. A nil
// cache disables caching.
func NewUserSettingsRepository(db *gorm.DB, settings *cache.TTLCache[uint, *domain.UserSettings]) UserSettingsRepository {
	return &gormUserSettingsRepo{db: db, cache: settings}
}

func (r *gormUserSettingsRepo) FindOrCreateByUserID(userID uint) (*domain.UserSettings, error) {
	if r.cache != nil {
		if cached, ok := r.cache.Get(userID); ok {
			snapshot := *cached
			return &snapshot, nil
		}
	}

	var s domain.UserSettings
	if err := r.db.FirstOrCreate(&s, domain.UserSettings{UserID: userID}).Error; err != nil {
		return nil, err
	}

	if r.cache != nil {
		stored := s
		r.cache.Set(userID, &stored)
	}
	return &s, nil
}

func (r *gormUserSettingsRepo) Save(settings *domain.UserSettings) error {
	if err := r.db.Save(settings).Error; err != nil {
		return err
	}
	if r.cache != nil {
		r.cache.Invalidate(settings.UserID)
	}
	return nil
}
