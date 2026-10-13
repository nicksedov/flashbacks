# Webapp (React + TypeScript) — Component Requirements

Component requirements for the `webapp/` SPA (and `tests/webapp/`). Read this
document **before changing any `.ts`/`.tsx` file**, `package.json`, i18n
translations, or the generated API types.

Project-wide rules (English-only, clean architecture, i18n en/ru parity, task
mode) live in [`AGENTS.md`](../../AGENTS.md:1). Rules here are component-specific
and win over the root file whenever the two differ, and they are the **only** rule
source for the webapp — there is no local `webapp/AGENTS.md`.

The backend contract that feeds it is documented in
[`docs/agent/go-services.md`](go-services.md); setup details for humans live in
[`webapp/README.md`](../../webapp/README.md:1).

---

## 1. Stack

| Layer | Technology |
|---|---|
| Framework | React 19 (function components + hooks + context) |
| Language | TypeScript 6, strict mode, `verbatimModuleSyntax` |
| Build/dev | Vite 8 (`npm run dev`, port 5180) |
| Styling | Tailwind CSS 4, `clsx` + `tailwind-merge`, `class-variance-authority` |
| Primitives | Radix UI, `lucide-react` icons |
| Maps | `leaflet` + `react-leaflet` |
| Content | `react-markdown` + `remark-gfm`, `sonner` toasts |
| Tests | Vitest 4 (+ integration config) |

## 2. Structure

```
webapp/
├── src/
│   ├── api/          # client.ts, endpoints.ts, deepseek.ts
│   ├── components/   # reusable UI components (+ shared tile/skeleton/empty-state primitives)
│   ├── hooks/        # custom hooks (data fetching, infinite scroll, state)
│   ├── i18n/         # translations.en.ts, translations.ru.ts, context/provider, types
│   ├── lib/          # constants, formatting, paths, toast helpers, utils
│   ├── providers/    # Auth, Settings, GallerySelection context providers (+ hooks)
│   ├── styles/       # global CSS
│   ├── theme/        # theme tokens and provider
│   ├── types/        # api.ts (generated), index.ts (shared types)
│   ├── utils/        # helpers (e.g. buildImageUrl)
│   ├── App.tsx       # root component and routing
│   └── main.tsx      # entry point
├── public/           # PWA manifest, service worker, icons
├── eslint.config.js
└── package.json
```

Placed next to it: `tests/webapp/` holds the HTTP integration suite
(`npm run test:integration`, config `vitest.integration.config.ts`).

## 3. TypeScript rules

- Strict mode; `noUnusedLocals` and `noUnusedParameters` are **enforced**.
- `import type` for every type-only import (`verbatimModuleSyntax`).
- No `any` without justification; no `@ts-ignore`; no unchecked `as Type`
  assertions; no `enum` (use union types with `as const`).
- Path alias `@/*` → `src/*`.
- Define interfaces for component props and keep them focused; export shared types
  from `src/types/index.ts`.
- Field names must match the Go JSON tags **exactly** (camelCase) — see §6.
- Prefer type guards over assertions for runtime validation.

## 4. React rules

- Functional components only — **never** class components.
- Use context providers for shared state (`AuthProvider`, `SettingsProvider`,
  `GallerySelectionContext`); no global state library.
- Extract reusable logic into custom hooks under `src/hooks/`.
- Composition over inheritance; keep props explicit.
- Do **not** use the deprecated `React.FormEvent` — submit handlers take
  `React.SyntheticEvent<HTMLFormElement>`.
- Lazy-loaded tabs wrap named exports:
  `lazy(() => import("...").then(m => ({ default: m.X })))`.
- Cover utility functions and hooks with unit tests.
- Test user interactions, not implementation details; mock external dependencies.

## 5. i18n rules

- Translations live in `src/i18n/translations.en.ts` and `translations.ru.ts`.
- Update **both locales together**; `src/i18n/translations.test.ts` fails on any
  mismatch.
- The `TranslationKey` type is strict — arbitrary strings in `t()` are rejected by
  the compiler (and are forbidden by project rules).
- All new user-facing text (labels, empty states, toasts, errors) needs both an
  English and a Russian entry in the same change.

## 6. API integration and contracts

- API base URL is `/api`, proxied by Vite in dev; use the `fetch` client in
  `src/api/client.ts` with proper error handling.
- Include the CSRF token on mutating requests; session-based auth.
- Use type-safe form validation and surface user-friendly error messages; client
  validation complements server-side validation and never replaces it.
- Types mirroring the backend live in `webapp/src/types/api.ts`, generated from
  [`docs/api-contracts/core.yaml`](../api-contracts/core.yaml) —
  **whenever the OpenAPI spec changes, run `make generate-types`** instead of
  editing the file by hand.
- Go JSON tags are camelCase and must match these TypeScript names one-to-one:
  renaming a field on either side without the other breaks the contract.

## 7. Design system and styling

- Before implementing any UI, consult the authoritative design system spec at
  [`openspec/specs/design-system/spec.md`](../../openspec/specs/design-system/spec.md)
  and follow its token, component, layout, interaction, responsive, theming, and
  accessibility rules.
- Known UI drift vs the spec is tracked in
  [`docs/design-system.md`](../design-system.md).
- Use Tailwind utility classes with the theme tokens from `src/theme/`; dark mode
  works through CSS variables. Reuse the shared primitives (tile frame/thumbnail/
  overlay, skeleton, empty state) rather than introducing one-off markup.
- Radix primitives for accessible components; use `sonner` for toast
  notifications; `npm run lint` enforces the React hooks/refresh rules.

## 8. Checks (required after every change)

```bash
cd webapp
npm run lint            # ESLint
npx tsc -b              # type-check (also runs as part of `npm run build`)
npm test                # Vitest unit tests
npm run build           # tsc -b && vite build — must succeed
```

- `make lint-webapp` additionally type-checks the integration suite
  (`npx tsc -p ../tests/webapp`).
- `make test-webapp-integration` runs `tests/webapp` against the test DB
  (`test.env`, never `.env`).
- PWA: `public/manifest.json`, `public/sw.js` and the icon set are regenerated
  with `webapp/scripts/generate-pwa-icons.js`; keep icon sizes consistent when
  touching branding.

## 9. Definition of done for a webapp change

- [ ] `npm run lint && npx tsc -b` clean; no unused locals/parameters.
- [ ] `import type` used for type-only imports; no `any`, no class components.
- [ ] `translations.en.ts` and `translations.ru.ts` updated together and
      `translations.test.ts` green.
- [ ] Design-system spec consulted for any UI work; shared primitives reused.
- [ ] API field names match the Go JSON tags; `make generate-types` run when the
      OpenAPI spec changed.
- [ ] `npm test` green, plus `npm run build` for build-affecting changes.

## 10. See also

- [`docs/agent/go-services.md`](go-services.md) — the backend counterpart.
- [`docs/agent/sdd-workflow.md`](sdd-workflow.md) — SDD cycle and evidence gate.
- [`webapp/README.md`](../../webapp/README.md:1) — setup, commands and scripts.
