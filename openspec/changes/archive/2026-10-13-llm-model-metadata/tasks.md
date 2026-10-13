# Tasks

## 1. Backend metadata model

- [x] 1.1 Extend [`llm.ModelInfo`](../../../backend/core/internal/infrastructure/llm/client.go:27) with `Modalities []string`, `InputPricePerMTok float64`, `OutputPricePerMTok float64`; extend [`domain.LlmProviderModel`](../../../backend/core/internal/domain/media.go:206) with the same fields plus a `Modalities []LlmModelModality` relation, and add the new `LlmModelModality` table (`llm_model_modalities`) and price columns. Verify `cd backend/core && go build ./...`.
- [x] 1.2 Add `LlmProviderModel` price columns and `LlmModelModality` to AutoMigrate in [`database.go`](../../../backend/core/internal/infrastructure/database/database.go:39) and [`testutil/testdb.go`](../../../backend/core/internal/testutil/testdb.go:40). Verify `cd backend/core && go build ./... && go vet ./...`.
- [x] 1.3 Extend [`dto.LlmModelDTO`](../../../backend/core/internal/interfaces/dto/media.go:649) with `modalities`, `inputPricePerMTok`, `outputPricePerMTok`; update [`domainModelsToDTOs`](../../../backend/core/internal/interfaces/handler/handlers_llm.go:66) and [`handleGetLlmModels`](../../../backend/core/internal/interfaces/handler/handlers_llm.go:548) to map/persist them; add a dto test `TestLlmModelDTOMetadataFields` asserting the camelCase JSON keys. Verify `cd backend/core && go test ./internal/interfaces/dto/... -count=1`.

## 2. Provider metadata collection

- [x] 2.1 Add shared helpers in a new [`metadata.go`](../../../backend/core/internal/infrastructure/llm/metadata.go:1) (package `llm`): capability normalization, modality inference, and a static per-provider pricing registry. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [x] 2.2 Ollama: extend [`fetchModelDetails`](../../../backend/core/internal/infrastructure/llm/ollama_client.go:249) to map `completion` → `chat`, keep `tools` → `tool_calling` / `vision` → `vision` / `embedding` → `embedding`, and infer the `image` input modality from `vision`; add `TestOllamaCapabilitiesNormalized` and `TestOllamaVisionModality`. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [x] 2.3 DeepSeek: change [`ListModels`](../../../backend/core/internal/infrastructure/llm/deepseek_client.go:290) to parse the rich `/models` shape (`context_window`, `input_modalities`, `output_modalities`, `name`) with a `/v1/models` fallback; add `TestDeepSeekContextParsed` and `TestDeepSeekModalitiesParsed`. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [x] 2.4 Alibaba: parse the ModelStudio `/api/v1/models` catalog (prices with `Per 1M tokens`, `inference_metadata.*_modality`, `model_info.context_window`, `capabilities`) with the existing heuristic/registry fallback; add `TestAlibabaPricingParsed` and `TestAlibabaModalitiesParsed`. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [x] 2.5 OpenAI-compatible: apply the shared normalization helpers and registry to [`ListModels`](../../../backend/core/internal/infrastructure/llm/openai_client.go:390). Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.

## 3. Contract and generated types

- [x] 3.1 Update [`LlmModelDTO`](../../../docs/api-contracts/core.yaml:2965) in the core OpenAPI contract: add `modalities`, `inputPricePerMTok`, `outputPricePerMTok`, `contextLength`, `capabilities` and fix `size` to `integer`; run `make generate-types` and confirm [`webapp/src/types/api.ts`](../../../webapp/src/types/api.ts:2144) is regenerated. Verify `git diff -- webapp/src/types/api.ts` shows the new fields.
- [x] 3.2 Update [`LlmModelDTO`](../../../webapp/src/types/index.ts:676) with `modalities?`, `inputPricePerMTok?`, `outputPricePerMTok?` matching the Go JSON tags. Verify `cd webapp && npx tsc -b`.

## 4. Webapp features and i18n

- [x] 4.1 Add a pure helper `filterModelsForInstrument` in a new [`webapp/src/lib/llm-model-filter.ts`](../../../webapp/src/lib/llm-model-filter.ts:1) mapping `chat`→`chat`, `vl`→`vision`, `ocr`→`vision`, `image_edit`→`image_edit`, `embedding`→`embedding`, with a Vitest test covering all five types. Verify `cd webapp && npm test`.
- [x] 4.2 Add a presentational [`ModelDetailsDialog`](../../../webapp/src/components/settings/ModelDetailsDialog.tsx:1) component (size, context length, modalities, capabilities, input/output price) with a Vitest test. Verify `cd webapp && npm run lint && npx tsc -b && npm test`.
- [x] 4.3 Wire the dialog into [`AdminLlmProvidersTab`](../../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx:634): make model rows clickable to open `ModelDetailsDialog`. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 4.4 Use `filterModelsForInstrument` in [`AdminAnalysisTab`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx:352) for the chat/vl/ocr/image_edit/embedding model dropdowns. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 4.5 Add en/ru translations for the new labels (modalities, capabilities, input/output price, model details dialog title, no-compatible-models hint) to [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:1) and [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:1) together. Verify `cd webapp && npm test` (includes `translations.test.ts`).

## 5. Focused-spec evidence integration

- [x] 5.1 Register the `go` runner in [`.focused-spec/config.yaml`](../../../.focused-spec/config.yaml:19) so Go evidence runs inside `backend/core`. Verify `node_modules/.bin/focused-spec validate --scope current --syntax-only` passes.
- [x] 5.2 Append the 13 evidence entries from the design.md table to [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) under `current`. Verify `node_modules/.bin/focused-spec validate --scope current --strict` reports `49 registered items, 0 planned evidence`.
- [x] 5.3 Turn each selector into its real test and run `node_modules/.bin/focused-spec run --scope current` until it reports all new evidence passing (`49 PASS, 0 FAIL`).
- [x] 5.4 Mutation-check each new/changed evidence entry: revert the behavior under test, confirm the focused evidence fails on the behavioral assertion, restore, and confirm it passes again.

## 6. Full verification

- [x] 6.1 Run the core suite and checks green: `cd backend/core && go test ./internal/application/... -count=1 && go test ./internal/infrastructure/llm/... ./internal/interfaces/dto/... -count=1 && go vet ./...`.
- [x] 6.2 Run the webapp checks green: `cd webapp && npm run lint && npx tsc -b && npm test && npm run build`.

## Workflow follow-up

- Sync the delta specs into `openspec/specs/llm-model-metadata/spec.md` and `openspec/specs/llm-settings/spec.md` (openspec-sync-specs) and run `openspec validate --specs`.
- Archive the change into `openspec/changes/archive/`.
