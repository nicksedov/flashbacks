package repository

import (
	"time"

	"github.com/flashbacks/core/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StoredProviderCatalog is the persisted model catalog of one provider together
// with the freshness metadata of its last successful refresh.
type StoredProviderCatalog struct {
	ProviderID uint
	Models     []domain.LlmProviderModel
	// FetchedAt is nil when the provider catalog has never been refreshed.
	FetchedAt *time.Time
	// SourceRich reports whether the last successful refresh came from a rich
	// provider source (as opposed to a degraded listing).
	SourceRich bool
}

// LlmCatalogRepository is the persistence port of the LLM model catalog: it
// reads the stored catalog, merges a refreshed catalog, and owns the provider's
// catalog freshness metadata.
type LlmCatalogRepository interface {
	// GetProviderByAlias resolves a configured provider by its alias.
	GetProviderByAlias(alias string) (*domain.LlmProvider, error)
	// GetProviderCatalog returns the stored catalog of a provider together with
	// its freshness metadata.
	GetProviderCatalog(providerID uint) (*StoredProviderCatalog, error)
	// MergeProviderCatalog merges a refreshed catalog into the stored one field by
	// field and records the refresh timestamp and provenance in the same
	// transaction. Models absent from a degraded (non-rich) refresh are kept.
	MergeProviderCatalog(providerID uint, models []domain.LlmProviderModel, fetchedAt time.Time, sourceRich bool) error
}

// NewLlmCatalogRepository creates a GORM-backed LlmCatalogRepository.
func NewLlmCatalogRepository(db *gorm.DB) LlmCatalogRepository {
	return &gormLlmRepo{db: db}
}

// GetProviderCatalog returns the stored catalog and freshness of a provider.
func (r *gormLlmRepo) GetProviderCatalog(providerID uint) (*StoredProviderCatalog, error) {
	var provider domain.LlmProvider
	if err := r.db.First(&provider, providerID).Error; err != nil {
		return nil, err
	}
	models, err := r.GetModelsByProviderID(providerID)
	if err != nil {
		return nil, err
	}
	return &StoredProviderCatalog{
		ProviderID: providerID,
		Models:     models,
		FetchedAt:  provider.ModelsFetchedAt,
		SourceRich: provider.ModelsSourceRich,
	}, nil
}

// MergeProviderCatalog upserts the refreshed models and writes the provider's
// freshness metadata inside one transaction, so a failed write leaves both the
// catalog rows and the timestamp untouched.
func (r *gormLlmRepo) MergeProviderCatalog(
	providerID uint,
	models []domain.LlmProviderModel,
	fetchedAt time.Time,
	sourceRich bool,
) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing []domain.LlmProviderModel
		if err := tx.Where("llm_provider_id = ?", providerID).
			Preload("Capabilities").
			Preload("Modalities").
			Find(&existing).Error; err != nil {
			return err
		}

		existingByModelID := make(map[string]domain.LlmProviderModel, len(existing))
		for _, m := range existing {
			existingByModelID[m.ModelID] = m
		}

		kept := make(map[string]struct{}, len(models))
		for i := range models {
			incoming := models[i]
			incoming.LlmProviderID = providerID

			previous, existed := existingByModelID[incoming.ModelID]
			if !existed {
				if err := createCatalogModel(tx, incoming); err != nil {
					return err
				}
				kept[incoming.ModelID] = struct{}{}
				continue
			}
			if err := mergeCatalogModel(tx, previous, incoming); err != nil {
				return err
			}
			kept[incoming.ModelID] = struct{}{}
		}

		// Only a rich refresh may delete models the provider no longer reports.
		if sourceRich {
			for _, stale := range existing {
				if _, ok := kept[stale.ModelID]; ok {
					continue
				}
				if err := deleteCatalogModel(tx, stale.ID); err != nil {
					return err
				}
			}
		}

		return tx.Model(&domain.LlmProvider{}).
			Where("id = ?", providerID).
			Updates(map[string]interface{}{
				"models_fetched_at":  fetchedAt,
				"models_source_rich": sourceRich,
			}).Error
	})
}

