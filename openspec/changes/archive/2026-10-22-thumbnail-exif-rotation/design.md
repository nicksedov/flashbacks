# Design

## Context

See `proposal.md` for motivation. The active thumbnail path is
`thumbnail.Service.generateThumbnail` in `backend/core/internal/application/thumbnail/service.go`:
it opens the file, decodes it with `image.Decode`, resizes with
`github.com/disintegration/imaging`, then encodes to WebP/JPEG/PNG. `image.Decode`
does not apply EXIF orientation, so rotated photos are cached sideways.

Orientation is available as the EXIF `Orientation` tag (values 1–8), the same value
the exif service already persists as `ImageMetadata.Orientation`. Core has no
access to the exif database (service boundary), and the path-based
`GET /exif/metadata` re-runs `exiftool` per call, which is too expensive for the
scan-time thumbnail hot path. `Service.GetOrGenerate` serializes cache-miss
generation under a write lock, so generation is effectively sequential.

## Goals / Non-Goals

**Goals:**

- Produce upright thumbnails for photos whose EXIF `Orientation` is `3`, `6` or `8`.
- Correct already-cached sideways thumbnails once, without a manual cache reset.
- Keep the change inside the core service: no new dependencies, no inter-service
  REST calls, no contract or schema changes.

**Non-Goals:**

- Mirrored orientations `2`, `4`, `5`, `7` (flip/transpose) are rendered as decoded.
- EXIF `Orientation` inside WebP files is not parsed.
- The unused in-memory `imaging.GenerateThumbnail` helper is not migrated.
- No change to thumbnail dimensions, format, quality, or cache layout/keys.

## Decisions

### D1 — Read orientation locally from the source file

Add an unexported reader in the `thumbnail` package that extracts the EXIF
`Orientation` tag from the file bytes:

- JPEG: walk segment headers from `SOI`; on the `APP1` marker whose payload starts
  with `Exif\0\0`, parse the TIFF header (`II`/`MM` byte order) and IFD0 entries,
  returning the `SHORT` value of tag `0x0112`.
- PNG: walk chunks; on `eXIf`, parse the same TIFF/IFD0 structure.
- Anything else, or no tag found: return `0` (treated as "no rotation").

Rationale: no network call, no `exiftool` invocation, deterministic and testable
with generated fixtures; works even when the exif service is unavailable.
Alternatives considered: (a) call `ExifClient.ExtractMetadata` — adds an HTTP
round trip and exiftool run per thumbnail and a runtime dependency on the exif
service; (b) add a third-party EXIF library — rejected to avoid a new dependency.

Reading is done from the already-open `*os.File` by scanning only JPEG segment
headers (stopping at `SOS`) or PNG chunk headers, so the full file is not buffered;
the file is then seeked back to `0` before `image.Decode`.

### D2 — Rotate before resize and recompute bounds afterwards

In `generateThumbnail`, after decode:

1. `orientation := readExifOrientation(file)`
2. apply the rotation (D3) to the decoded `image.Image`
3. recompute `bounds` from the rotated image
4. resize as today (`MaxSize` on the longer side), encode as today

Rationale: rotating first makes the longer-side selection and the resulting
thumbnail dimensions reflect the upright image (a landscape photo with orientation
`6`/`8` becomes a portrait thumbnail). Rotating after resize would preserve the
wrong aspect ratio and cost an extra resample.

### D3 — Orientation-to-rotation mapping

| Orientation | Transform |
|---|---|
| `1` / `0` (absent) | none |
| `3` | `imaging.Rotate180` |
| `6` | `imaging.Rotate90` (90° clockwise) |
| `8` | `imaging.Rotate270` (270° clockwise = 90° counter-clockwise) |
| `2`,`4`,`5`,`7` | none (non-goal; rendered as decoded) |

### D4 — Startup backfill manager

Add `backend/core/internal/application/thumbnail/backfill.go` with a
`RotationBackfill` that:

- depends on `ThumbnailProvider` and a narrow `ImageFileLister` interface
  (`ListAll() ([]domain.ImageFile, error)`), defined in the `thumbnail` package to
  avoid importing the exif layer or `gorm` into application code;
- is started from `App.Init` in its own goroutine with the application context, so
  it never blocks startup;
