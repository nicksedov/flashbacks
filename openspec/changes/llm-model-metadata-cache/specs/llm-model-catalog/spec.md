# Spec Delta

## Purpose

Defines the single authoritative catalog of LLM models exposed to the admin UI: how metadata
is collected from provider sources, how long it stays valid, and what happens when a refresh
degrades or fails.

## ADDED Requirements

### Requirement: Model metadata is collected through one prioritized pipeline

The system SHALL collect model metadata through a single pipeline that consults sources in a
fixed priority order: the provider's native rich catalog, the provider's native per-model
detail, the provider listing enriched by the curated model-family registry, model-ID
heuristics, and finally a safe default. A lower-priority source SHALL only supply fields that
higher-priority sources left unknown.

<!-- focused-spec:id=llm-model-catalog.native-source-preferred -->
#### Scenario: Native rich catalog is preferred

- **WHEN** a provider exposes a native rich catalog for its models
- **THEN** every listed model carries the capabilities, modalities, context length and token prices that catalog reports

<!-- focused-spec:id=llm-model-catalog.registry-fills-partial-listing -->
#### Scenario: Curated registry fills a listing without metadata

- **WHEN** a provider exposes only a model listing without capabilities or modalities
- **THEN** those models receive capabilities and modalities from their curated model-family registry entry

<!-- focused-spec:id=llm-model-catalog.unknown-model-safe-default -->
#### Scenario: Undescribed model gets the safe default

- **WHEN** neither the provider nor the curated registry describes a model's capabilities
- **THEN** the model is recorded as chat and tool_calling with text-only modalities

### Requirement: The models endpoint serves the stored catalog

The models endpoint SHALL answer from the persisted catalog of the requested provider. It
SHALL NOT return a model list whose metadata differs from what is stored for that provider.

<!-- focused-spec:id=llm-model-catalog.response-equals-stored -->
#### Scenario: Response metadata equals stored metadata

- **WHEN** an admin requests the model list for a provider whose stored catalog is current
- **THEN** the returned capabilities, modalities, context length and token prices equal the stored ones for every model

<!-- focused-spec:id=llm-model-catalog.response-exposes-freshness -->
#### Scenario: Response exposes catalog freshness

- **WHEN** the models endpoint returns a provider catalog
- **THEN** the response reports the time of the last successful refresh as `fetchedAt`

### Requirement: Catalog entries refresh on a TTL

The system SHALL treat a stored provider catalog as valid for a configured time to live and
SHALL re-collect it once that interval has elapsed. The time to live SHALL be configurable and
SHALL default to 24 hours.

<!-- focused-spec:id=llm-model-catalog.fresh-catalog-not-refetched -->
#### Scenario: Fresh catalog is served without re-collection

- **WHEN** a stored catalog is younger than the configured time to live
- **THEN** the models request is answered from the stored catalog without querying the provider

<!-- focused-spec:id=llm-model-catalog.expired-catalog-refetched -->
#### Scenario: Expired catalog is re-collected

- **WHEN** a stored catalog is older than the configured time to live
- **THEN** the system re-collects the provider catalog before answering the request

### Requirement: An explicit refresh ignores the time to live

The models endpoint SHALL accept an explicit refresh request that re-collects a provider
catalog regardless of its age.

<!-- focused-spec:id=llm-model-catalog.explicit-refresh-ignores-ttl -->
#### Scenario: Explicit refresh re-collects a fresh catalog

- **WHEN** an admin requests the model list with the refresh flag
- **THEN** the provider catalog is re-collected even though the stored catalog is younger than the time to live

### Requirement: A degraded refresh never overwrites known metadata

A refresh SHALL be merged into the stored catalog field by field. A field the refresh could
not determine SHALL keep its stored value, and models absent from a degraded refresh SHALL
remain in the catalog.

<!-- focused-spec:id=llm-model-catalog.degraded-refresh-keeps-vision -->
#### Scenario: Degraded refresh keeps a known vision capability

- **WHEN** a refresh cannot determine a model's capabilities and the stored catalog records that model as vision-capable
- **THEN** the stored vision capability is kept

<!-- focused-spec:id=llm-model-catalog.degraded-refresh-keeps-context -->
#### Scenario: Degraded refresh keeps a known context length

- **WHEN** a refresh returns a model without a context length and the stored catalog has one
- **THEN** the stored context length is kept

<!-- focused-spec:id=llm-model-catalog.degraded-refresh-keeps-models -->
#### Scenario: Degraded refresh keeps missing models

- **WHEN** a degraded refresh returns fewer models than the stored catalog
- **THEN** the models absent from that refresh remain in the catalog

### Requirement: A failed catalog write is reported and leaves the catalog intact

When a refreshed catalog cannot be persisted the system SHALL report the failure to the caller
instead of reporting success, and the previously stored catalog SHALL remain in effect.

<!-- focused-spec:id=llm-model-catalog.write-failure-reported -->
#### Scenario: Failed write is reported

- **WHEN** persisting a refreshed provider catalog fails
- **THEN** the models response reports a failed refresh instead of success

<!-- focused-spec:id=llm-model-catalog.write-failure-keeps-catalog -->
#### Scenario: Failed write leaves the catalog unchanged

- **WHEN** persisting a refreshed provider catalog fails
- **THEN** the models endpoint still returns the previously stored catalog for that provider
