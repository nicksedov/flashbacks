## Why

VL (vision-language) and LLM-based OCR tasks currently share a single `vl` LLM instrument, so the model/provider chosen for image description, tags, and visual Q&A is also forced onto text recognition. Users need to point OCR at a different model (e.g. a cheaper or OCR-specialized model) without affecting the VL tasks.

## What Changes

- Add a new LLM instrument type `ocr` alongside `chat`, `vl`, `embedding`, `image_edit`.
- Route LLM-based OCR recognition through the new `ocr` instrument:
  - `POST /api/llm/recognize` (async LLM OCR) resolves its client from the `ocr` instrument instead of `vl`.
  - The `recognizeText` AI action resolves its client from the `ocr` instrument; `describe`, `tags`, and `askQuestion` keep using `vl`.
- Add an "OCR LLM Settings" card in the Analysis tab UI, placed directly below the "VL LLM Settings" card, using the same provider/model selector pattern.
- Update the VL card description so it no longer lists OCR (OCR is now configured separately).
- Extend the API contract: add `ocr` to the `LlmInstrumentDTO.type` enum in [`docs/api-contracts/api-service.yaml`](docs/api-contracts/api-service.yaml:2828).
- Seed a default `ocr` instrument during bootstrap so existing environments get a working OCR instrument after upgrade.
- Add en/ru i18n keys for the new card (label + description) and update the VL description in both locales.

## Capabilities

### New Capabilities

- `llm-settings`: per-instrument LLM model/provider assignment (chat, vl, ocr, embedding, image_edit) and its admin UI in the Analysis tab.

### Modified Capabilities

<!-- None. No existing capability spec covers LLM instrument settings. -->

## Non-goals

- No change to the Tesseract OCR service (`backend/ocr`) or its `/classify` endpoint — this is about LLM-based OCR only.
- No change to `chat`, `embedding`, or `image_edit` instrument behavior.
- No changes to OCR/VL prompts, model selection heuristics, or provider capability inference.
- No database schema migration: instruments are rows keyed by a unique `type` column; the new `ocr` row is created via the existing seed path.

## Impact

- **api-service**: [`domain/media.go`](backend/api-service/internal/domain/media.go:251) (new `InstrumentOCR` constant), [`handlers_llm.go`](backend/api-service/internal/interfaces/handler/handlers_llm.go:16) (map `"ocr"` string + route recognize/recognizeText), [`helpers/llm.go`](backend/api-service/internal/interfaces/handler/helpers/llm.go:44) (new `CreateOCRClient`), [`database.go`](backend/api-service/internal/infrastructure/database/database.go:106) and [`testutil/testdb.go`](backend/api-service/internal/testutil/testdb.go:68) (seed OCR instrument).
- **webapp**: [`types/index.ts`](webapp/src/types/index.ts:575) (add `"ocr"` to `LlmInstrumentType`), [`AdminAnalysisTab.tsx`](webapp/src/components/tabs/AdminAnalysisTab.tsx:47) (new card + labels), [`AdminLlmProvidersTab.tsx`](webapp/src/components/tabs/AdminLlmProvidersTab.tsx:54) (usage badge entry), [`translations.en.ts`](webapp/src/i18n/translations.en.ts:662) and [`translations.ru.ts`](webapp/src/i18n/translations.ru.ts:572) (new keys).
- **Contract**: [`docs/api-contracts/api-service.yaml`](docs/api-contracts/api-service.yaml:2828) — `LlmInstrumentDTO.type` enum gains `ocr`.
