# Proposal

## Why

The Analysis tab (LLM Settings → image edit instrument) lists only the models whose
stored capabilities include `image_edit`. The backend grants that capability only
for the ModelStudio code `IG`/`image_generation` or for identifiers containing
`image-edit`/`image_edit`, and the curated family registry has no entry for the wan
or qwen image families. As a result the image-generation models of the wan2.7
family (`wan2.7-image`, `wan2.7-i2i`, …) are recorded as `chat` + `tool_calling`,
so they never appear in the image edit dropdown even though the Alibaba client
already edits images with them. The same misclassification also leaks image
generators into the chat dropdown.

## What Changes

- Image-generation and editing identifiers are detected structurally: the curated
  token list (`image-edit`, `image_edit`, `imageedit`, `qwen-image`, `wanx`,
  `cogview`, `gpt-image`, `dall-e`, `stable-diffusion`, `i2i`, `t2i`) plus the
  family-prefix rule that matches versioned `wan*`/`qwen*` names containing
  `image`. A detected image generator is recorded as exactly `image_edit` and never
  as `chat`/`tool_calling`.
- Video siblings of the same families (`wan2.7-t2v`, `wan2.7-i2v`) are explicitly
  not image-edit capable.
- A model whose reported output modality is `image`, and that reports no
  capability codes, is recorded as exactly `image_edit` instead of `chat`. The
  modality derivation now receives both modality directions.
- `image_edit` models derive image input and image output when the provider reports
  no modalities.
- The Alibaba catalog conversion applies the identifier rule, so entries that omit
  or under-report the `IG` code are corrected, and the separator-less
  `ImageGeneration`/`imagegen` spellings map to `image_edit`.
- A catalog refresh replaces a stored classification when the refresh reports the
  model's modalities and capabilities, so an installation that stored `wan2.7-image`
  as `chat` is corrected on the next refresh instead of keeping the stale value
  forever. Identifier-only (heuristic) refreshes still never overwrite stored
  metadata.

## Non-goals

- No change to the webapp: the image edit dropdown keeps filtering on the stored
  `image_edit` capability ([`llm-model-filter.ts`](../../../webapp/src/lib/llm-model-filter.ts)).
- No change to the provider/native-catalog priority order, the catalog TTL, the
  `stale` reporting, or the models API contract — no OpenAPI or generated TS type
  change.
- No change to how chat/VL/OCR/embedding instruments are resolved, and no new AI
  action.
- Video-capability detection for `wan2.7-t2v`/`wan2.7-i2v` from the identifier
  alone is out of scope; those models keep relying on the provider's `VG` code.
- No new dependency and no new user-facing string, so no `en`/`ru` locale change.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `llm-model-metadata`: capability collection gains image-generation family
  detection and image-output modality derivation, and a refresh replaces a stored
  classification the provider's own report determines.

## Impact

- Backend core, infrastructure by provider-agnostic metadata pipeline:
  [`metadata.go`](../../../backend/core/internal/infrastructure/llm/metadata.go)
  (identifier detection, modality derivation, code aliases) and
  [`alibaba_client.go`](../../../backend/core/internal/infrastructure/llm/alibaba_client.go)
  (catalog conversion, Alibaba heuristics); the DeepSeek listing passes both
  modality directions to the shared derivation.
- Backend core, application catalog:
  [`catalog.go`](../../../backend/core/internal/application/llm/catalog.go)
  (`describesFully` accepts a partial entry that reported modalities and
  capabilities).
- Tests:
  [`metadata_test.go`](../../../backend/core/internal/infrastructure/llm/metadata_test.go),
  [`alibaba_client_test.go`](../../../backend/core/internal/infrastructure/llm/alibaba_client_test.go),
  [`catalog_test.go`](../../../backend/core/internal/application/llm/catalog_test.go).
- Data effect: stored provider catalogs are corrected on the next refresh (TTL or
  the explicit “load models” action); no migration and no schema change.
- Evidence registry: new `llm-model-metadata.*` scenario bindings in
  [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml).
