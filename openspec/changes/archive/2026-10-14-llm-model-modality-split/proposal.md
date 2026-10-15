# Proposal

## Why

The model details view in the LLM Providers tab shows a single merged
"Modalities" line. The models API collapses each model's input and output
modalities into one `modalities` array (Alibaba `request_modality` +
`response_modality`), so an admin cannot tell what a model accepts from what it
returns — e.g. `Qwen3.8-Max` reports `Image`, `Text`, and `Video` input but only
`Text` output, yet both are rendered as one undifferentiated list.

## What Changes

- **BREAKING** (models API): replace the merged `LlmModelDTO.modalities` field
  with `inputModalities` and `outputModalities` on `GET /api/llm/models` and the
  cached models embedded in `GET /api/llm/settings`.
- Persist modality direction: `LlmModelModality` gains an `input`/`output`
  direction column, so the merged union is no longer stored.
- Attribute each modality to a direction per provider: Alibaba
  `inference_metadata.request_modality` → input and `response_modality` → output;
  DeepSeek `input_modalities` → input and `output_modalities` → output; Ollama
  and OpenAI-compatible providers infer input (`text`, plus `image` when vision)
  and output (`text`).
- Capability derivation continues to treat an `image` **input** modality as the
  `vision` capability.
- Show input modalities and output modalities as two separate rows in the model
  details window, with en/ru labels.
- Update the core OpenAPI contract and regenerate the webapp types.

## Capabilities

### New Capabilities

_None — the behavior changes belong to the existing metadata capability._

### Modified Capabilities

- `llm-model-metadata`: the models API exposes input and output modalities
  separately instead of one merged list; provider parsers attribute each modality
  to a direction; the model details window shows input and output modalities as
  separate rows.

## Impact

- **Backend core**: [`ModelInfo`](../../../backend/core/internal/infrastructure/llm/client.go:27),
  provider `ListModels` in
  [`alibaba_client.go`](../../../backend/core/internal/infrastructure/llm/alibaba_client.go:913),
  [`deepseek_client.go`](../../../backend/core/internal/infrastructure/llm/deepseek_client.go:361),
  [`ollama_client.go`](../../../backend/core/internal/infrastructure/llm/ollama_client.go:296),
  and [`openai_client.go`](../../../backend/core/internal/infrastructure/llm/openai_client.go:409);
  capability/modality helpers in
  [`metadata.go`](../../../backend/core/internal/infrastructure/llm/metadata.go:74);
  domain [`LlmProviderModel`/`LlmModelModality`](../../../backend/core/internal/domain/media.go:206);
  [`LlmModelDTO`](../../../backend/core/internal/interfaces/dto/media.go:649);
  [`handlers_llm.go`](../../../backend/core/internal/interfaces/handler/handlers_llm.go:66);
  [`llm_repo.go`](../../../backend/core/internal/domain/repository/llm_repo.go:83);
  AutoMigrate in both `database.go` and `testutil/testdb.go`.
- **API contract**: [`core.yaml`](../../../docs/api-contracts/core.yaml:2965) —
  `LlmModelDTO` (drop `modalities`, add `inputModalities` and `outputModalities`),
  then `make generate-types` for [`webapp/src/types/api.ts`](../../../webapp/src/types/api.ts:2144).
- **Webapp**: [`types/index.ts`](../../../webapp/src/types/index.ts:676),
  [`ModelDetailsDialog.tsx`](../../../webapp/src/components/settings/ModelDetailsDialog.tsx:60),
  i18n [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:785) +
  `translations.ru.ts`, and the dialog test.
- No new third-party dependencies. No Wire DI graph change.

## Non-goals

- No change to the standardized capability vocabulary or to instrument
  (chat/VL/OCR/image-edit/embedding) filtering.
- No retroactive backfill of already-cached merged modality rows; a provider
  model refresh repopulates directionality.
- No new provider integrations and no new modality values.
