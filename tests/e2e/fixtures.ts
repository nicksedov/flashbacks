import { test as base, expect } from "@playwright/test"
import { PeopleApi } from "./support/api"
import { LOGIN, PASSWORD } from "./support/env"

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

/**
 * Test with the catalog walkthrough fixtures.
 *
 * The `api` fixture signs in through the login form once per test and exposes
 * the catalog API bound to the browser's cookie jar. Specs that need credentials
 * call `test.skip(!HAS_CREDENTIALS, ...)`.
 */
export const test = withTunnel.extend<{ api: PeopleApi }>({
  api: async ({ page, baseURL }, use) => {
    // Sign in through the form, exactly like a user: the app shell is only
    // rendered for a real session, and `page.request` then shares the browser
    // context's cookies for seeding and assertions.
    await page.goto(baseURL ?? "/")
    if (await page.locator('input[type="password"]').count()) {
      await page.getByRole("textbox", { name: "Login" }).fill(LOGIN)
      await page.getByRole("textbox", { name: "Enter password" }).fill(PASSWORD)
      await page.getByRole("button", { name: "Sign In" }).click()
    }
    await expect(page.getByRole("button", { name: /^(People|Люди)$/ }).first()).toBeVisible()

    const api = new PeopleApi(page.request)
    await use(api)
    await page.request.post("/api/auth/logout").catch(() => undefined)
  },
})

export { expect }
