// Package llm implements the LLM model catalog: the single authoritative
// pipeline that collects, enriches, persists, and serves provider model
// metadata.
package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	modelllm "github.com/flashbacks/core/internal/infrastructure/llm"
)

// DefaultCacheTTL is the lifetime of a stored provider catalog when no other
// value is configured.
const DefaultCacheTTL = 24 * time.Hour

// ErrProviderNotFound reports an unknown provider alias.
var ErrProviderNotFound = errors.New("llm provider not found")

// ModelLister lists the models a provider currently exposes. It is the
// transport-facing port of the catalog pipeline.
type ModelLister interface {
	ListModels(ctx context.Context, provider domain.LlmProvider, model string) ([]modelllm.ModelInfo, error)
}

// ClientLister lists models by building the appropriate provider client.
type ClientLister struct {
	maxImageMegapixels float64
}

// NewClientLister creates a ModelLister backed by the provider client factory.
func NewClientLister(maxImageMegapixels float64) *ClientLister {
	return &ClientLister{maxImageMegapixels: maxImageMegapixels}
}

// ListModels implements ModelLister.
func (l *ClientLister) ListModels(ctx context.Context, provider domain.LlmProvider, model string) ([]modelllm.ModelInfo, error) {
	client, err := modelllm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, model, l.maxImageMegapixels)
	if err != nil {
		return nil, err
	}
	return client.ListModels(ctx)
}

// Config configures the catalog service.
type Config struct {
	// TTL is how long a stored provider catalog stays valid.
	TTL time.Duration
	// DefaultModel is the model used when a provider listing needs one.
	DefaultModel string
	// Now returns the current time; it is replaceable for tests.
	Now func() time.Time
}

// Result is a served provider catalog together with its freshness.
type Result struct {
	Provider domain.LlmProvider
	Models   []modelllm.ModelInfo
	// FetchedAt is the time of the last successful refresh (nil = never).
	FetchedAt *time.Time
	// Stale reports that the last refresh attempt failed, so the served catalog
	// is the previously stored one.
	Stale bool
	// RefreshError carries the failure text while Stale is true.
	RefreshError error
}

// Service is the authoritative LLM model catalog.
type Service struct {
	store  repository.LlmCatalogRepository
	lister ModelLister
	config Config
	logger *slog.Logger

	mu       sync.Mutex
	inflight map[uint]*refreshFlight
}

// refreshFlight is an in-progress refresh that concurrent callers wait on.
type refreshFlight struct {
	done chan struct{}
	err  error
}

// NewService creates a catalog service.
func NewService(store repository.LlmCatalogRepository, lister ModelLister, config Config) *Service {
	if config.TTL <= 0 {
		config.TTL = DefaultCacheTTL
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.DefaultModel == "" {
		config.DefaultModel = "minicpm-v"
	}
	return &Service{
		store:    store,
		lister:   lister,
		config:   config,
		logger:   slog.Default(),
		inflight: make(map[uint]*refreshFlight),
	}
}

// Get returns the stored catalog of a provider. The catalog is re-collected when
// it has never been refreshed, is older than the configured TTL, or the caller
// asked for an explicit refresh.
//
// When a re-collection fails, the previously stored catalog is returned with
// Stale set. Only a provider with no stored catalog at all surfaces an error.
func (s *Service) Get(ctx context.Context, providerAlias string, refresh bool) (Result, error) {
	provider, err := s.store.GetProviderByAlias(providerAlias)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s", ErrProviderNotFound, providerAlias)
	}
	return s.getForProvider(ctx, *provider, refresh)
}

// getForProvider implements Get for an already resolved provider.
func (s *Service) getForProvider(ctx context.Context, provider domain.LlmProvider, refresh bool) (Result, error) {
	stored, err := s.store.GetProviderCatalog(provider.ID)
	if err != nil {
		return Result{}, fmt.Errorf("load provider catalog: %w", err)
	}

	if !refresh && !s.isExpired(stored) {
		return resultFromStored(provider, stored, false, nil), nil
	}

	refreshErr := s.refresh(ctx, provider)

	stored, err = s.store.GetProviderCatalog(provider.ID)
	if err != nil {
		return Result{}, fmt.Errorf("reload provider catalog: %w", err)
	}

	if refreshErr != nil {
		if len(stored.Models) == 0 {
			return Result{}, fmt.Errorf("refresh provider catalog: %w", refreshErr)
		}
		s.logger.Warn("llm catalog refresh failed, serving the stored catalog",
			"provider", provider.Alias, "stage", "refresh", "error", refreshErr)
		return resultFromStored(provider, stored, true, refreshErr), nil
	}

	return resultFromStored(provider, stored, false, nil), nil
}

// isExpired reports whether the stored catalog must be re-collected.
func (s *Service) isExpired(stored *repository.StoredProviderCatalog) bool {
	if stored.FetchedAt == nil {
		return true
	}
	return s.config.Now().Sub(*stored.FetchedAt) >= s.config.TTL
}

