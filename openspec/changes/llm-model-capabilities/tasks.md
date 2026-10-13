# Tasks

## 1. Backend — standardized vocabulary and Alibaba native-code mapping

- [ ] 1.1 Expand the standardized capability vocabulary and the native-code mapping in `backend/core/internal/infrastructure/llm/metadata.go` per design D1–D2 (add `reasoning`, `video_generation`, `text_to_speech`, `speech_recognition`, `realtime`, `omni`, `world_model`; map `IG`, `VG`, `World-Model`, `TTS`, `ASR`, `Realtime-*`, `*Omni`, `Reasoning`). Verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [ ] 1.2 Rework `alibabaCatalogModelToInfo` in `backend/core/internal/infrastructure/llm/alibaba_client.go` per design D3 so capabilities come from native codes, the modality derivation is used only as a fallback, and `vision` is added only for image-input + text-output models. Verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [ ] 1.3 Add the eight Alibaba capability tests named in design.md's evidence plan to `backend/core/internal/infrastructure/llm/alibaba_client_test.go`, comparing capability sets order-insensitively. Verify with `cd backend/core && go test ./internal/infrastructure/llm/... -count=1`.
- [ ] 1.4 Run the full core suite and fix any failures. Verify with `cd backend/core && go test ./... -count=1`.

## 2. Webapp — capability vocabulary, localized display, and i18n

- [ ] 2.1 Add `webapp/src/lib/llm-capabilities.ts` defining the twelve-value capability union, its display order, and the i18n label key per value. Verify with `cd webapp && npm run lint && npx tsc -b`.
- [ ] 2.2 Render localized capability labels in `webapp/src/components/settings/ModelDetailsDialog.tsx`, falling back to the raw value for unknown capabilities. Verify with `cd webapp && npm test -- ModelDetailsDialog`.
- [ ] 2.3 Extend `webapp/src/components/tabs/AdminLlmProvidersTab.tsx` (capability union, label map, and the `inferCapabilities` filter) to accept all standardized values. Verify with `cd webapp && npm run lint && npx tsc -b`.
- [ ] 2.4 Add the twelve capability labels to `webapp/src/i18n/translations.en.ts` and `webapp/src/i18n/translations.ru.ts` together. Verify with `cd webapp && npm test -- translations`.
- [ ] 2.5 Add the webapp tests named in design.md's evidence plan: the ModelDetailsDialog new-capability label test and the `filterModelsForInstrument` non-text exclusion test. Verify with `cd webapp && npm test -- ModelDetailsDialog llm-model-filter`.
- [ ] 2.6 Run the full webapp checks and fix any failures. Verify with `cd webapp && npm run lint && npx tsc -b && npm test`.

## 3. Evidence integration (apply phase)

- [ ] 3.1 Add registry entries for the ten new scenario IDs to `.focused-spec/evidence.yaml` using the runner/selector table in design.md, and confirm `node_modules/.bin/focused-spec validate --scope current` reports no missing or duplicate bindings.
- [ ] 3.2 Execute the new evidence: `node_modules/.bin/focused-spec run --scope current` passes with no remaining `planned:` references.
- [ ] 3.3 Mutation-check every new or changed evidence entry (revert the behavior hunk, observe FAIL, restore, observe PASS) and record the fail → restore → pass triple.

## Workflow follow-up

- Sync the delta specs into `openspec/specs/**`, preserving the existing `focused-spec:id` markers on unchanged scenarios and adding the new ones; verify with `openspec validate --specs`.
- Archive the change into `openspec/changes/archive/YYYY-MM-DD-llm-model-capabilities/` after review, then verify with `openspec list`.
