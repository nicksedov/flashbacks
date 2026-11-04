# Proposal

## Why

LLM model metadata is currently produced by two diverging paths: a rich provider-native
catalog (capabilities, input/output modalities, context window, per-1M prices) and a lossy
ID-heuristic fallback, while the stored cache is written only on an explicit forced refresh
and fails silently. The result is visible in production: `qwen3.5-flash` is stored as
`1.0M` context with `text, image, video` input and `vision`, yet the same model is returned
by `GET /api/llm/models?force=true` as `32768` context, `text`-only and without `vision`, so
it never reaches the OCR/VL model dropdowns. The cache also never converges because a single
duplicate capability violates a unique index, the write transaction rolls back, and the error
is discarded.

## What Changes

- Introduce one authoritative metadata collection pipeline with an explicit source priority
  (native rich catalog → native per-model detail → listing + curated model-family registry →
  ID heuristics → safe default), applied identically to the API response and to the stored
  cache; no path may bypass enrichment.
- Make capability normalization total and safe: capabilities are deduplicated before storage,
  derived from modalities when native codes are missing, and never inferred from an ID token
  that merely looks like a modality (image/video generators are not `vision`, chat models are
  only `chat`/`tool_calling` when they produce text).
- **BREAKING (internal behavior)**: stop serving a live list that differs from the stored
  catalog. `GET /api/llm/models` returns the stored catalog, re-collected when it is older than
  a configurable TTL (default 24h); if that re-collection fails, the last good catalog is served
  and marked stale. `refresh=true` remains an explicit override.
- Never swallow catalog write failures: a failed or degraded refresh keeps the previous
  metadata (merge, not replace), reports the failure, and never downgrades a model that was
  previously known to be vision-capable.
- Populate instrument model dropdowns automatically from the cached catalog; the manual
  "load models" action is no longer required to use a dropdown.
- Extend the core OpenAPI contract with an optional `refresh` query parameter and the
  `fetchedAt` / `stale` response fields of the models endpoint.

## Capabilities

### New Capabilities

- `llm-model-catalog`: how the system collects model metadata from each provider source
  priority, persists it as the single authoritative catalog, refreshes it on a TTL, and keeps
  the last good metadata when a refresh fails or degrades.

### Modified Capabilities

- `llm-model-metadata`: adds requirements that collected capabilities are deduplicated, are
  derived from reported modalities when native codes are absent, and are never granted from a
  model identifier alone.
- `llm-settings`: adds requirements that instrument dropdowns are populated automatically from
  the stored catalog, that the explicit refresh action updates them, and that a stale catalog is
  indicated to the admin in English and Russian.

## Impact

- Backend (core): `internal/infrastructure/llm/` (`alibaba_client.go`, `openai_client.go`,
  `deepseek_client.go`, `ollama_client.go`, `metadata.go`), `internal/interfaces/handler/handlers_llm.go`,
  `internal/domain/media.go` + `internal/domain/repository/llm_repo.go` (catalog fetch timestamp,
  merge-on-refresh), `internal/infrastructure/database` (schema), `cmd/server/wire.go` +
  `wire_gen.go`, new config value for the cache TTL.
- Webapp: `components/tabs/AdminAnalysisTab.tsx` (automatic model loading, refresh control,
  stale notice), `lib/llm-model-filter.ts` (filter unchanged: OCR and VL still map to the
  `vision` capability), `types/api.ts` (regenerated), `i18n/translations.{en,ru}.ts` for the
  stale-catalog notice.
- Contracts: `docs/api-contracts/core.yaml` (models endpoint query/response), then
  `make generate-types`.
- No change to OCR recognition flow, provider credentials, or the exif/ocr services.

## Non-goals

- No new providers or provider types.
- No capability/price editing UI and no manual metadata overrides.
- No change to how a recognition request picks its model/provider at runtime.
- No per-model refresh scheduling: the TTL applies per provider catalog.
- No migration of historical metadata beyond re-collecting it on the first post-deploy refresh.
