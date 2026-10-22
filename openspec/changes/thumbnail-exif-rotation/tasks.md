# Tasks

## 1. Local EXIF orientation reader

- [x] 1.1 Add `backend/core/internal/application/thumbnail/exif_orientation.go` implementing `readExifOrientation(*os.File) int` that parses the JPEG `APP1`/`Exif` TIFF IFD0 for tag `0x0112` and the PNG `eXIf` chunk, returning `0` on any parse error or absent tag; verify `cd backend/core && go build ./...` succeeds.
- [x] 1.2 Add parser unit tests (JPEG orientations `1`,`3`,`6`,`8`, a PNG `eXIf` file, and a file without EXIF) with a fixture helper that writes a JPEG with an injected orientation; verify `cd backend/core && go test ./internal/application/thumbnail -run TestReadExifOrientation -count=1` passes.

## 2. Orientation-aware thumbnail generation

- [x] 2.1 Apply the rotation in `thumbnail.Service.generateThumbnail` after decode and before resize, recompute bounds from the rotated image, and map `3`→`Rotate180`, `6`→`Rotate90`, `8`→`Rotate270` (other values unchanged); verify `cd backend/core && go test ./internal/application/thumbnail -count=1` passes.
- [x] 2.2 Add generation tests with injected-orientation JPEG fixtures asserting observable outcomes: dimensions swapped for `6` and `8`, 180° pixel rotation with preserved dimensions for `3`, and unchanged output for `1`/no EXIF; verify `cd backend/core && go test ./internal/application/thumbnail -run TestGenerateThumbnail_Orientation -count=1` passes.
- [x] 2.3 Document orientation-aware thumbnail generation in `backend/core/README.md` (thumbnail cache section) and verify the described behaviour matches the implemented tests.

## 3. Startup orientation backfill

- [x] 3.1 Add `backend/core/internal/application/thumbnail/backfill.go` with an `ImageFileLister` interface (`ListAll() ([]domain.ImageFile, error)`) and a `RotationBackfill` that iterates files sequentially, checks context cancellation, reads orientation locally, and calls `Invalidate` + `GetOrGenerate` for `3/6/8`; add `ListAll()` to `ImageFileRepository` and its GORM implementation; verify `cd backend/core && go build ./...` succeeds.
- [x] 3.2 Add the `.orientation-backfill.v1` marker under the active cache directory after a completed pass and skip when it exists, resolving the cache directory from `provider.GetStats()`; verify the backfill idempotency test passes.
- [x] 3.3 Wire `RotationBackfill` into the `App` struct, start it from `App.Init` in a goroutine with the application context, add a DI provider, run `go generate ./...`, and commit `wire.go` + regenerated `wire_gen.go`; verify `cd backend/core && go build ./...` succeeds and `wire_gen.go` contains the new provider call.
- [x] 3.4 Add backfill tests: regenerates cached thumbnails for orientation `3/6/8`, does nothing on a second run when the marker exists, and leaves orientation `1`/no-EXIF files untouched; verify `cd backend/core && go test ./internal/application/thumbnail -run TestRotationBackfill -count=1` passes.
- [x] 3.5 Add a repository test for `ImageFileRepository.ListAll` on in-memory SQLite; verify `cd backend/core && go test ./internal/domain/repository -count=1` passes.

## 4. Evidence binding and verification

- [x] 4.1 Add the seven `thumbnails.*` entries to `.focused-spec/evidence.yaml` under `scopes.current` using the planned `go::./internal/application/thumbnail::<TestName>` selectors from `design.md`; verify `node_modules/.bin/focused-spec validate --scope current --strict` reports no errors.
- [x] 4.2 Run the selected evidence and observe PASS; verify `node_modules/.bin/focused-spec run --scope current` reports every `thumbnails.*` scenario as passing.
- [x] 4.3 For every new evidence reference, run the controlled mutation check: PASS → revert the implementation hunk → observe failure on the behavioural assertion → restore → PASS; verify each pair is recorded.
- [x] 4.4 Run the core service checks and verify they pass: `cd backend/core && go test ./... -count=1`.

## Workflow follow-up

- Validate: audit the branch diff against this task list and the delta spec, and resolve any blocking findings.
- Sync the `thumbnails` delta into `openspec/specs/thumbnails/spec.md` with `openspec-sync-specs`.
- Archive the change with `openspec-archive-change` once review requirements are satisfied.
