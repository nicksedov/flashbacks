# Design

## Context

See [`proposal.md`](proposal.md) for motivation.

Current state in [`webapp/src/components/tabs/AboutTab.tsx`](../../../webapp/src/components/tabs/AboutTab.tsx:15):

- The About section already has three tabs (`about`, `license`, `thirdParty`)
  and i18n keys `about.section.thirdParty` / `about.thirdParty.description` in
  both [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:882)
  and [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:882).
- The third-party list is a static `ThirdPartyComponent[]` where each item is
  `{ name, license, url }`; `url` currently points to the project homepage.
  Eleven frontend entries are listed; the list is rendered as a single
  ungrouped `<ul>`.
- The backend is four Go modules with direct third-party dependencies:
  `backend/api-service/go.mod`, `backend/exif/go.mod`,
  `backend/ocr/go.mod`, and `tools/embeddings-builder/go.mod`
  (`backend/shared/go.mod` has none).

## Goals / Non-Goals

**Goals:**

- Append the backend's third-party Go frameworks and libraries to the existing
  single list.
- Make each backend entry link to the dependency's license text (or its
  canonical license page).
- Keep the change frontend-only and visually minimal.

**Non-Goals:**

- Generating the list at build time from `go.mod` files, or exposing it through
  an API endpoint.
- Listing indirect/transitive dependencies — only modules that appear as
  **direct** (no `// indirect`) requirements.
- Listing non-Go runtime/system components (Tesseract OCR, Leptonica, the
  `exiftool` Perl binary, PostgreSQL) — the request scoped this to Go libraries.
- Changing the existing frontend entries or their homepage links.
- Any backend code, database, or API/MCP contract change.

## Decisions

### D1 — Hand-maintained static array (not generated)

Keep `thirdPartyComponents` as a static, hand-maintained array. The set is
small and stable, and generation would add build-time tooling or an API surface
for little benefit.

- *Alternative considered:* a script/CI step that parses `go.mod` and emits the
  list. Rejected: adds tooling and a data channel the UI does not need, and the
  license/link curation still requires manual verification.

### D2 — Single combined list; reuse `url` as the link target

