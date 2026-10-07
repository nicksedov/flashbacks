# Tasks

## 1. Curate and verify backend component data

- [x] 1.1 Enumerate every direct third-party Go module (a `require` entry
  without an `// indirect` marker) from
  [`backend/core/go.mod`](../../../backend/core/go.mod:1),
  [`backend/exif/go.mod`](../../../backend/exif/go.mod:1),
  [`backend/ocr/go.mod`](../../../backend/ocr/go.mod:1), and
  [`tools/embeddings-builder/go.mod`](../../../tools/embeddings-builder/go.mod:1);
  exclude `github.com/flashbacks/*` and de-duplicate by module path. Verify the
  resulting set matches the 19-module inventory in
  [`design.md`](design.md:63) (`D3 — Scope of backend entries`).
- [x] 1.2 Verify each module's license identifier and license URL against its
  published LICENSE file, correcting the [`design.md`](design.md:63) table where
  needed; confirm in particular that
  [`github.com/barasher/go-exiftool`](../../../backend/exif/go.mod:6) resolves to
  **GPL-3.0**. Verify every final URL returns the expected license text.

## 2. Add backend entries to the About list

- [x] 2.1 In
  [`webapp/src/components/tabs/AboutTab.tsx`](../../../webapp/src/components/tabs/AboutTab.tsx:77),
  append the verified backend rows to the existing `thirdPartyComponents` array
  after the eleven frontend entries, using short display names and the license
  URL as `url`. Verify the frontend entries are byte-for-byte unchanged and run
  `npm run lint && npx tsc -b` (must pass with no errors).
- [x] 2.2 Add a short source-of-truth comment above `thirdPartyComponents`
  pointing at the backend `go.mod` files as the place to update when
  dependencies change. Verify the comment is present and run
  `npm run lint && npx tsc -b` (must pass).
- [x] 2.3 Confirm the list renders each new entry with its name and license
  identifier and that links keep `target="_blank"` /
  `rel="noopener noreferrer"`; confirm no new `TranslationKey` is required, or,
  if one is, add it to
  [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:882) and
  [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:882)
  together. Run `npm run lint && npx tsc -b` (must pass).

## 3. Integration verification

- [x] 3.1 Run the webapp test suite `cd webapp && npm test` and confirm the
  i18n `translations.test.ts` passes with no en/ru drift.
- [x] 3.2 Manually open About → Third-party components in both English and
  Russian; verify the single combined list shows all backend libraries with
  license links, the frontend entries are unchanged, and switching locale keeps
  the same components and license identifiers.
