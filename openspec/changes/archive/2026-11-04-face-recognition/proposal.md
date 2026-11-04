# Proposal

## Why

Flashbacks can already find duplicates, tag images, and read text/EXIF, but it
has no notion of *who* is in a photo. Users cannot answer "show me every photo
of this person", which is the core Photoprism-style browsing experience. Adding
face recognition turns the flat image index into a person-centric library:
background clustering groups the same face across the collection, and a new
People view lets the user browse one person's photos.

## What Changes

- Introduce a dedicated `faces` microservice (`backend/faces`) that runs the
  ML pipeline over REST: SCRFD face detection, ArcFace face embeddings, and
  HDBSCAN clustering of embeddings into people. It registers in the service
  registry like `exif`/`ocr`.
- Add a new core background job `FaceRecognitionManager` driven by the unified
  `schedules` table (new `ScheduleKind` `face_recognition`), following the
  existing `TagScanManager` pattern: it fetches unprocessed images, calls the
  `faces` service for detection + embedding, clusters the embeddings, and
  persists people and face-to-person assignments.
- Add core domain models and their migration: `faces` (detected face box +
  embedding reference), `face_embeddings` (pgvector `halfvec`), and `people`
  (a face cluster = a person).
- Add core REST endpoints: list people, list one person's photos, and face
  recognition job status / start / stop; expose face recognition settings next
  to the existing analysis settings.
- Extend the OpenAPI contract (`docs/api-contracts/core.yaml`, new
  `docs/api-contracts/faces.yaml`) and regenerate `webapp/src/types/api.ts`.
- Add a lazy-loaded People tab (person grid) plus a person detail photo view,
  and a face recognition settings section in the Analysis admin tab.
- Add en/ru translations for every new user-facing string.

## Capabilities

### New Capabilities

- `face-recognition`: background classification of photos by the faces found on
  them (detection, embedding, clustering into people), the person-browsing API,
  and the People UI plus its job settings.

### Modified Capabilities

<!-- none -->

## Non-goals

- No live video / camera face recognition — photos only.
- No automatic face *identification* against an external identity database; the
  system only groups similar faces into unnamed people (clusters), which the
  user can later rename.
- No face matching across users/tenants; the whole library is a single
  namespace like the rest of the gallery.
- No on-device / web-worker inference; all ML runs in the `faces` service.
- No GPU-specific orchestration in this change; CPU ONNX Runtime is the
  baseline, with GPU as a documented future option.

## Impact

- **Backend core** (`backend/core`): new domain models, repository interfaces,
  `FaceRecognitionManager`, handlers, settings, schedule kind, Wire graph
  change (`wire.go` + regenerated `wire_gen.go`), i18n en/ru.
- **New service** `backend/faces`: Python/FastAPI service running InsightFace
  (SCRFD detection + ArcFace embeddings via ONNX Runtime) and HDBSCAN
  clustering, REST surface and registry registration.
- **Webapp**: new People tab, person detail view, Analysis tab settings section,
  API client helpers, translations.
- **Contracts**: `docs/api-contracts/core.yaml` extended, new
  `docs/api-contracts/faces.yaml`; `make generate-types` required.
- **Docs/ops**: `docs/agent/go-services.md` service table, deployment skills and
  Docker assets for the new `faces` service, `backend/core/.env.example`.