Append backend entries to the existing `thirdPartyComponents` array. The
existing `url` field stays the link target: for new backend entries it points to
the license, while the eleven frontend entries keep their homepage links
unchanged (recorded assumption — the user chose "keep a single combined list
and just append backend Go libraries with license links"). The license
identifier is already rendered next to every entry, so the link's meaning stays
readable.

- *Alternative considered:* add a separate `licenseUrl` field and make every
  entry link to its license. Rejected: it would change the existing frontend
  entries, which the request excludes.

### D3 — Scope of backend entries

Include every **direct** third-party module (a `require` entry without an
`// indirect` marker) across the backend `go.mod` files, de-duplicated by
module path, excluding Flashbacks-owned `github.com/flashbacks/*` modules.
Indirect transitive dependencies are excluded because they are numerous and not
deliberately selected.

- `github.com/stretchr/testify` is a direct requirement and is therefore
  included, even though it is test-only.
- Module versions differ between services (e.g. `gorm.io/gorm` v1.31.2 in
  api-service vs v1.30.0 in exif); the list de-duplicates by module path and
  does not show versions.

### D4 — License link targets

Link each backend entry to the dependency's license text at the repository
(`https://github.com/<owner>/<repo>/blob/HEAD/LICENSE`), or to a canonical
license page where the module has no single repository LICENSE. For
`golang.org/x/*` use the Go project's GitHub mirror
(`https://github.com/golang/<name>/blob/master/LICENSE`); for `gorm.io/*` use
the `go-gorm` GitHub repositories.

### D5 — No i18n changes expected

Component names and license identifiers are language-neutral, and the section
description key already exists in both locales. No new `TranslationKey` is
anticipated; if one is introduced it must be added to both
`translations.en.ts` and `translations.ru.ts` together.

### Backend component inventory

Licenses below were read from each repository's published LICENSE file on
2026-10-05. Implementation MUST re-verify each link before shipping.

| Component | Module path | License | Link |
|---|---|---|---|
| bild | `github.com/anthonynsimon/bild` | MIT | `https://github.com/anthonynsimon/bild/blob/HEAD/LICENSE` |
| go-exiftool | `github.com/barasher/go-exiftool` | **GPL-3.0** | `https://github.com/barasher/go-exiftool/blob/HEAD/LICENSE` |
| webp | `github.com/deepteams/webp` | MIT | `https://github.com/deepteams/webp/blob/HEAD/LICENSE` |
| imaging | `github.com/disintegration/imaging` | MIT | `https://github.com/disintegration/imaging/blob/HEAD/LICENSE` |
| gin-contrib/cors | `github.com/gin-contrib/cors` | MIT | `https://github.com/gin-contrib/cors/blob/HEAD/LICENSE` |
| gin | `github.com/gin-gonic/gin` | MIT | `https://github.com/gin-gonic/gin/blob/HEAD/LICENSE` |
| glebarez/sqlite | `github.com/glebarez/sqlite` | MIT | `https://github.com/glebarez/sqlite/blob/HEAD/License` |
| gocluster | `github.com/MadAppGang/gocluster` | MIT | `https://github.com/MadAppGang/gocluster/blob/HEAD/LICENSE` |
| go-sdk (MCP) | `github.com/modelcontextprotocol/go-sdk` | Apache-2.0 | `https://github.com/modelcontextprotocol/go-sdk/blob/HEAD/LICENSE` |
| openai-go | `github.com/openai/openai-go/v3` | Apache-2.0 | `https://github.com/openai/openai-go/blob/HEAD/LICENSE` |
| gosseract | `github.com/otiai10/gosseract/v2` | MIT | `https://github.com/otiai10/gosseract/blob/HEAD/LICENSE` |
| testify | `github.com/stretchr/testify` | MIT | `https://github.com/stretchr/testify/blob/HEAD/LICENSE` |
| google/uuid | `github.com/google/uuid` | BSD-3-Clause | `https://github.com/google/uuid/blob/HEAD/LICENSE` |
| wire | `github.com/google/wire` | Apache-2.0 | `https://github.com/google/wire/blob/HEAD/LICENSE` |
| godotenv | `github.com/joho/godotenv` | MIT | `https://github.com/joho/godotenv/blob/HEAD/LICENCE` |
| golang.org/x/crypto | `golang.org/x/crypto` | BSD-3-Clause | `https://github.com/golang/crypto/blob/master/LICENSE` |
| golang.org/x/image | `golang.org/x/image` | BSD-3-Clause | `https://github.com/golang/image/blob/master/LICENSE` |
| gorm | `gorm.io/gorm` | MIT | `https://github.com/go-gorm/gorm/blob/master/LICENSE` |
| postgres driver | `gorm.io/driver/postgres` | MIT | `https://github.com/go-gorm/postgres/blob/HEAD/License` |

Display names stay short and recognizable (the left column), matching the
existing frontend entries' style.

## Risks / Trade-offs

- **[GPL-3.0 copyleft dependency]** `github.com/barasher/go-exiftool` (used by
  the exif service) is GPL-3.0, so the exif binary is a GPL-3.0 derivative,
  while the About → License tab states the software is MIT. → Mitigation: this
  change discloses the exact component license; the discrepancy between the
  product's MIT statement and the GPL-3.0 exif dependency is surfaced here for
  a separate decision and is not resolved by this change.
- **[Manual list drifts from reality]** New backend dependencies may be added
  without updating the array. → Mitigation: add a short source comment above
  the array pointing at the backend `go.mod` files as the source of truth.
- **[Incorrect license or dead link]** A license may be misidentified or a repo
  may move/rename. → Mitigation: the implementation task verifies each entry's
  LICENSE and link.
- **[Mixed link semantics]** Frontend entries link to homepages while backend
  entries link to licenses. → Accepted per D2; the license identifier rendered
  beside each entry keeps the intent legible.

## Migration Plan

- Frontend-only change. Rebuild and redeploy the webapp; no backend services,
  database, or API contracts are touched.
- Rollback is a revert of the `AboutTab.tsx` list change.

## Open Questions

None. The data is static and self-contained; license verification happens
during implementation.
