package repository

import (
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/infrastructure/cache"
	"gorm.io/gorm"
)

// appSettingsSingletonID is the primary key of the single app_settings row.
const appSettingsSingletonID = 1

// AppSettingsRepository provides data access for the app_settings singleton record.
type AppSettingsRepository interface {
	Get() (*domain.AppSettings, error)
	Save(settings *domain.AppSettings) error
	Update(updates map[string]interface{}) error
}

type gormAppSettingsRepo struct {
	db *gorm.DB
	// cache may be nil, which disables caching.
	cache *cache.TTLCache[int, *domain.AppSettings]
}

// NewAppSettingsRepository creates a GORM-backed AppSettingsRepository. A nil
// cache disables caching.
func NewAppSettingsRepository(db *gorm.DB, settings *cache.TTLCache[int, *domain.AppSettings]) AppSettingsRepository {
	return &gormAppSettingsRepo{db: db, cache: settings}
}

func (r *gormAppSettingsRepo) Get() (*domain.AppSettings, error) {
	if r.cache != nil {
		if cached, ok := r.cache.Get(appSettingsSingletonID); ok {
			snapshot := *cached
			return &snapshot, nil
		}
	}

	var s domain.AppSettings
	if err := r.db.First(&s, appSettingsSingletonID).Error; err != nil {
		return nil, err
	}

	if r.cache != nil {
		stored := s
		r.cache.Set(appSettingsSingletonID, &stored)
	}
	return &s, nil
}

func (r *gormAppSettingsRepo) Save(settings *domain.AppSettings) error {
	if err := r.db.Save(settings).Error; err != nil {
		return err
	}
	r.invalidate()
	return nil
}

func (r *gormAppSettingsRepo) Update(updates map[string]interface{}) error {
	var s domain.AppSettings
	if err := r.db.First(&s, appSettingsSingletonID).Error; err != nil {
		return err
	}
	if err := r.db.Model(&s).Updates(updates).Error; err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// invalidate drops the cached singleton after a write, so the next read sees
// the new values.
func (r *gormAppSettingsRepo) invalidate() {
	if r.cache != nil {
		r.cache.Invalidate(appSettingsSingletonID)
	}
}
