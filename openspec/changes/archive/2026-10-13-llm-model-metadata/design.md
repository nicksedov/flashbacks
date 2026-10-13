# Design

## Context

The metadata pipeline today is: provider `ListModels` → [`llm.ModelInfo`](../../../backend/core/internal/infrastructure/llm/client.go:27) → [`domain.LlmProviderModel`](../../../backend/core/internal/domain/media.go:206) (+ normalized `llm_model_capabilities`) → [`dto.LlmModelDTO`](../../../backend/core/internal/interfaces/dto/media.go:649) → `GET /api/llm/models` and the cached models of `GET /api/llm/settings`. The webapp shows only capability badges and context length inline in [`AdminLlmProvidersTab`](../../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx), and [`AdminAnalysisTab`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx) model dropdowns list every provider model. See `proposal.md` for motivation.

The [`models/`](../../../models/) examples pin the real API shapes: Ollama `/api/show` (`capabilities`, `model_info.*.context_length`), DeepSeek `/models` (`context_window`, `input_modalities`, `output_modalities`, `name`), and Alibaba ModelStudio `/api/v1/models` (`prices[].prices[]` with `type`/`price`/`price_unit`, `inference_metadata.request_modality`/`response_modality`, `model_info.context_window`, `capabilities`).

## Goals / Non-Goals

**Goals:**

- Extend the shared metadata shape with modalities and input/output token price (per 1M tokens) end to end.
- Normalize provider payloads into that shape, using the `models/` examples as the contract.
- Surface the metadata in a model details window and filter instrument dropdowns by model type.

**Non-Goals:**

- No live pricing service or external dependency for pricing.
- No change to the instrument settings storage or to how inference resolves its model.
- No billing/usage accounting.

## Decisions

### 1. Extend the relational metadata model

`ModelInfo` and `LlmProviderModel` gain `Modalities []string` and `InputPricePerMTok` / `OutputPricePerMTok float64` (0 = unknown). Modalities are stored in a new normalized `llm_model_modalities` table mirroring `llm_model_capabilities` (avoids the JSON-blob anti-pattern the model cache was migrated away from). Prices are scalar columns on `llm_provider_models`.

- Alternative: JSON column for modalities — rejected; it reintroduces the legacy blob pattern.

### 2. Standardized capability vocabulary

The webapp `ModelCapability` union (`chat`, `tool_calling`, `vision`, `embedding`) gains `image_edit`. Providers normalize:

- Ollama `completion` → `chat`, `tools` → `tool_calling`, `vision` → `vision`, `embedding` → `embedding`. `thinking` is intentionally not surfaced as a capability (no instrument depends on it).
- `vision` implies the `image` input modality.
- `image_edit` is derived from the model ID (`image-edit`, `edit`) for providers without a native image-edit signal (Alibaba `qwen-image-edit`, OpenAI-compatible edits).

### 3. Per-provider collection strategy

- **Ollama**: keep `/api/tags` + `/api/show`; extend the capability mapping and add modality inference.
- **DeepSeek**: call `/models` (the rich endpoint seen in [`models/deepseek-response.json`](../../../models/deepseek-response.json:1)) instead of `/v1/models`; parse `context_window`, `input_modalities`, `output_modalities`, `name`. Fall back to `/v1/models` if `/models` is unavailable.
- **Alibaba**: call the ModelStudio catalog at the derived base (strip `/compatible-mode/v1`) `/api/v1/models`, parse `prices`, `inference_metadata.*_modality`, `model_info.context_window`, `capabilities`; fall back to the existing heuristics plus a static registry seeded from [`models/alibaba-response.json`](../../../models/alibaba-response.json:1) when the catalog endpoint is unreachable or empty.
- **OpenAI-compatible**: no metadata endpoint; use a static registry for common models plus ID heuristics (same normalization helpers).

### 4. Pricing representation

Prices are stored and returned per 1M tokens as `float64`; `0` means unknown. Where a provider returns a `Per 1M tokens` price directly (Alibaba), use it; otherwise a static per-provider registry supplies known model prices.

### 5. Webapp: filtering helper + details dialog

