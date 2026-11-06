# People catalog — browser walkthrough

Playwright suite that exercises the four People screens against a **live
deployment**: the workspace grid (search, rename, merge, disband), the unassigned
faces view (attach, create) and the lightbox People tab (rename, reassign,
detach, attach). It replaces the manual checklist that used to close the
`people-catalog-management` change.

```
tests/e2e/
├── fixtures.ts                 # auth + language fixture, optional tunnel browser
├── browser-host.cjs            # run this on YOUR machine for tunnel mode
├── people-grid.spec.ts         # search / rename / merge / disband
├── people-unassigned.spec.ts   # attach to a person, create from faces
├── people-lightbox.spec.ts     # rename / reassign / detach / attach
└── support/
    ├── api.ts                  # catalog API client used for seeding + assertions
    ├── env.ts                  # E2E_* inputs and the skip reason
    └── ui.ts                   # page helpers built on the UI's data-slot hooks
```

The runner config is [`playwright.config.ts`](../../playwright.config.ts:1).

## Requirements

- `npm install` at the repository root (`@playwright/test` is a root dev
  dependency).
- A Chromium build for the runner: `npx playwright install chromium` (already
  done on the code-server host).
- A Flashbacks account **with catalog permissions** and a gallery where face
  recognition has left at least four unassigned faces. The specs read the
  unassigned listing first and `SKIP` when the gallery cannot seed them; they
  never modify existing people.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `E2E_BASE_URL` | `https://staging.flashbacks.life` | Target deployment |
| `E2E_LOGIN` | — | Account login |
| `E2E_PASSWORD` | — | Account password |
| `PW_TUNNEL` | unset | `1` runs the browser on your machine (see below) |
| `E2E_TUNNEL_PORT` | `56767` | Port the test runner exposes for the tunnel |
| `E2E_TUNNEL_ENDPOINT` | `ws://localhost:56767` | Endpoint `browser-host.cjs` polls |

Without `E2E_LOGIN` / `E2E_PASSWORD` every spec reports **SKIP** with the reason
from [`env.ts`](support/env.ts:1), so a plain checkout stays green.

```bash
E2E_LOGIN=admin E2E_PASSWORD='…' npm run test:e2e
```

## Headless on the server (default, CI)

```bash
npm run test:e2e          # or: npx playwright test
npm run test:e2e:list     # enumerate specs without credentials
```

Chromium runs headless in the code-server environment. Traces, screenshots and
videos of failures are written under `test-results/`.

## Watching the run locally (optional tunnel)

The tests still execute remotely; only the browser runs on your machine.

1. Forward port `56767` from the runner host to your machine (VS Code /
   code-server "Ports" panel, or `ssh -L 56767:localhost:56767 <host>`).
2. On your machine, install the browser host dependencies:

   ```bash
   npm install --no-save playwright @rushstack/playwright-browser-tunnel @rushstack/terminal
   npx playwright install chromium
   ```

   The local `playwright` version must match the runner's `@playwright/test`.
3. Start the browser host (leave it running):

   ```bash
   E2E_TUNNEL_ENDPOINT=ws://localhost:56767 node tests/e2e/browser-host.cjs
   ```

4. Run the suite with the tunnel flag:

   ```bash
   PW_TUNNEL=1 E2E_LOGIN=admin E2E_PASSWORD='…' npm run test:e2e:tunnel
   ```

The `browser` fixture in [`fixtures.ts`](fixtures.ts:1) uses
`@rushstack/playwright-browser-tunnel` only when `PW_TUNNEL=1`; the headless path
is unchanged.

## How seeding works

Each spec seeds through the catalog API with the same session the browser uses:

1. `GET /api/faces/unassigned` supplies the faces.
2. `POST /api/people` creates throwaway people named `E2E … <run suffix>`.
3. The UI performs the walkthrough; assertions poll the API for the resulting
   state (photo counts, face person ids, unassigned listing).
4. `finally` disbands everything the spec created, which returns the faces to
   unassigned. The suite leaves no long-lived catalog rows.

The `api` fixture also signs in through the login form (the app shell renders
only for a real session) before handing the test a `PeopleApi` bound to the
browser's cookie jar. Assertions target the UI's `data-slot` / `data-testid`
hooks, so they do not depend on the account's display language.
