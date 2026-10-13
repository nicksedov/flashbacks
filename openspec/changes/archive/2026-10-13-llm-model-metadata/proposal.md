# Proposal

## Why

LLM providers currently return sparse model metadata — context length from a
few providers and partial capabilities — and the webapp surfaces it only as
inline badges. Admins cannot see a model's modalities, capabilities, or token
pricing, and the Analysis Tools dropdowns list every model of a provider
regardless of whether the model can actually serve chat, VL, OCR, image edit,
or embedding. We have concrete per-provider payloads in [`models/`](../../../models/)
that let us collect this metadata correctly.

## What Changes

- Extend the LLM model metadata model with **modalities** (input/output data
  types such as `text`, `image`) and **token pricing** (input/output price per
  1M tokens), alongside the existing context length and capabilities.
- Teach each provider's `ListModels` to parse real metadata from its API,
  driven by the example payloads in [`models/`](../../../models/):
  Ollama `/api/show` (capabilities → `completion`/`thinking`/`tools`/`vision`
  plus modality inference), DeepSeek `/models` (`context_window`,
  `input_modalities`, `output_modalities`, `name`), Alibaba ModelStudio
  `/api/v1/models` (`prices`, `inference_metadata.request_modality` /
  `response_modality`, `model_info.context_window`, `capabilities`), and
  OpenAI-compatible providers via heuristics plus a static registry.
- Persist the extended metadata (a new normalized modalities table plus price
  columns) and return it through `GET /api/llm/models` and the cached models in
  `GET /api/llm/settings`.
- Add a **model details window** in the LLM Providers tab showing context
  length, modalities, capabilities, and input/output token pricing.
- Filter the Analysis Tools instrument dropdowns so each instrument type only
  lists compatible models.
- Update the core OpenAPI contract and regenerate webapp types.

## Capabilities

### New Capabilities

- `llm-model-metadata`: collection, persistence, and display of LLM model
  metadata (context length, modalities, capabilities, token pricing).

### Modified Capabilities

- `llm-settings`: the Analysis tab instrument dropdowns filter models by LLM
  instrument type instead of listing every provider model.

## Impact

- `backend/core`: `ModelInfo` and per-provider `ListModels` implementations,
  domain `LlmProviderModel` + new modalities relation + price columns,
  `LlmModelDTO`, `handleGetLlmModels`, AutoMigrate.
- `docs/api-contracts/core.yaml`: `LlmModelDTO` schema (new fields, fix `size`
  type).
- `webapp`: `src/types/index.ts` `LlmModelDTO`, generated `src/types/api.ts`,
  `AdminLlmProvidersTab` (details dialog), `AdminAnalysisTab` (filtering),
  i18n en/ru.
- No new third-party dependencies.

## Non-goals

- No live/automatic price refresh from an external pricing service; pricing is
  best-effort from provider APIs or a static registry.
- No change to the instrument settings storage schema or how inference
  resolves its model.
- No removal of manual model-name entry.
- No billing or usage-cost tracking.
