# Tasks

## 1. Normalize model metadata at the source

- [x] 1.1 Deduplicate capability codes in the Alibaba fallback inference (`inferAlibabaCapabilities`) so a model matching several rules carries one entry per capability, and verify with a unit test that a model matching both a vision token and an OCR token records `vision` once (`cd backend/core && go test ./internal/infrastructure/llm/... -count=1`)
- [x] 1.2 Derive capabilities from reported modalities when a provider supplies modalities but no capability codes, and verify with a unit test that a listing with image input and text output records `vision` (`cd backend/core && go test ./internal/infrastructure/llm/... -count=1`)
- [x] 1.3 Turn the static pricing table into a curated model-family registry (capabilities, default modalities, context length, prices, longest-token-first matching) consulted only for fields the provider left unknown, and verify with unit tests that an undescribed chat model gets no `vision` while a described family gets its defaults (`cd backend/core && go test ./internal/infrastructure/llm/... -count=1`)
- [x] 1.4 Add a single normalization step that lower-cases and deduplicates capabilities and modalities while preserving direction, drops unknown capability codes, and verify with unit tests over native-code and listing inputs (`cd backend/core && go test ./internal/infrastructure/llm/... -count=1`)
- [x] 1.5 Bind evidence for `llm-model-metadata.capabilities-deduplicated`, `llm-model-metadata.image-input-implies-vision`, `llm-model-metadata.chat-identifier-not-vision` in `.focused-spec/evidence.yaml`, then verify with `node_modules/.bin/focused-spec validate --scope current --strict` and `node_modules/.bin/focused-spec run --scope current` plus the controlled-regression mutation check

## 2. Catalog service in the application layer

- [x] 2.1 Add the catalog domain/repository port (read stored catalog, upsert merged catalog, read/write provider freshness) in `internal/domain`, and verify the package compiles with `cd backend/core && go build ./...`
- [x] 2.2 Implement the prioritized enrichment chain (native rich → native detail → registry → ID heuristics → safe default) with per-field knowledge tracking, and verify with unit tests covering every source priority (`cd backend/core && go test ./internal/application/... -count=1`)
- [x] 2.3 Implement merge-on-refresh (upsert by model id, keep unknown fields, delete only after a rich refresh) and verify with unit tests for kept vision, kept context length, and kept models missing from a degraded refresh (`cd backend/core && go test ./internal/application/... -count=1`)
- [x] 2.4 Implement TTL evaluation and the explicit refresh override with per-provider single-flight, and verify with unit tests for fresh-skip, expired-refetch, and forced-refresh cases (`cd backend/core && go test ./internal/application/... -count=1`)
- [x] 2.5 Make refresh failures return an error while the previous catalog stays readable, and verify with unit tests for reported failure and unchanged catalog (`cd backend/core && go test ./internal/application/... -count=1`)
- [x] 2.6 Bind evidence for the `llm-model-catalog.*` scenarios in `.focused-spec/evidence.yaml`, then verify with `node_modules/.bin/focused-spec validate --scope current --strict` and `node_modules/.bin/focused-spec run --scope current` plus the controlled-regression mutation check

## 3. Persistence and schema

- [x] 3.1 Add `models_fetched_at` and `models_source_rich` to the LLM provider entity and AutoMigrate list, and verify a fresh SQLite test database carries both columns (`cd backend/core && go test ./internal/domain/repository/... -count=1`)
- [x] 3.2 Replace the destructive provider-model replace with a transactional merge/upsert that tolerates duplicate capability rows (`ON CONFLICT DO NOTHING`), and verify with repository tests on in-memory SQLite that a duplicate capability no longer aborts the write (`cd backend/core && go test ./internal/domain/repository/... -count=1`)
- [x] 3.3 Persist the refresh timestamp and provenance flag in the same transaction as the rows, and verify with a repository test that a failed write leaves both the rows and the timestamp untouched (`cd backend/core && go test ./internal/domain/repository/... -count=1`)

## 4. HTTP API and wiring

- [x] 4.1 Route `GET /api/llm/models` through the catalog service, adding `refresh` as the explicit override and `fetchedAt` / `stale` / `refreshError` to the response, and verify with handler tests that the response metadata equals the stored catalog and that a failed refresh with a stored catalog answers `stale: true` (`cd backend/core && go test ./internal/interfaces/... -count=1`)
- [x] 4.2 Return a failure response when a refresh fails and no catalog exists, and verify with a handler test for the empty-catalog case (`cd backend/core && go test ./internal/interfaces/... -count=1`)
- [x] 4.3 Register the catalog service in the Wire graph, run `cd backend/core && go generate ./...`, and verify `wire_gen.go` is regenerated and `go build ./...` succeeds
- [x] 4.4 Add the `LLM_MODELS_CACHE_TTL` setting (duration, default `24h`), document it in `backend/core/.env.example` and `backend/core/README.md`, and verify the documented default parses and applies (`cd backend/core && go test ./internal/... -count=1`)
- [x] 4.5 Update `docs/api-contracts/core.yaml` for the models endpoint query and response fields, then regenerate `webapp/src/types/api.ts` with `make generate-types` and verify the generated types compile (`cd webapp && npx tsc -b`)

## 5. Analysis tab behavior

- [x] 5.1 Load each instrument's provider catalog on mount without forcing a refresh and keep the refresh button as the explicit forced action, and verify with a Vitest test that the dropdown is populated on mount and that the refresh action replaces the list (`cd webapp && npm run lint && npx tsc -b && npm test`)
- [x] 5.2 Render the stale-catalog notice from new i18n keys added to both `translations.en.ts` and `translations.ru.ts`, and verify with Vitest tests asserting "The model list may be outdated" and "Список моделей может быть устаревшим" (`cd webapp && npm run lint && npx tsc -b && npm test`)
- [x] 5.3 Bind evidence for the `llm-settings.*` scenarios in `.focused-spec/evidence.yaml`, then verify with `node_modules/.bin/focused-spec validate --scope current --strict` and `node_modules/.bin/focused-spec run --scope current` plus the controlled-regression mutation check

## 6. Integration verification

- [x] 6.1 Run the full core suite and the webapp suite and confirm both are green (`cd backend/core && go test ./... -count=1`; `cd webapp && npm test`)
- [ ] 6.2 Confirm on a running instance that `GET /api/llm/models?provider=<alias>` returns a stored catalog with `fetchedAt`, and that `qwen3.5-flash` reports its stored context length, input modalities and `vision` capability after a refresh
- [ ] 6.3 Confirm the Analysis tab OCR dropdown contains every vision-capable model of the configured provider without pressing "load models"

## Workflow follow-up

- Sync the delta specs into `openspec/specs/` with `openspec-sync-specs`, then archive the change with `openspec-archive-change`.
- Re-run `node_modules/.bin/focused-spec run --scope current` after the sync to confirm the merged specs still bind.