- A pure helper `filterModelsForInstrument(models, type)` maps an instrument type to the required capability: `chat` → `chat`, `vl` → `vision`, `ocr` → `vision`, `image_edit` → `image_edit`, `embedding` → `embedding`. `AdminAnalysisTab` uses it for every instrument dropdown; manual model-name entry remains available.
- A presentational `ModelDetailsDialog` renders size, context length, modalities, capabilities, and input/output price; `AdminLlmProvidersTab` opens it from a model row.

### 6. Evidence plan (focused-spec)

Identity is bound via the `<!-- focused-spec:id=... -->` markers already placed on the delta scenarios. At apply time we register the `go` runner in [`.focused-spec/config.yaml`](../../../.focused-spec/config.yaml:19) (so backend scenarios can be executed; the vitest runner is already registered) and append the following entries to [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) under `current`:

| Scenario ID | Planned selector |
|---|---|
| `llm-model-metadata.metadata-exposed` | `go::./internal/interfaces/dto::TestLlmModelDTOMetadataFields` |
| `llm-model-metadata.ollama-capabilities-normalized` | `go::./internal/infrastructure/llm::TestOllamaCapabilitiesNormalized` |
| `llm-model-metadata.ollama-vision-modality` | `go::./internal/infrastructure/llm::TestOllamaVisionModality` |
| `llm-model-metadata.deepseek-context-parsed` | `go::./internal/infrastructure/llm::TestDeepSeekContextParsed` |
| `llm-model-metadata.deepseek-modalities-parsed` | `go::./internal/infrastructure/llm::TestDeepSeekModalitiesParsed` |
| `llm-model-metadata.alibaba-pricing-parsed` | `go::./internal/infrastructure/llm::TestAlibabaPricingParsed` |
| `llm-model-metadata.alibaba-modalities-parsed` | `go::./internal/infrastructure/llm::TestAlibabaModalitiesParsed` |
| `llm-model-metadata.model-details-window` | `vitest::webapp/src/components/settings/ModelDetailsDialog.test.tsx::ModelDetailsDialog > shows context length, modalities, capabilities and pricing` |
| `llm-settings.chat-filter` | `vitest::webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > chat returns only chat-capable models` |
| `llm-settings.vl-filter` | `vitest::webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > vl returns only vision-capable models` |
| `llm-settings.ocr-filter` | `vitest::webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > ocr returns only vision-capable models` |
| `llm-settings.image-edit-filter` | `vitest::webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > image_edit returns only image-edit-capable models` |
| `llm-settings.embedding-filter` | `vitest::webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > embedding returns only embedding-capable models` |

Go packages are relative to the `backend/core` module; the `go` runner will be configured to run `go test` inside `backend/core` (via the runner's `cwd`/`commandArgs`; the exact invocation is validated at apply time with the local CLI). Each entry is turned into a real test, then `validate` + `run` + the mutation check are executed.

## Risks / Trade-offs

- [Alibaba ModelStudio host is workspace-specific] → the catalog call is best-effort with a static-registry/heuristic fallback; never blocks model listing.
- [DeepSeek `/models` endpoint may not exist for some deployments] → fall back to `/v1/models` (IDs only) without metadata.
- [Prices drift over time] → the static registry is a fallback; providers that expose prices are parsed live. Out-of-scope: automatic refresh.
- [Marker IDs must survive sync into the main spec] → the sync step carries markers verbatim; verified with `openspec validate --specs`.
- [Go evidence requires the `go` runner registration] → added in this change's apply stage; the vitest runner already exists.

## Migration Plan

- `AutoMigrate` already includes `LlmProviderModel` and `LlmModelCapability`; add the new `llm_model_modalities` table and the price columns to that migration list (SQLite in-memory for tests, Postgres in production).
- `make generate-types` regenerates `webapp/src/types/api.ts` after the OpenAPI `LlmModelDTO` update; `webapp/src/types/index.ts` is updated by hand and must stay field-aligned.
- Rollback is reverting the feature branch; the new columns/table are additive and nullable-safe (empty list / 0 = unknown).
