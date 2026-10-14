# Tasks

## 1. Backend — modality direction in the domain and model info

- [ ] 1.1 Add `Direction string` with `ModalityDirectionInput = "input"` / `ModalityDirectionOutput = "output"` constants to [`LlmModelModality`](../../../backend/core/internal/domain/media.go:242); confirm `AutoMigrate` adds the column in `database.go` and `testutil/testdb.go`. Verify `cd backend/core && go build ./... && go vet ./...`.
- [ ] 1.2 Replace `ModelInfo.Modalities` with `InputModalities []string` and `OutputModalities []string` in [`client.go`](../../../backend/core/internal/infrastructure/llm/client.go:27) and switch [`capabilitiesFromModalities`](../../../backend/core/internal/infrastructure/llm/metadata.go:88) to derive `vision` from the input list. Verify `cd backend/core && go build ./...`.

## 2. Backend — provider parsers carry direction

- [ ] 2.1 Alibaba: set input from `inference_metadata.request_modality` and output from `response_modality` in `alibabaCatalogModelToInfo` and `alibabaEnrichedModelInfo` ([`alibaba_client.go`](../../../backend/core/internal/infrastructure/llm/alibaba_client.go:905)); update `TestAlibabaModalitiesParsed` to assert both directions. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [ ] 2.2 DeepSeek: map `input_modalities` → input and `output_modalities` → output in [`deepseek_client.go`](../../../backend/core/internal/infrastructure/llm/deepseek_client.go:361); update `TestDeepSeekModalitiesParsed`. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [ ] 2.3 Ollama: return input/output separately from [`fetchModelDetails`](../../../backend/core/internal/infrastructure/llm/ollama_client.go:249) and set both fields; update `TestOllamaVisionModality` and `TestOllamaCapabilitiesNormalized`. Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [ ] 2.4 OpenAI-compatible: set both fields from `inferModalities` in [`openai_client.go`](../../../backend/core/internal/infrastructure/llm/openai_client.go:409). Verify `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.

## 3. Backend — DTO, handlers, repository

- [ ] 3.1 Replace `LlmModelDTO.modalities` with `inputModalities` and `outputModalities` in [`media.go`](../../../backend/core/internal/interfaces/dto/media.go:649); update `TestLlmModelDTOMetadataFields` to assert the new camelCase keys. Verify `cd backend/core && go test ./internal/interfaces/dto/... -count=1`.
- [ ] 3.2 Update [`domainModelsToDTOs`](../../../backend/core/internal/interfaces/handler/handlers_llm.go:66) to group persisted rows by `Direction`, and [`handleGetLlmModels`](../../../backend/core/internal/interfaces/handler/handlers_llm.go:628) to map the live lists and persist both directions. Verify `cd backend/core && go test ./internal/interfaces/handler/... -count=1`.
- [ ] 3.3 Extend [`llm_repo_test.go`](../../../backend/core/internal/domain/repository/llm_repo_test.go:1) to assert `Preload("Modalities")` returns directional rows for both input and output. Verify `cd backend/core && go test ./internal/domain/repository/... -count=1`.
- [ ] 3.4 Run the full core suite green. Verify `cd backend/core && go test ./... -count=1 && go vet ./...`.

## 4. API contract and generated types

- [ ] 4.1 Update `LlmModelDTO` in [`core.yaml`](../../../docs/api-contracts/core.yaml:2965) — drop `modalities`, add `inputModalities` and `outputModalities` — then run `make generate-types`. Verify `git diff -- webapp/src/types/api.ts` shows the new fields and no `modalities`.
- [ ] 4.2 Align [`types/index.ts`](../../../webapp/src/types/index.ts:676) `LlmModelDTO` with `inputModalities?` and `outputModalities?`. Verify `cd webapp && npx tsc -b`.

## 5. Webapp — dialog, i18n, tests

- [ ] 5.1 Render separate "Input modalities" and "Output modalities" rows in [`ModelDetailsDialog.tsx`](../../../webapp/src/components/settings/ModelDetailsDialog.tsx:45), showing an em dash when a list is empty. Verify `cd webapp && npm run lint && npx tsc -b`.
- [ ] 5.2 Replace `llm_model_details.modalities` with `llm_model_details.inputModalities` and `llm_model_details.outputModalities` in [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:785) and [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:785) together. Verify `cd webapp && npm test -- translations`.
- [ ] 5.3 Update `ModelDetailsDialog.test.tsx` to assert the input and output rows (new test name `shows context length, input modalities, output modalities, capabilities and pricing`). Verify `cd webapp && npm test -- ModelDetailsDialog`.
- [ ] 5.4 Run the full webapp checks green. Verify `cd webapp && npm run lint && npx tsc -b && npm test`.

## 6. Evidence integration (apply phase)

- [ ] 6.1 Update the four changed entries in [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:111) (`metadata-exposed`, `alibaba-modalities-parsed`, `deepseek-modalities-parsed`, `model-details-window`) to their new selectors; confirm `node_modules/.bin/focused-spec validate --scope current --strict` reports no missing or duplicate bindings.
- [ ] 6.2 Execute the evidence: `node_modules/.bin/focused-spec run --scope current` passes with no remaining `planned:` references.
- [ ] 6.3 Mutation-check every new or changed evidence entry (revert the behavior, observe FAIL on the behavioral assertion, restore, observe PASS) and record the fail → restore → pass triple.

## 7. Integration verification

- [ ] 7.1 Confirm the webapp production build succeeds after the contract change. Verify `cd webapp && npm run build`.

## Workflow follow-up

- Sync the delta into `openspec/specs/llm-model-metadata/spec.md` (openspec-sync-specs), preserving existing `focused-spec:id` markers, and run `openspec validate --specs`.
- Archive the change into `openspec/changes/archive/YYYY-MM-DD-llm-model-modality-split/` and verify with `openspec list`.
