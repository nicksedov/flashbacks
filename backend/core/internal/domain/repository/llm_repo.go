package repository

import (
	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// LlmRepository provides data access for LLM settings, providers, and model caches.
type LlmRepository interface {
	// --- Providers ---
	GetProviderByAlias(alias string) (*domain.LlmProvider, error)
	GetFirstProviderExcept(id uint) (*domain.LlmProvider, error)
	CreateProvider(provider *domain.LlmProvider) error
	UpdateProviderByAlias(alias string, updates map[string]interface{}) error
	DeleteProvider(provider *domain.LlmProvider) error

	// --- Model Caches (deprecated JSON blob) ---
	GetAllModelCaches() ([]domain.LlmProviderModelCache, error)
	GetModelCacheByAlias(alias string) (*domain.LlmProviderModelCache, error)
	UpsertModelCache(cache *domain.LlmProviderModelCache) error
	DeleteModelCacheByAlias(alias string) error
	UpdateModelCacheAlias(oldAlias, newAlias string) error

	// --- Provider Models (normalized) ---
	GetModelsByProviderID(providerID uint) ([]domain.LlmProviderModel, error)
	GetModelsByProviderAlias(alias string) ([]domain.LlmProviderModel, error)
	ReplaceProviderModels(providerID uint, models []domain.LlmProviderModel) error
	DeleteModelsByProviderID(providerID uint) error

	// --- Tag Scan Settings ---
	GetTagScanSettings() (*domain.TagScanSettings, error)
	UpsertTagScanSettings(settings *domain.TagScanSettings) error

	// --- LLM Instrument Settings ---
	GetInstrumentByType(instrumentType domain.InstrumentType) (*domain.LlmInstrumentSettings, error)
	GetAllInstruments() ([]domain.LlmInstrumentSettings, error)
	UpsertInstrument(settings *domain.LlmInstrumentSettings) error
	DeleteInstrumentByType(instrumentType domain.InstrumentType) error

	// --- Embedding Settings ---
	GetEmbeddingSettings() (*domain.EmbeddingSettings, error)
	UpsertEmbeddingSettings(settings *domain.EmbeddingSettings) error
}

type gormLlmRepo struct {
	db *gorm.DB
}

// NewLlmRepository creates a GORM-backed LlmRepository.
func NewLlmRepository(db *gorm.DB) LlmRepository {
	return &gormLlmRepo{db: db}
}

// --- Providers ---

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

// --- Model Caches (deprecated) ---

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

// --- Provider Models (normalized) ---

func (r *gormLlmRepo) GetModelsByProviderID(providerID uint) ([]domain.LlmProviderModel, error) {
	var models []domain.LlmProviderModel
	err := r.db.Where("llm_provider_id = ?", providerID).
		Preload("Capabilities").
		Order("model_name ASC").
		Find(&models).Error
	return models, err
}

func (r *gormLlmRepo) GetModelsByProviderAlias(alias string) ([]domain.LlmProviderModel, error) {
	var provider domain.LlmProvider
	if err := r.db.Where("alias = ?", alias).First(&provider).Error; err != nil {
		return nil, err
	}
	return r.GetModelsByProviderID(provider.ID)
}

// ReplaceProviderModels atomically replaces all models for a provider.
// Deletes existing models (capabilities cascade) and inserts new ones.
func (r *gormLlmRepo) ReplaceProviderModels(providerID uint, models []domain.LlmProviderModel) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing models for this provider (capabilities cascade via FK)
		if err := tx.Where("llm_provider_id = ?", providerID).Delete(&domain.LlmProviderModel{}).Error; err != nil {
			return err
		}

		// Insert new models one by one (GORM's Create doesn't handle nested creates well in bulk)
		for i := range models {
			models[i].LlmProviderID = providerID
			models[i].ID = 0
			if err := tx.Create(&models[i]).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *gormLlmRepo) DeleteModelsByProviderID(providerID uint) error {
	return r.db.Where("llm_provider_id = ?", providerID).Delete(&domain.LlmProviderModel{}).Error
}

// --- Tag Scan Settings ---

func (r *gormLlmRepo) GetTagScanSettings() (*domain.TagScanSettings, error) {
	var s domain.TagScanSettings
	if err := r.db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormLlmRepo) UpsertTagScanSettings(settings *domain.TagScanSettings) error {
	var existing domain.TagScanSettings
	if err := r.db.First(&existing).Error; err == nil {
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"enabled":         settings.Enabled,
			"start_hour":      settings.StartHour,
			"start_minute":    settings.StartMinute,
			"end_hour":        settings.EndHour,
			"end_minute":      settings.EndMinute,
			"timezone_offset": settings.TimezoneOffset,
		}).Error
	}
	return r.db.Create(settings).Error
}

// --- LLM Instrument Settings ---

func (r *gormLlmRepo) GetInstrumentByType(instrumentType domain.InstrumentType) (*domain.LlmInstrumentSettings, error) {
	var s domain.LlmInstrumentSettings
	if err := r.db.Where("type = ?", instrumentType).Preload("Provider").First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormLlmRepo) GetAllInstruments() ([]domain.LlmInstrumentSettings, error) {
	var rows []domain.LlmInstrumentSettings
	err := r.db.Preload("Provider").Find(&rows).Error
	return rows, err
}

func (r *gormLlmRepo) UpsertInstrument(settings *domain.LlmInstrumentSettings) error {
	var existing domain.LlmInstrumentSettings
	if err := r.db.Where("type = ?", settings.Type).First(&existing).Error; err == nil {
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"provider_id": settings.ProviderID,
			"model":       settings.Model,
		}).Error
	}
	return r.db.Create(settings).Error
}

func (r *gormLlmRepo) DeleteInstrumentByType(instrumentType domain.InstrumentType) error {
	return r.db.Where("type = ?", instrumentType).Delete(&domain.LlmInstrumentSettings{}).Error
}

// --- Embedding Settings ---

func (r *gormLlmRepo) GetEmbeddingSettings() (*domain.EmbeddingSettings, error) {
	var s domain.EmbeddingSettings
	if err := r.db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormLlmRepo) UpsertEmbeddingSettings(settings *domain.EmbeddingSettings) error {
	var existing domain.EmbeddingSettings
	if err := r.db.First(&existing).Error; err == nil {
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"dimension":  settings.Dimension,
			"batch_size": settings.BatchSize,
		}).Error
	}
	return r.db.Create(settings).Error
}
