# Tasks

## 1. faces service scaffolding

- [x] 1.1 Create `backend/faces` FastAPI project (`pyproject.toml`, dependencies on `fastapi`, `uvicorn`, `insightface`, `onnxruntime`, `hdbscan`, `numpy`, `pillow`) with an app skeleton exposing `GET /faces/health`; verify with `python -m pytest` in `backend/faces` and a manual `curl` of `/faces/health` returning 200
- [x] 1.2 Add `backend/faces/Dockerfile`, `backend/faces/README.md`, and registry registration via the `/registry/v1/*` REST facade with self-healing re-registration; verify the image builds (`docker build`) and `/faces/health` reports ready

## 2. faces service ML endpoints

- [x] 2.1 Implement `POST /faces/detect` (SCRFD-10G via InsightFace) returning face boxes + confidence, and load the model once at startup; verify a pytest over a fixture image asserts at least one box is returned
- [x] 2.2 Implement `POST /faces/embed` (ArcFace w600k_r50) returning a 512-d vector per face crop; verify a pytest asserts the returned vector length equals 512
- [x] 2.3 Implement `POST /faces/cluster` (HDBSCAN, cosine metric, configurable `minClusterSize`) returning a label per embedding with `-1` for noise; verify a pytest feeds two same-identity and one different embedding and asserts the same-identity pair share a label

## 3. core domain and persistence

- [x] 3.1 Add `Person`, `Face`, and `FaceEmbedding` (`halfvec`) GORM models with camelCase JSON tags and AutoMigrate registration; verify `cd backend/core && go build ./... && go test ./internal/application/... -count=1`
- [x] 3.2 Add `PersonRepository` and `FaceRepository` interfaces plus their GORM implementations and in-memory SQLite tests; verify `cd backend/core && go test ./internal/application/... -count=1`

## 4. core faces client and DI

- [x] 4.1 Add a typed `faces` REST client (detect/embed/cluster) with context timeouts, registry resolution and bootstrap fallback, plus its mock; verify `cd backend/core && go test ./internal/application/... -count=1`
- [x] 4.2 Wire the new client, repositories, and manager into `cmd/server/wire.go`, run `go generate ./...`, and commit both `wire.go` and the regenerated `wire_gen.go`; verify `cd backend/core && go build ./... && go test ./internal/application/... -count=1`

## 5. core background job

- [x] 5.1 Add `ScheduleKind` `face_recognition` and its `DefaultSchedule` (enabled, weekdays 22:00–07:00 window); verify `cd backend/core && go test ./internal/domain/... -count=1`
- [x] 5.2 Implement `FaceRecognitionManager` (detect → embed → persist → cluster → assign `personID`, resumable over unprocessed images) following the `TagScanManager` pattern; verify `cd backend/core && go test ./internal/application/... -count=1`

## 6. core API, settings, and i18n

- [x] 6.1 Add handlers for `GET /api/people`, `GET /api/people/:id/photos` (gallery-style pagination), `GET /api/faces/status` (running/processed/total), and `POST /api/faces/start` / `POST /api/faces/stop` with session auth and CSRF on mutations; verify `cd backend/core && go test ./internal/application/... -count=1`
- [x] 6.2 Add face recognition settings (enabled flag + schedule) to the existing settings DTO and persistence path; verify `cd backend/core && go test ./internal/application/... -count=1`
- [x] 6.3 Add the `Msg*` constants and both `en`/`ru` locale entries for all new core strings together; verify both `en.json` and `ru.json` are updated and `cd backend/core && go test ./internal/application/... -count=1`

## 7. OpenAPI contracts

- [x] 7.1 Extend `docs/api-contracts/core.yaml` with the new endpoints/DTOs and add `docs/api-contracts/faces.yaml`; run `make generate-types`; verify the regenerated `webapp/src/types/api.ts` field names match the Go JSON tags exactly

## 8. webapp API client

- [x] 8.1 Add typed API helpers in `webapp/src/api/endpoints.ts` (people list, person photos, face status/start/stop, settings) using the generated types; verify `cd webapp && npm run lint && npx tsc -b`

## 9. webapp People UI and settings

- [x] 9.1 Add a lazy-loaded `PeopleTab` rendering a person grid (shared `Tile`/`EmptyState`/`Skeleton` primitives) plus a person detail photo view; verify `cd webapp && npm run lint && npx tsc -b && npm test`
- [x] 9.2 Add a face recognition settings section to `AdminAnalysisTab` (enable toggle, shared `ScheduleEditor`, Start/Stop, progress badge) following the tag-scan pattern; verify `cd webapp && npm run lint && npx tsc -b && npm test`
- [x] 9.3 Add all new user-facing strings to `translations.en.ts` and `translations.ru.ts` together; verify `cd webapp && npm test` (translations parity) and `npm run lint && npx tsc -b`

## 10. focused-spec evidence

- [x] 10.1 Bind the `planned:` entries in `.focused-spec/evidence.yaml` under `scopes.current` — `go::` selectors for the core scenarios and `vitest::` selectors for the People tab scenarios; verify `node_modules/.bin/focused-spec validate --scope current --strict` reports no missing or duplicate bindings
- [x] 10.2 Execute the evidence: `node_modules/.bin/focused-spec run --scope current`; verify it passes with no remaining `planned:` references
- [x] 10.3 Mutation-check every new/changed evidence entry (revert the behavior hunk → observe FAIL on the behavioral assertion → restore → observe PASS) and record each fail → restore → pass triple

## 11. Integration checks

- [x] 11.1 Run the full backend check suite (`cd backend/core && go vet ./... && go build ./... && go test ./internal/application/... -count=1`), the faces pytest suite (`cd backend/faces && python -m pytest`), and the webapp suite (`cd webapp && npm run lint && npx tsc -b && npm test && npm run build`); verify all green
- [x] 11.2 Update `docs/agent/go-services.md` service table with the `faces` service and its checks; verify the documented commands run as written

## Workflow follow-up

- Sync the delta spec into `openspec/specs/face-recognition/spec.md` (openspec-sync-specs) and verify `openspec validate --specs`.
- Archive the change into `openspec/changes/archive/YYYY-MM-DD-face-recognition/` after review.
