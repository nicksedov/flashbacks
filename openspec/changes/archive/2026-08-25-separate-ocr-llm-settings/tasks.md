## 1. Backend: domain + factory

- [x] 1.1 Add `InstrumentOCR InstrumentType = "ocr"` constant to [`domain/media.go`](backend/api-service/internal/domain/media.go:251)
- [x] 1.2 Add `CreateOCRClient(c *gin.Context) (llm.Client, domain.LlmProvider, domain.LlmInstrumentSettings, bool)` to [`helpers/llm.go`](backend/api-service/internal/interfaces/handler/helpers/llm.go:44), delegating to `createClientByInstrument(c, domain.InstrumentOCR)`
- [x] 1.3 Add `case "ocr": return domain.InstrumentOCR` to `instrumentTypeFromString` in [`handlers_llm.go`](backend/api-service/internal/interfaces/handler/handlers_llm.go:17)

## 2. Backend: routing

- [x] 2.1 Switch [`handleLlmRecognize`](backend/api-service/internal/interfaces/handler/handlers_llm.go:397) to call `s.llmFactory.CreateOCRClient` instead of `CreateVLClient`
- [x] 2.2 In [`handleAiAction`](backend/api-service/internal/interfaces/handler/handlers_llm.go:679), branch on `req.Action`: use `CreateOCRClient` for `recognizeText` and `CreateVLClient` for `describe`/`tags`/`askQuestion`

## 3. Backend: seed + tests

- [x] 3.1 Add an idempotent "ensure OCR instrument" step in [`database.go`](backend/api-service/internal/infrastructure/database/database.go:91) that creates an `ocr` instrument copied from the current `vl` instrument (ProviderID + Model) when none exists
- [x] 3.2 Add an `ocr` instrument seed alongside the existing seeds in [`testutil/testdb.go`](backend/api-service/internal/testutil/testdb.go:68)
- [x] 3.3 Add/update unit tests covering `instrumentTypeFromString("ocr")`, `CreateOCRClient` resolution, and the `recognizeText` routing branch
- [x] 3.4 Run `cd backend/api-service && go test ./internal/application/... -count=1`

## 4. API contract

- [x] 4.1 Add `ocr` to the `LlmInstrumentDTO.type` enum in [`docs/api-contracts/api-service.yaml`](docs/api-contracts/api-service.yaml:2828)
- [x] 4.2 Run `make generate-types` and verify `webapp/src/types/api.ts` reflects the new enum value

## 5. Webapp: types + i18n

- [x] 5.1 Add `"ocr"` to the `LlmInstrumentType` union in [`webapp/src/types/index.ts`](webapp/src/types/index.ts:575)
- [x] 5.2 Add `llm_ocr.ocrSettings` and `llm_ocr.ocrSettingsDescription` keys to both [`translations.en.ts`](webapp/src/i18n/translations.en.ts:662) and [`translations.ru.ts`](webapp/src/i18n/translations.ru.ts:572); update the VL description in both locales to remove "OCR"; add `llm_providers.usageShortOCR` to both locales

## 6. Webapp: UI

- [x] 6.1 Add `ocr` entries to `INSTRUMENT_LABELS` and `INSTRUMENT_DESCRIPTIONS` in [`AdminAnalysisTab.tsx`](webapp/src/components/tabs/AdminAnalysisTab.tsx:47) and render the `ocr` card directly below the `vl` card
- [x] 6.2 Add an `ocr` entry to `INSTRUMENT_ICONS` in [`AdminLlmProvidersTab.tsx`](webapp/src/components/tabs/AdminLlmProvidersTab.tsx:54)
- [x] 6.3 Run `cd webapp && npm run lint && npx tsc -b` and fix all errors