// createCatalogModel inserts a model row and its capability/modality rows.
func createCatalogModel(tx *gorm.DB, model domain.LlmProviderModel) error {
	row := domain.LlmProviderModel{
		LlmProviderID:      model.LlmProviderID,
		ModelID:            model.ModelID,
		ModelName:          model.ModelName,
		Size:               model.Size,
		ContextLength:      model.ContextLength,
		InputPricePerMTok:  model.InputPricePerMTok,
		OutputPricePerMTok: model.OutputPricePerMTok,
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	if err := insertCapabilities(tx, row.ID, model.Capabilities); err != nil {
		return err
	}
	return insertModalities(tx, row.ID, model.Modalities)
}

// mergeCatalogModel keeps stored values for every field the refresh could not
// determine. Determined capabilities and modalities replace the stored ones so
// a corrected capability set can actually shrink.
func mergeCatalogModel(tx *gorm.DB, previous, incoming domain.LlmProviderModel) error {
	merged := domain.LlmProviderModel{
		ID:                 previous.ID,
		LlmProviderID:      previous.LlmProviderID,
		ModelID:            previous.ModelID,
		ModelName:          incoming.ModelName,
		Size:               incoming.Size,
		ContextLength:      incoming.ContextLength,
		InputPricePerMTok:  incoming.InputPricePerMTok,
		OutputPricePerMTok: incoming.OutputPricePerMTok,
		CreatedAt:          previous.CreatedAt,
	}
	if merged.ModelName == "" {
		merged.ModelName = previous.ModelName
	}
	if merged.Size == 0 {
		merged.Size = previous.Size
	}
	if merged.ContextLength == 0 {
		merged.ContextLength = previous.ContextLength
	}
	if merged.InputPricePerMTok == 0 {
		merged.InputPricePerMTok = previous.InputPricePerMTok
	}
	if merged.OutputPricePerMTok == 0 {
		merged.OutputPricePerMTok = previous.OutputPricePerMTok
	}

	if err := tx.Save(&merged).Error; err != nil {
		return err
	}

	if len(incoming.Capabilities) > 0 {
		if err := tx.Where("model_id = ?", previous.ID).
			Delete(&domain.LlmModelCapability{}).Error; err != nil {
			return err
		}
		if err := insertCapabilities(tx, previous.ID, incoming.Capabilities); err != nil {
			return err
		}
	}

	for _, direction := range []string{domain.ModalityDirectionInput, domain.ModalityDirectionOutput} {
		refreshed := modalitiesInDirection(incoming.Modalities, direction)
		if len(refreshed) == 0 {
			continue
		}
		if err := tx.Where("model_id = ? AND direction = ?", previous.ID, direction).
			Delete(&domain.LlmModelModality{}).Error; err != nil {
			return err
		}
		if err := insertModalities(tx, previous.ID, refreshed); err != nil {
			return err
		}
	}

	return nil
}

// deleteCatalogModel removes a model row and its association rows.
func deleteCatalogModel(tx *gorm.DB, modelRowID uint) error {
	if err := tx.Where("model_id = ?", modelRowID).
		Delete(&domain.LlmModelCapability{}).Error; err != nil {
		return err
	}
	if err := tx.Where("model_id = ?", modelRowID).
		Delete(&domain.LlmModelModality{}).Error; err != nil {
		return err
	}
	return tx.Delete(&domain.LlmProviderModel{}, modelRowID).Error
}

// insertCapabilities inserts capability rows, skipping duplicates both in Go and
// via ON CONFLICT DO NOTHING so a single odd row can never abort the whole
// provider refresh.
func insertCapabilities(tx *gorm.DB, modelRowID uint, capabilities []domain.LlmModelCapability) error {
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if capability.Capability == "" {
			continue
		}
		if _, duplicate := seen[capability.Capability]; duplicate {
			continue
		}
		seen[capability.Capability] = struct{}{}

		row := domain.LlmModelCapability{
			ModelID:    modelRowID,
			Capability: capability.Capability,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// insertModalities inserts modality rows, skipping duplicates.
func insertModalities(tx *gorm.DB, modelRowID uint, modalities []domain.LlmModelModality) error {
	seen := make(map[string]struct{}, len(modalities))
	for _, modality := range modalities {
		if modality.Modality == "" {
			continue
		}
		direction := modality.Direction
		if direction == "" {
			direction = domain.ModalityDirectionInput
		}
		key := direction + "|" + modality.Modality
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}

		row := domain.LlmModelModality{
			ModelID:   modelRowID,
			Modality:  modality.Modality,
			Direction: direction,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// modalitiesInDirection filters modality rows by direction.
func modalitiesInDirection(modalities []domain.LlmModelModality, direction string) []domain.LlmModelModality {
	out := make([]domain.LlmModelModality, 0, len(modalities))
	for _, modality := range modalities {
		rowDirection := modality.Direction
		if rowDirection == "" {
			rowDirection = domain.ModalityDirectionInput
		}
		if rowDirection == direction {
			out = append(out, modality)
		}
	}
	return out
}
