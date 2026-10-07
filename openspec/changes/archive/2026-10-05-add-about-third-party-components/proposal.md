# Proposal

## Why

The "About → Third-party components" tab currently lists only frontend
libraries (React, Leaflet, Radix UI, …). The backend microservices
(core, exif, ocr, shared, embeddings-builder) rely on a substantial set
of Go frameworks and libraries whose licenses are not disclosed anywhere in the
product. Users self-hosting Flashbacks need this attribution to understand the
licensing obligations of the shipped backend.

## What Changes

- Extend the existing single combined list in the "About → Third-party
  components" tab with the third-party Go frameworks and libraries actually
  used by the backend services.
- Each newly added backend component links directly to its license text (or the
  canonical license page for components without a single license file), instead
  of a project homepage.
- The existing frontend entries and their homepage links remain unchanged.
- No new UI controls, tabs, or API endpoints are introduced; the list stays a
  static, hand-maintained array in the webapp.

## Capabilities

### New Capabilities
- `third-party-components`: The About section discloses the third-party
  frameworks and libraries used across the product, each attributable to a
  license, including the backend Go dependencies.

### Modified Capabilities
<!-- No existing capability has its spec-level requirements changed. `branding`
     covers logo rendering only, and no other spec touches the About section. -->

## Impact

- `webapp/src/components/tabs/AboutTab.tsx` — extends the
  `thirdPartyComponents` array with backend entries.
- `webapp/src/i18n/translations.en.ts` / `translations.ru.ts` — only if new
  labels/strings are needed; the list is data-driven, so likely unchanged.
- No backend service code, database, API contract (OpenAPI/MCP), or Docker
  images are affected.
- License identifiers and links must be verified against each dependency's
  published license at implementation time.
