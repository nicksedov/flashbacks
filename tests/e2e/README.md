# Browser end-to-end suite

Playwright suite that exercises the running webapp in a real browser: the
sign-in/sign-out flow, the People catalog screens, gallery browsing and the
shared lightbox modes, every sidebar destination with its documented empty
states, and — opt-in — the destructive maintenance flows.

```
tests/e2e/
├── fixtures.ts                 # signed-in and signed-out tests, error guard, temp gallery
├── browser-host.cjs            # run this on YOUR machine for tunnel mode
├── auth.spec.ts                # login form, rejected password, sign-out
├── gallery.spec.ts             # all-images grid, lightbox modes, calendar/folders
├── navigation.spec.ts          # every sidebar destination + empty/degraded states
├── maintenance.spec.ts         # destructive flows, gated by E2E_DESTRUCTIVE=1
├── people-grid.spec.ts         # search / rename / merge / disband
├── people-unassigned.spec.ts   # attach to a person, create from faces
├── people-lightbox.spec.ts     # rename / reassign / detach / attach
└── support/
    ├── api.ts                  # FlashbacksApi client used for seeding + assertions
    ├── env.ts                  # E2E_* inputs, skip reasons, destructive guard
    ├── gallery.ts              # temporary gallery folder helpers
    └── ui.ts                   # page helpers built on the UI's data-slot hooks
```

The runner config is [`playwright.config.ts`](../../playwright.config.ts:1).

## Requirements

- `npm install` at the repository root (`@playwright/test` is a root dev
  dependency).
- A Chromium build for the runner: `npx playwright install chromium` (already
  done on the code-server host).
- A Flashbacks account on the target deployment. Read-only specs `SKIP` with a
  reason when their precondition is absent (no credentials, no photos, no
  duplicate groups, no dangling trash).

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `E2E_BASE_URL` | `https://staging.flashbacks.life` | Target deployment |
| `E2E_LOGIN` | — | Account login |
| `E2E_PASSWORD` | — | Account password |
| `E2E_DESTRUCTIVE` | unset | `1` enables the maintenance spec (mutates data) |
| `E2E_ALLOW_REMOTE_DESTRUCTIVE` | unset | `1` allows destructive runs off localhost |
| `PW_TUNNEL` | unset | `1` runs the browser on your machine (see below) |
| `E2E_TUNNEL_PORT` | `56767` | Port the test runner exposes for the tunnel |
| `E2E_TUNNEL_ENDPOINT` | `ws://localhost:56767` | Endpoint `browser-host.cjs` polls |
| `E2E_DATASET_ROOT` | `tests/dataset` | Where the maintenance fixtures copy photos from |

Without `E2E_LOGIN` / `E2E_PASSWORD` every spec reports **SKIP** with the reason
from [`env.ts`](support/env.ts:1), so a plain checkout stays green.

```bash
E2E_LOGIN=admin E2E_PASSWORD='…' npm run test:e2e
```

## Read-only run (default, CI)

```bash
npm run test:e2e          # or: npx playwright test
npm run test:e2e:list     # enumerate specs without credentials
npx playwright test gallery navigation auth   # one area at a time
```

`auth`, `gallery` and `navigation` never mutate the deployment: they assert what
it already holds. `maintenance` reports SKIP unless `E2E_DESTRUCTIVE=1`.

## Destructive run (local stack only)

```bash
E2E_DESTRUCTIVE=1 E2E_BASE_URL=http://localhost:5180 \
  E2E_LOGIN=admin E2E_PASSWORD='…' npx playwright test maintenance
```

The maintenance spec owns everything it touches:

1. the `gallery` fixture copies a `tests/dataset` photo into a fresh OS temp
   directory, registers it with `POST /api/folders` and polls `/api/gallery`
   until the photo is indexed;
2. the spec points the stack's trash directory at a second temp directory
   through the admin settings form (a trash path inside a registered folder is
   rejected by the API) and restores the previous value in `finally`;
3. every delete, restore, folder add/remove and batch dedup rule is applied
   through the UI, with assertions polled through the API and the file system;
4. teardown drops the folder registration, deletes both temp directories and
   leaves no trash rows behind.

A destructive run against anything that is not `localhost` / `127.0.0.1` is
refused before a session is opened or a file is copied, unless
`E2E_ALLOW_REMOTE_DESTRUCTIVE=1` is set as well. That guard lives in the
`destructiveRun` fixture in [`fixtures.ts`](fixtures.ts:1) and runs first.

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

## Fixtures and guards

- `test` ([`fixtures.ts`](fixtures.ts:1)) signs in through the login form — the
  app shell only renders for a real session — and exposes `api`, a
  `FlashbacksApi` bound to the browser's cookie jar.
- `anonymous` is the same base test without the `api` fixture, for flows that
  start signed out.
- Every test fails on an uncaught browser error: the auto `pageErrors` fixture
  collects `pageerror` events and fails the test with the message. Console noise
  and failed requests are not asserted.
- `destructive` adds the guard plus the temporary `gallery` folder described
  above.

## How seeding works

Specs seed through the API with the same session the browser uses, then drive
the UI and poll the API for the resulting state:

1. `GET /api/faces/unassigned`, `GET /api/gallery`, `GET /api/duplicates`,
   `GET /api/trash` supply the preconditions.
2. `POST /api/people` creates throwaway people named `E2E … <run suffix>`; the
   maintenance spec creates its photos in temp folders named after the run.
3. The UI performs the walkthrough; assertions poll the API (photo counts, face
   person ids, trash rows, gallery paths).
4. `finally` / fixture teardown removes everything the spec created.

Assertions target the UI's `data-slot` / `data-testid` hooks and, where copy is
the only hook, an en/ru label pair, so the suite runs against either language.
