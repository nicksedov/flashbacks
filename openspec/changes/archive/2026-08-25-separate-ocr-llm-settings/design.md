## Context

LLM clients are created per-instrument via [`LLMFactory`](backend/api-service/internal/interfaces/handler/helpers/llm.go:15), which reads a `llm_instrument_settings` row keyed by a unique `type` column (`chat`, `vl`, `embedding`, `image_edit`) and builds a client from that row's provider + model. Today both LLM OCR recognition and all VL actions resolve the `vl` row through [`CreateVLClient`](backend/api-service/internal/interfaces/handler/helpers/llm.go:44). See [`proposal.md`](openspec/changes/separate-ocr-llm-settings/proposal.md) for motivation.

Two backend call sites share the VL client today:

- [`handleLlmRecognize`](backend/api-service/internal/interfaces/handler/handlers_llm.go:397) — the LLM OCR path.
- [`handleAiAction`](backend/api-service/internal/interfaces/handler/handlers_llm.go:679) — handles `describe`, `tags`, `recognizeText`, `askQuestion`.

The instrument table already stores one row per type with a `uniqueIndex` on `type`, so a new instrument type is a new row, not a schema change.

## Goals / Non-Goals

**Goals:**

- Give LLM OCR its own instrument (`ocr`) that is fully independent from `vl`.
- Preserve backward compatibility for existing deployments: after upgrade, OCR keeps working with the model it previously shared with VL.
- Keep the change confined to api-service (routing + seed) and webapp (types + one card + i18n) with no new services or dependencies.

**Non-Goals:**

- No changes to the Tesseract OCR microservice (`backend/ocr`).
- No changes to `chat`, `embedding`, or `image_edit` routing.
- No changes to prompts, model-list fetching, or provider capability inference.
- No new external dependency and no DI graph change.

## Decisions

### Decision 1: Reuse the existing `llm_instrument_settings` table with a new `ocr` type

Add `InstrumentOCR InstrumentType = "ocr"` to [`domain/media.go`](backend/api-service/internal/domain/media.go:251). No GORM migration is needed — the type is a string column with a unique index, and AutoMigrate already creates the table.

- *Alternative considered*: A separate `ocr_settings` table. Rejected — it duplicates provider/model storage and breaks the uniform instrument model used everywhere else.

### Decision 2: Add `CreateOCRClient` mirroring `CreateVLClient`

Add `CreateOCRClient(c) (llm.Client, domain.LlmProvider, domain.LlmInstrumentSettings, bool)` to [`LLMFactory`](backend/api-service/internal/interfaces/handler/helpers/llm.go:15) that calls the existing `createClientByInstrument(c, domain.InstrumentOCR)`. This keeps error semantics identical (404 "settings not found" / 500 "recognition failed").

- *Alternative considered*: Parameterizing `createClientByInstrument` with a runtime string from the request. Rejected — the instrument type is a compile-time constant, and typed helpers keep call sites self-documenting.

### Decision 3: Route OCR paths to the `ocr` instrument

- `handleLlmRecognize` switches from `CreateVLClient` to `CreateOCRClient`.
- `handleAiAction` branches on `req.Action`: `recognizeText` → `CreateOCRClient`; `describe`/`tags`/`askQuestion` → `CreateVLClient`. The client selection happens once before `StartAiActionAsync`, and the rest of the flow is unchanged.

This is the only behavioral change in the request path; the `RecognizeWithLlm`/`StartAiActionAsync` service methods already receive the resolved client/provider/model and need no changes.

### Decision 4: Idempotent "ensure OCR instrument" seed for upgrades

The existing seed in [`database.go`](backend/api-service/internal/infrastructure/database/database.go:91) only runs when there are zero providers, so existing installs would never get an `ocr` row. Add a separate idempotent step that runs regardless of provider count: if no `ocr` instrument exists, create one copied from the current `vl` instrument's `ProviderID` and `Model`. This preserves the pre-upgrade behavior (OCR uses the same model VL used) while still being independently editable afterward.

- *Alternative considered*: Rely on the admin to configure OCR from the new UI card (the update handler already creates a missing row). Rejected as the sole mechanism — it would leave OCR broken immediately after upgrade until someone visits the tab.

### Decision 5: UI renders OCR as a card below VL, reusing the existing card renderer

Reuse [`renderInstrumentCard`](webapp/src/components/tabs/AdminAnalysisTab.tsx:289) for the `ocr` type and render it directly after the `vl` card in the JSX. Add `ocr` entries to `INSTRUMENT_LABELS` and `INSTRUMENT_DESCRIPTIONS`, and an `ocr` entry to `INSTRUMENT_ICONS` in [`AdminLlmProvidersTab.tsx`](webapp/src/components/tabs/AdminLlmProvidersTab.tsx:54) so the `Record<LlmInstrumentType, …>` stays exhaustive. Update the VL description strings to drop "OCR".

### Decision 6: Contract + type union updates

- Add `ocr` to the `LlmInstrumentDTO.type` enum in [`docs/api-contracts/api-service.yaml`](docs/api-contracts/api-service.yaml:2828).
- Add `"ocr"` to the `LlmInstrumentType` union in [`webapp/src/types/index.ts`](webapp/src/types/index.ts:575).

The api-service `types/api.ts` is generated from the OpenAPI doc via `make generate-types`, so the contract change and the TS type change must stay consistent.

### Decision 7: No Wire regeneration

`LLMFactory` is already constructed with `db` + `maxImageMegapixels`; adding a method does not change the constructor signature or introduce a new dependency. `wire_gen.go` does not change.

## Risks / Trade-offs

- **[Existing deployments have no `ocr` row]** → Mitigated by Decision 4 (ensure-seed copies the `vl` provider/model).
- **[OCR card initially renders empty for existing installs]** → The ensure-seed guarantees a row exists, so the card shows the inherited provider/model; the admin can then change it independently.
- **[`Record<LlmInstrumentType, …>` exhaustiveness in two tabs]** → Mitigated by adding the `ocr` entries in the same change; `npx tsc -b` fails loudly if any record is left incomplete.
- **[RecognizeText now fails if the `ocr` instrument is ever deleted]** → Acceptable and explicit (404 "settings not found"); the spec defines this as the expected behavior rather than silent VL fallback.

## Migration Plan

1. Deploy api-service with the ensure-seed step: on startup it creates the `ocr` instrument copied from `vl` if absent.
2. Deploy webapp with the new card and updated labels.
3. Rollback: api-service rollback leaves an extra `ocr` row unused (harmless); webapp rollback removes the card but existing OCR requests would again use `vl` only if the old binary is restored.
