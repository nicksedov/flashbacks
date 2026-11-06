import { expect, type Locator, type Page } from "@playwright/test"

/** Open the workspace People tab and wait for the catalog grid. */
export async function openPeopleTab(page: Page): Promise<void> {
  await page.getByRole("button", { name: /^(People|Люди)$/ }).first().click()
  await expect(page.locator('[data-slot="people-search"]')).toBeVisible()
}

/** The clickable tile button of one person (carries the display name). */
export function personTile(page: Page, name: string): Locator {
  return page.locator('[data-slot="person-tile"]').filter({ hasText: name })
}

/** The tile wrapper holding both the open button and the tile actions. */
export function personCard(page: Page, name: string): Locator {
  return personTile(page, name).locator("xpath=..")
}

/** Select the person with the tile checkbox. */
export async function selectPerson(page: Page, name: string): Promise<void> {
  await personCard(page, name).locator('[data-slot="person-select"]').click()
  await expect(personCard(page, name).locator('[data-slot="person-select"]')).toHaveAttribute(
    "aria-pressed",
    "true",
  )
}

/** Search the grid by name. */
export async function searchPeople(page: Page, text: string): Promise<void> {
  await page.locator('[data-slot="people-search"]').fill(text)
}

/**
 * Make sure the lightbox people side panel is open. It is collapsible below
 * `md`; on wide viewports the content is already rendered.
 */
export async function openLightboxPeopleList(page: Page): Promise<void> {
  const firstEntry = page.getByTestId("people-list-item").first()
  if (!(await firstEntry.isVisible().catch(() => false))) {
    await page.getByTestId("mobile-detail-toggle").click()
  }
  await expect(firstEntry).toBeVisible()
}