- iterates indexed files sequentially (the service already serializes generation),
  checks `ctx.Done()` between items, reads orientation locally (D1), and for
  `3/6/8` calls `provider.Invalidate(path)` then `provider.GetOrGenerate(path)`;
- tolerates per-file failures (logs and continues) and skips cleanly when the
  provider is nil or disabled.

`repository.ImageFileRepository` gains `ListAll() ([]domain.ImageFile, error)` and
its GORM implementation; the backfill constructor accepts it through the narrow
`ImageFileLister` interface.

Alternatives considered: put the logic in `BackgroundSyncManager` — rejected, that
manager runs on a schedule and is unrelated to a one-time cache correction; run in
`main` instead of `App.Init` — rejected, `App.Init` already owns service startup.

### D5 — Idempotency marker per cache directory

After a successful pass the backfill writes `<CacheDir>/.orientation-backfill.v1`
and skips on later starts when that marker exists. `CacheDir` comes from
`provider.GetStats()`. When the cache directory changes (new path) or the marker is
absent (fresh cache), the backfill runs once for that cache.

Rationale: the marker is O(1), needs no schema change, and naturally re-runs for a
new/empty cache. Alternatives considered: a per-file DB column/flag — adds a
migration and service-boundary coupling for a one-time correction; always running —
wasteful regeneration of every rotated thumbnail on each start.

### D6 — Wire DI changes

`ProvideThumbnailProvider` is unchanged. A new
`ProvideRotationBackfill(repo repository.ImageFileRepository, provider thumbnail.ThumbnailProvider)`
is added to the DI provider set and the `App` struct gains a `RotationBackfill`
field. Because the graph changes, run `go generate ./...` and commit both
`wire.go` and the regenerated `wire_gen.go`.

No inter-service REST communication is introduced: orientation is read locally and
the exif/ocr services are not called.

### D7 — Evidence plan

Scenarios bind to Go tests in `./internal/application/thumbnail` (runner `go`, whose
`commandArgs` already set `-C backend/core`), using programmatically generated
JPEG fixtures with an injected EXIF `Orientation`:

| Scenario id | Planned evidence selector |
|---|---|
| `thumbnails.orientation-6-rotated` | `go::./internal/application/thumbnail::TestGenerateThumbnail_Orientation6SwapsDimensions` |
| `thumbnails.orientation-8-rotated` | `go::./internal/application/thumbnail::TestGenerateThumbnail_Orientation8SwapsDimensions` |
| `thumbnails.orientation-3-rotated` | `go::./internal/application/thumbnail::TestGenerateThumbnail_Orientation3Rotates180` |
| `thumbnails.orientation-1-unrotated` | `go::./internal/application/thumbnail::TestGenerateThumbnail_Orientation1Unrotated` |
| `thumbnails.startup-backfill-rotated` | `go::./internal/application/thumbnail::TestRotationBackfill_RegeneratesRotated` |
| `thumbnails.startup-backfill-idempotent` | `go::./internal/application/thumbnail::TestRotationBackfill_Idempotent` |
| `thumbnails.startup-backfill-unrotated-skipped` | `go::./internal/application/thumbnail::TestRotationBackfill_SkipsUnrotated` |

Registry entries are added during apply (planning defers the registry). The
generation tests decode the produced thumbnail and assert observable geometry/pixel
outcomes, not internal calls.

## Risks / Trade-offs

- **Full-file read for orientation** → the JPEG scan stops at `SOS` and the PNG scan
  stops at the first non-`eXIf` chunk boundary, so only headers are read; the file is
  seeked back before decode.
- **Backfill cost on first start** → runs once per cache, sequentially, and only
  regenerates rotated files; failures are logged and do not abort the service.
- **Marker can get stale if the cache is partially restored** → a new cache path or a
  missing marker triggers a fresh pass; a manual full re-run is possible by deleting
  the marker.
- **Malformed EXIF** → the reader returns `0` on any parse error and generation
  proceeds unrotated, matching current behaviour.

## Migration Plan

1. Deploy the new core image. On first start the backfill runs once and rewrites the
   cached thumbnails of orientation `3/6/8` files, then writes the marker.
2. No database or API migration. Rollback: redeploy the previous core image; the
   regenerated thumbnails stay valid (they are upright and still decode normally), and
   the marker file is harmless to the old code.
