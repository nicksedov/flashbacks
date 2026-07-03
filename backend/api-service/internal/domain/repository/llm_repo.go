package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// LlmRepository provides data access for LLM settings, providers, and model caches.
type LlmRepository interface {
	GetSettings() (*domain.LlmSettings, error)
	CreateSettings(settings *domain.LlmSettings) error
	UpdateSettings(updates map[string]interface{}) error
	ReloadSettings() (*domain.LlmSettings, error)

	GetProviderByAlias(alias string) (*domain.LlmProvider, error)
	GetFirstProviderExcept(id uint) (*domain.LlmProvider, error)
	CreateProvider(provider *domain.LlmProvider) error
	UpdateProviderByAlias(alias string, updates map[string]interface{}) error
	DeleteProvider(provider *domain.LlmProvider) error

	GetAllModelCaches() ([]domain.LlmProviderModelCache, error)
	GetModelCacheByAlias(alias string) (*domain.LlmProviderModelCache, error)
	UpsertModelCache(cache *domain.LlmProviderModelCache) error
	DeleteModelCacheByAlias(alias string) error
	UpdateModelCacheAlias(oldAlias, newAlias string) error
}

type gormLlmRepo struct {
	db *gorm.DB
}

// NewLlmRepository creates a GORM-backed LlmRepository.
func NewLlmRepository(db *gorm.DB) LlmRepository {
	return &gormLlmRepo{db: db}
}

func (r *gormLlmRepo) GetSettings() (*domain.LlmSettings, error) {
	var s domain.LlmSettings
	if err := r.db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormLlmRepo) CreateSettings(settings *domain.LlmSettings) error {
	return r.db.Create(settings).Error
}

func (r *gormLlmRepo) UpdateSettings(updates map[string]interface{}) error {
	var s domain.LlmSettings
	if err := r.db.First(&s).Error; err != nil {
		return err
	}
	return r.db.Model(&s).Updates(updates).Error
}

func (r *gormLlmRepo) ReloadSettings() (*domain.LlmSettings, error) {
	var s domain.LlmSettings
	if err := r.db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormLlmRepo) GetProviderByAlias(alias string) (*domain.LlmProvider, error) {
	var p domain.LlmProvider
	if err := r.db.Where("alias = ?", alias).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormLlmRepo) GetFirstProviderExcept(id uint) (*domain.LlmProvider, error) {
	var p domain.LlmProvider
	if err := r.db.Where("id != ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormLlmRepo) CreateProvider(provider *domain.LlmProvider) error {
	return r.db.Create(provider).Error
}

func (r *gormLlmRepo) UpdateProviderByAlias(alias string, updates map[string]interface{}) error {
	var p domain.LlmProvider
	if err := r.db.Where("alias = ?", alias).First(&p).Error; err != nil {
		return err
	}
	return r.db.Model(&p).Updates(updates).Error
}

func (r *gormLlmRepo) DeleteProvider(provider *domain.LlmProvider) error {
	return r.db.Delete(provider).Error
}

func (r *gormLlmRepo) GetAllModelCaches() ([]domain.LlmProviderModelCache, error) {
	var rows []domain.LlmProviderModelCache
	err := r.db.Find(&rows).Error
	return rows, err
}

func (r *gormLlmRepo) GetModelCacheByAlias(alias string) (*domain.LlmProviderModelCache, error) {
	var c domain.LlmProviderModelCache
	if err := r.db.Where("provider_alias = ?", alias).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *gormLlmRepo) UpsertModelCache(cache *domain.LlmProviderModelCache) error {
	var existing domain.LlmProviderModelCache
	if err := r.db.Where("provider_alias = ?", cache.ProviderAlias).First(&existing).Error; err == nil {
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"models_json": cache.ModelsJSON,
			"fetched_at":  cache.FetchedAt,
		}).Error
	}
	return r.db.Create(cache).Error
}

func (r *gormLlmRepo) DeleteModelCacheByAlias(alias string) error {
	return r.db.Where("provider_alias = ?", alias).Delete(&domain.LlmProviderModelCache{}).Error
}

func (r *gormLlmRepo) UpdateModelCacheAlias(oldAlias, newAlias string) error {
	return r.db.Model(&domain.LlmProviderModelCache{}).
		Where("provider_alias = ?", oldAlias).
		Update("provider_alias", newAlias).Error
}
