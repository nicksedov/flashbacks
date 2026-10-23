# Tasks

## 1. Image-family identifier detection

- [ ] 1.1 Add `imageGenerationFamilyTokens` and `isImageGenerationModelID()` to `backend/core/internal/infrastructure/llm/metadata.go` (curated tokens plus the versioned `wan*`/`qwen*` + `image` rule) and make `inferCapabilitiesByID()` return exactly `image_edit` for a detected family before the chat default; verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1` including `TestImageGenerationIdentifiersAreImageEditCapable` and `TestEnrichedWanImageModelSupportsImageEditing`, which also assert that `wan2.7-t2v`, `wan2.7-i2v`, and plain chat identifiers do not receive `image_edit`.
- [ ] 1.2 Apply the same predicate in `inferAlibabaCapabilities()` (`backend/core/internal/infrastructure/llm/alibaba_client.go`) so the Alibaba identifier heuristics agree with the shared pipeline; verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1` and confirm `TestAlibabaCapabilitiesDeduplicated` stays green.

## 2. Modality-derived capabilities

- [ ] 2.1 Change `capabilitiesFromModalities()` to accept input and output modalities and return exactly `image_edit` for an `image` output, keep `vision` for an `image` input with text output, and update both call sites (`normalizeModelInfo()` and the DeepSeek listing) plus the `normalizeModelInfo()` guard so an output-only modality list still derives capabilities; verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1` covering the new `TestImageOutputModalityIsImageEditCapable` and the existing `TestNormalizeModelInfoListingInput`.
- [ ] 2.2 Extend `inferModalities()` so an `image_edit` capability implies image input and image output; verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`, where `TestEnrichedWanImageModelSupportsImageEditing` asserts both modality directions for `wan2.7-image`.

## 3. Alibaba catalog conversion

- [ ] 3.1 In `alibabaCatalogModelToInfo()`, let `isImageGenerationModelID()` decide for the wan/qwen image families: an entry that does not answer in text becomes exactly `image_edit`, an entry that answers in text gains `image_edit` additively; verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1` covering `TestAlibabaCatalogUnderReportedImageModelIsImageEditCapable`, `TestAlibabaCatalogWanImageWithNativeCodeIsImageEditCapable`, and `TestAlibabaCatalogWanVideoStaysVideoOnly`.
- [ ] 3.2 Map the separator-less `ImageGeneration`/`imagegen` capability codes to `image_edit` in `capabilityCodeMap` (`metadata.go`); verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1` and the extended `TestNormalizeCapabilitiesImageGeneration`.

## 4. Refresh authority for provider-described models

- [ ] 4.1 In `backend/core/internal/application/llm/catalog.go`, let `describesFully()` accept a `partial` entry that carries capabilities, an input modality, and a context length, keeping `heuristic` entries protected; verify with `cd backend/core && go test ./internal/application/llm/... -count=1` for the new `TestRefreshCorrectsImageEditClassification` and the existing `TestDegradedRefreshKeepsStoredModalities`.

## 5. Evidence and scenario binding

- [ ] 5.1 Bind every `llm-model-metadata.wan-image-identifier`, `image-family-identifiers`, `video-family-not-image-edit`, `image-output-implies-image-edit`, `alibaba-underreported-image-model`, `alibaba-image-code-spellings`, `alibaba-video-model-not-image-edit`, and `refresh-replaces-stored-classification` entry in `.focused-spec/evidence.yaml` under scope `current` with exact `go::<package>::<TestName>` references, and verify `./node_modules/.bin/focused-spec validate --scope current --strict` reports no problems.
- [ ] 5.2 Run `./node_modules/.bin/focused-spec run --scope current` and observe `PASS` for every new reference; then prove sensitivity per scenario: make `isImageGenerationModelID()` return `false`, observe the family and catalog scenarios `FAIL`, restore it and observe `PASS`; make `capabilitiesFromModalities()` ignore the output list, observe the modality scenario `FAIL`, restore it and observe `PASS`; revert `describesFully()` to the `rich`-only form, observe the refresh scenario `FAIL`, restore it and observe `PASS`.

## 6. Full component verification

- [ ] 6.1 Run the core check suite from `backend/core`: `go test ./internal/application/... ./internal/infrastructure/llm/... -count=1 && go vet ./... && go build ./...`, and confirm `gofmt -l` reports no file touched by this change.

## Workflow follow-up

- Review the branch diff against `docs/agent/sdd-workflow.md` §7 and record the verdict.
- Sync the delta into `openspec/specs/llm-model-metadata/spec.md` with `openspec-sync-specs`.
- Archive the change with `openspec-archive-change`.
