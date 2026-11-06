import { test as base, expect, type Page } from "@playwright/test"
import { FlashbacksApi } from "./support/api"
import { LOGIN, PASSWORD, assertDestructiveAllowed } from "./support/env"
import { createTempGallery, destroyTempGallery, type TempGallery } from "./support/gallery"
import { expectWorkspace, signIn } from "./support/ui"

type BaseTest = typeof base

/**
 * Tunnel mode: the browser runs on the developer's machine while the tests run
 * here. Enable it with `PW_TUNNEL=1`, start `tests/e2e/browser-host.cjs`
 * locally and forward `E2E_TUNNEL_PORT` (default 56767) from the local machine
 * to this environment. Without the flag the fixture launches Chromium locally,
 * which is the default and the CI path.
 */
const withTunnel: BaseTest =
  process.env.PW_TUNNEL === "1"
    ? (base.extend({
        browser: async ({ browserName, launchOptions, channel, headless }, use) => {
          const { createTunneledBrowserAsync } = await import("@rushstack/playwright-browser-tunnel")
          const { ConsoleTerminalProvider, Terminal } = await import("@rushstack/terminal")

          const terminal = new Terminal(new ConsoleTerminalProvider())
          const port = Number(process.env.E2E_TUNNEL_PORT ?? "56767")
          const tunneled = await createTunneledBrowserAsync(
            browserName as "chromium" | "firefox" | "webkit",
            { ...(launchOptions as Record<string, unknown>), channel, headless } as never,
            terminal,
            port,
          )

          await use(tunneled.browser)
          await tunneled[Symbol.asyncDispose]()
        },
      }) as unknown as BaseTest)
    : base

/** Messages of uncaught browser errors collected during a test. */
type ErrorGuard = { pageErrors: string[] }

/**
 * Auto fixture: collects uncaught browser errors and fails the test when any
 * occurred. A screen that renders blank because a chunk threw is a defect even
 * when its selectors happen to survive, so the guard turns silent failures into
 * red tests. React warnings and failed network requests are not page errors and
 * stay out of the collector.
 */
const guarded = withTunnel.extend<ErrorGuard>({
  pageErrors: [
    async ({ page }: { page: Page }, use: (errors: string[]) => Promise<void>) => {
      const errors: string[] = []
      const onPageError = (error: Error) => {
        errors.push(error.message)
      }
      page.on("pageerror", onPageError)
      await use(errors)
      page.off("pageerror", onPageError)
      if (errors.length > 0) {
        throw new Error(`Uncaught browser error(s) during the test:\n${errors.join("\n")}`)
      }
    },
    { auto: true },
  ],
})

/**
 * Base test for specs that must start signed out (authentication flows) or do
 * not need the API helper. Like every spec in this suite it still fails on an
 * uncaught browser error.
 */
export const anonymous = guarded

/**
 * Test with the signed-in fixtures.
 *
 * The `api` fixture signs in through the login form once per test and exposes
 * the API client bound to the browser's cookie jar. Specs that need credentials
 * call `test.skip(!HAS_CREDENTIALS, ...)`.
 */
export const test = guarded.extend<{ api: FlashbacksApi }>({
  api: async ({ page, baseURL }, use) => {
    // Sign in through the form, exactly like a user: the app shell is only
    // rendered for a real session, and `page.request` then shares the browser
    // context's cookies for seeding and assertions.
    await page.goto(baseURL ?? "/")
    if (await page.locator('input[type="password"]').count()) {
      await signIn(page, LOGIN, PASSWORD)
    }
    await expectWorkspace(page)

    const api = new FlashbacksApi(page.request)
    await use(api)
    await page.request.post("/api/auth/logout").catch(() => undefined)
  },
})

/**
 * Test with a throwaway gallery folder.
 *
 * `destructiveRun` is an auto fixture and therefore runs before the signed-in
 * `api` fixture: a run that is not allowed is refused before a session is
 * opened or anything is created. The `gallery` fixture then copies a sample
 * photo into an OS temp directory, registers the folder and waits for the photo
 * to be indexed. Teardown removes the registration and the directory even when
 * the test fails, so repeated runs start from the same state.
 */
type DestructiveFixtures = { gallery: TempGallery; destructiveRun: string }

export const destructive = test.extend<DestructiveFixtures>({
  destructiveRun: [
    async ({}, use) => {
      assertDestructiveAllowed()
      await use("allowed")
    },
    { auto: true },
  ],
  gallery: async ({ api }, use) => {
    const gallery = await createTempGallery(api)
    try {
      await use(gallery)
    } finally {
      await destroyTempGallery(api, gallery)
    }
  },
})

export { expect }
