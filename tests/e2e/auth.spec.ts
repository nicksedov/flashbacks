import { anonymous as test, expect } from "./fixtures"
import { HAS_CREDENTIALS, LOGIN, PASSWORD, SKIP_REASON } from "./support/env"
import {
  AUTH_LABELS,
  expectLoginScreen,
  expectWorkspace,
  loginError,
  signIn,
  signOut,
} from "./support/ui"

test.describe("Authentication", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)

  test("guards the workspace behind the form and returns to it on sign-out", async ({ page }) => {
    // A fresh visitor sees the form and nothing else: the workspace shell is
    // only rendered for an authenticated session.
    await page.goto("/")
    await expectLoginScreen(page)
    await expect(page.locator("aside")).toHaveCount(0)

    // A wrong password is reported in place and leaves the visitor signed out.
    await signIn(page, LOGIN, `${PASSWORD}-not-the-password`)
    await expect(loginError(page)).toHaveText(AUTH_LABELS.invalidCredentials)
    await expect(page.locator("aside")).toHaveCount(0)

    // The same form, submitted with the real password, opens the workspace.
    await page.getByRole("textbox", { name: AUTH_LABELS.password }).fill(PASSWORD)
    await page.getByRole("button", { name: AUTH_LABELS.signIn }).click()
    await expectWorkspace(page)

    // Signing out drops the session and shows the form again.
    await signOut(page)
    await expect(page.locator("aside")).toHaveCount(0)
  })
})