// refresh re-collects and persists a provider catalog, with one in-flight
// refresh per provider so concurrent requests do not stampede the provider.
func (s *Service) refresh(ctx context.Context, provider domain.LlmProvider) error {
	return s.singleFlight(ctx, provider.ID, func() error {
		return s.collectAndPersist(ctx, provider)
	})
}

// singleFlight runs fn once per provider while concurrent callers wait for the
// same result.
func (s *Service) singleFlight(ctx context.Context, providerID uint, fn func() error) error {
	s.mu.Lock()
	if flight, running := s.inflight[providerID]; running {
		s.mu.Unlock()
		select {
		case <-flight.done:
			return flight.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	flight := &refreshFlight{done: make(chan struct{})}
	s.inflight[providerID] = flight
	s.mu.Unlock()

	err := fn()
	flight.err = err
	close(flight.done)

	s.mu.Lock()
	delete(s.inflight, providerID)
	s.mu.Unlock()

	return err
}

// collectAndPersist lists the provider models, runs them through the metadata
// pipeline, and merges the result into the stored catalog.
func (s *Service) collectAndPersist(ctx context.Context, provider domain.LlmProvider) error {
	raw, err := s.lister.ListModels(ctx, provider, s.config.DefaultModel)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return fmt.Errorf("provider %s returned no models", provider.Alias)
	}

	rich := true
	models := make([]domain.LlmProviderModel, 0, len(raw))
	for _, info := range raw {
		if !isRichInfo(info) {
			rich = false
		}
		models = append(models, storedModelFromInfo(modelllm.EnrichModelInfo(info)))
	}

	if !rich {
		s.logger.Info("llm catalog refresh produced a degraded catalog",
			"provider", provider.Alias, "models", len(models))
	}

	return s.store.MergeProviderCatalog(provider.ID, models, s.config.Now(), rich)
}

// isRichInfo reports whether a collected model carries a full, provider-supplied
// description. A refresh containing any undescribed model is not rich, so it may
// not delete stored models.
func isRichInfo(info modelllm.ModelInfo) bool {
	return len(info.Capabilities) > 0 &&
		info.ContextLength > 0 &&
		len(info.InputModalities) > 0
}

// resultFromStored builds the served result from the persisted catalog.
func resultFromStored(provider domain.LlmProvider, stored *repository.StoredProviderCatalog, stale bool, refreshErr error) Result {
	return Result{
		Provider:     provider,
		Models:       modelInfosFromStored(stored.Models),
		FetchedAt:    stored.FetchedAt,
		Stale:        stale,
		RefreshError: refreshErr,
	}
}

// storedModelFromInfo converts a normalized model into its persisted form.
func storedModelFromInfo(info modelllm.ModelInfo) domain.LlmProviderModel {
	model := domain.LlmProviderModel{
		ModelID:            info.ID,
		ModelName:          info.Name,
		Size:               info.Size,
		ContextLength:      info.ContextLength,
		InputPricePerMTok:  info.InputPricePerMTok,
		OutputPricePerMTok: info.OutputPricePerMTok,
	}
	if model.ModelName == "" {
		model.ModelName = info.ID
	}

	seen := make(map[string]struct{}, len(info.Capabilities))
	for _, capability := range info.Capabilities {
		if _, duplicate := seen[capability]; duplicate {
			continue
		}
		seen[capability] = struct{}{}
		model.Capabilities = append(model.Capabilities, domain.LlmModelCapability{Capability: capability})
	}

	for _, modality := range info.InputModalities {
		model.Modalities = append(model.Modalities, domain.LlmModelModality{
			Modality:  modality,
			Direction: domain.ModalityDirectionInput,
		})
	}
	for _, modality := range info.OutputModalities {
		model.Modalities = append(model.Modalities, domain.LlmModelModality{
			Modality:  modality,
			Direction: domain.ModalityDirectionOutput,
		})
	}

	return model
}

// modelInfosFromStored converts persisted models back into served models.
func modelInfosFromStored(models []domain.LlmProviderModel) []modelllm.ModelInfo {
	infos := make([]modelllm.ModelInfo, len(models))
	for i, stored := range models {
		info := modelllm.ModelInfo{
			ID:                 stored.ModelID,
			Name:               stored.ModelName,
			Size:               stored.Size,
			ContextLength:      stored.ContextLength,
			InputPricePerMTok:  stored.InputPricePerMTok,
			OutputPricePerMTok: stored.OutputPricePerMTok,
		}
		for _, capability := range stored.Capabilities {
			info.Capabilities = append(info.Capabilities, capability.Capability)
		}
		for _, modality := range stored.Modalities {
			if modality.Direction == domain.ModalityDirectionOutput {
				info.OutputModalities = append(info.OutputModalities, modality.Modality)
				continue
			}
			info.InputModalities = append(info.InputModalities, modality.Modality)
		}
		infos[i] = info
	}
	return infos
}
