# Design

## Context

See [proposal.md](proposal.md) for motivation and
[specs/llm-model-metadata/spec.md](specs/llm-model-metadata/spec.md) for the
revised requirements.

Today a model's modalities are collapsed into a single `[]string` at every layer:
[`ModelInfo.Modalities`](../../../backend/core/internal/infrastructure/llm/client.go:27)
(union of input + output), the normalized
[`domain.LlmModelModality`](../../../backend/core/internal/domain/media.go:242)
child rows (one row per modality, no direction), and
[`dto.LlmModelDTO.modalities`](../../../backend/core/internal/interfaces/dto/media.go:649).
The provider payloads already carry direction — Alibaba
`inference_metadata.request_modality` / `response_modality`, DeepSeek
`input_modalities` / `output_modalities` — but
[`mergeModalities`](../../../backend/core/internal/infrastructure/llm/metadata.go:96)
discards it. The webapp renders the merged list as one row in
[`ModelDetailsDialog`](../../../webapp/src/components/settings/ModelDetailsDialog.tsx:45).

Constraints: the models API is consumed only by the webapp; the DB is auto-migrated
from domain structs (`AutoMigrate` in `database.go` and `testutil/testdb.go`); no
Wire DI graph change is involved; no inter-service REST calls are touched.

## Goals / Non-Goals

**Goals:**

- Carry modality direction (input vs. output) end-to-end: provider parser →
  `ModelInfo` → persistence → DTO → webapp.
- Keep capability derivation (`vision` from an `image` input) working.
- Keep the change a single, reviewable delta across the four layers.

**Non-Goals:**

- Not changing the standardized capability vocabulary or instrument filtering.
- Not adding a migration tool to split already-cached merged rows.
- Not changing provider-selection or pricing behavior.

## Decisions

### D1 — Persist direction as a column on `LlmModelModality`, not a new table

Add `Direction string` (`input` | `output`) to
[`LlmModelModality`](../../../backend/core/internal/domain/media.go:242) with
exported constants, and keep the existing
`Modalities []LlmModelModality` association.

- **Why:** the association and its `Preload("Modalities")` already exist; a column
  is added by `AutoMigrate` with no hand-written migration. Reads group rows by
  `Direction`.
- **Alternatives:** (a) two separate `input_modalities` / `output_modalities`
  tables — larger migration and two preloads for no gain; (b) a JSON blob on the
  parent row — regresses the normalized model the project deliberately adopted;
  (c) two CSV columns — loses the association and per-value rows.

### D2 — `ModelInfo` exposes `InputModalities` and `OutputModalities`

Replace `ModelInfo.Modalities` with separate
`InputModalities` / `OutputModalities` fields and drop the merged field.

- **Why:** direction is known at parse time in every client; keeping a merged
  field invites re-introducing the collapse. Capability derivation switches to the
  **input** list (an `image` input ⇒ `vision`).
- **Alternatives:** keep `Modalities` as a derived union for convenience —
  rejected as redundant and the source of the original defect.

### D3 — DTO and API expose `inputModalities` / `outputModalities`

Replace `LlmModelDTO.modalities` with `inputModalities` and `outputModalities`
(camelCase, matching the Go JSON tags and webapp TS types). This is a **BREAKING**
field change; the only consumer is the webapp.

- `domainModelsToDTOs` groups persisted rows by `Direction`.
- `handleGetLlmModels` maps `ModelInfo.InputModalities` / `OutputModalities` for
  the live path and persists both directions for the cached path.

### D4 — Legacy cached rows are rebuilt on refresh, not backfilled

`AutoMigrate` adds `direction` with a default; pre-existing rows cannot be split
retroactively because their direction was already discarded. The existing
`ReplaceProviderModels` refresh path (the provider "refresh" action) deletes and
re-inserts rows with correct direction.

- **Why:** a one-off data migration would have to guess direction from modality
  names (unknowable) and adds risky complexity for a cached, refreshable dataset.

### D5 — Webapp renders two rows; i18n keys split

[`ModelDetailsDialog`](../../../webapp/src/components/settings/ModelDetailsDialog.tsx:60)
renders separate "Input modalities" and "Output modalities" rows (em dash when
empty). Replace `llm_model_details.modalities` with
`llm_model_details.inputModalities` + `llm_model_details.outputModalities` in
`translations.en.ts` and `translations.ru.ts` together.

### D6 — Contract-first with regenerated types

Update the `LlmModelDTO` schema in
[`core.yaml`](../../../docs/api-contracts/core.yaml:2965), then `make generate-types`
to refresh [`webapp/src/types/api.ts`](../../../webapp/src/types/api.ts:2144), and
hand-align [`types/index.ts`](../../../webapp/src/types/index.ts:676).

## Risks / Trade-offs

- **Cached rows show only input until refreshed** → Mitigation: the provider model
  "refresh" rebuilds direction; call this out in the details and release notes.
- **BREAKING removal of `modalities`** → Mitigation: the webapp is the sole
  consumer; grep confirms no other reader; update the dialog, types, and tests in
  the same change.
- **GORM ordering of grouped rows** → The association is unordered; the webapp
  joins in the order returned. Acceptable — the lists are short and order is not
  contractual.

## Migration Plan

1. Ship backend + contract + webapp together (single PR on
   `feature/llm-model-modality-split`).
2. `AutoMigrate` adds the `direction` column on startup; no manual SQL.
3. Operators open each provider's model list once (or press refresh) to rebuild
   directional rows.
4. Rollback: revert the code; the extra `direction` column is inert and harmless.

## Evidence Plan

Existing scenario IDs keep their bindings; the evidence for the four changed
scenarios is updated in place (selector or assertions), so each is mutation-checked.

| Scenario (scope `current`) | Runner | Selector |
|---|---|---|
| `llm-model-metadata.metadata-exposed` | go | `./internal/interfaces/dto::TestLlmModelDTOMetadataFields` |
| `llm-model-metadata.alibaba-modalities-parsed` | go | `./internal/infrastructure/llm::TestAlibabaModalitiesParsed` |
| `llm-model-metadata.deepseek-modalities-parsed` | go | `./internal/infrastructure/llm::TestDeepSeekModalitiesParsed` |
| `llm-model-metadata.model-details-window` | vitest | `webapp/src/components/settings/ModelDetailsDialog.test.tsx::ModelDetailsDialog > shows context length, input modalities, output modalities, capabilities and pricing` |
| `llm-model-metadata.ollama-vision-modality` | go | `./internal/infrastructure/llm::TestOllamaVisionModality` (unchanged) |

The concrete registry entries for `.focused-spec/evidence.yaml` are written during
the apply phase, as required by the focused-spec workflow.
