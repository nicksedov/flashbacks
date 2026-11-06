import { defineConfig, devices } from "@playwright/test"

/**
 * End-to-end configuration for the catalog walkthrough.
 *
 * The suite runs against a live deployment — staging by default — because the
 * four People screens only prove themselves over real HTTP and a real browser.
 * Point `E2E_BASE_URL` at another environment (for example production, read-only
 * checks only) when needed.
 *
 * Headless Chromium runs in the code-server environment by default. Set
 * `PW_TUNNEL=1` to drive a browser on the developer's own machine through
 * `@rushstack/playwright-browser-tunnel`; see `tests/e2e/README.md`.
 */
const baseURL = process.env.E2E_BASE_URL ?? "https://staging.flashbacks.life"

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  // The specs seed and clean up shared catalog rows, so they run one at a time.
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: !!process.env.CI,
  reporter: [["list"]],
  use: {
    baseURL,
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
})
