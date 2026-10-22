# Proposal

## Why

Thumbnails are produced by decoding the original file and resizing it, which
ignores the EXIF `Orientation` tag. Photos captured in portrait or rotated
in-camera therefore appear sideways in the gallery, trash, duplicate and geo
views, and already-cached thumbnails for such photos are stored with the wrong
orientation.

## What Changes

- Apply the EXIF Orientation rotation (values `3`, `6`, `8`) to the decoded
  image before resizing, so the thumbnail is generated from the photo rotated to
  its correct upright angle.
- Read the orientation locally from the source file (JPEG APP1 `Exif` segment,
  PNG `eXIf` chunk) — no cross-service calls and no new dependencies.
- Add a startup background backfill that scans indexed gallery files and
  regenerates cached thumbnails for files whose orientation is `3`, `6` or `8`,
  correcting thumbnails that were cached before this change.
- Make the backfill idempotent per cache directory (a version marker) so it does
  not regenerate the same thumbnails on every service start.

## Capabilities

### New Capabilities

- `thumbnails`: server-side thumbnail generation and caching, including
  EXIF-orientation-aware rendering and a startup correction pass over the cache.

### Modified Capabilities

- (none)

## Non-goals

- Mirrored EXIF orientations `2`, `4`, `5` and `7` (flip/transpose) are not
  applied; only the pure rotations `3`, `6` and `8` are handled.
- EXIF `Orientation` embedded in WebP files is not parsed.
- The unused in-memory `imaging.GenerateThumbnail` helper is left unchanged.
- No new REST endpoints and no OpenAPI/MCP contract changes; no database schema
  changes.

## Impact

- Code: `backend/core/internal/application/thumbnail/*` (orientation reader,
  generation, startup backfill), `backend/core/cmd/server/app.go` (startup hook),
  and the core DI graph (`backend/core/internal/infrastructure/di/*` plus the
  regenerated `wire_gen.go`).
- Behaviour: gallery, trash, duplicate and geo thumbnails show rotated photos
  upright; the first start after deploy regenerates the affected cached
  thumbnails.
- Contracts/dependencies: none changed.
