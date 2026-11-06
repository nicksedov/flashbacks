import { expect, test } from "./fixtures"
import { HAS_CREDENTIALS, SKIP_REASON, uniquePersonName } from "./support/env"
import { openLightboxPeopleList, openPeopleTab, personTile } from "./support/ui"

/**
 * Lightbox → People: rename a person, reassign a face through the whole-catalog
 * picker, detach it (unnumbered box, compacted list) and attach an unassigned
 * face again — all against a live deployment.
 */
test.describe("People catalog — lightbox corrections", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)

  test("renames, reassigns, detaches and re-attaches a face", async ({ page, api }) => {
    const created: number[] = []
    try {
      const faces = await api.unassignedFaces(1, 100)
      test.skip(
        faces.length < 2,
        "The gallery needs at least two unassigned faces to seed the walkthrough.",
      )
      const [photoFace, otherFace] = faces

      const personName = uniquePersonName("Photo")
      const otherName = uniquePersonName("Other")
      const person = await api.createPerson(personName, [photoFace.id])
      created.push(person.id)
      const other = await api.createPerson(otherName, [otherFace.id])
      created.push(other.id)

      // Open the seeded photo in the lightbox People mode.
      await openPeopleTab(page)
      await personTile(page, personName).click()
      await page.locator('[data-slot="person-photo"]').first().click()

      const faceBox = page.locator(`[data-testid="people-face-box"][data-face-id="${photoFace.id}"]`)
      await expect(faceBox).toBeVisible()
      await openLightboxPeopleList(page)

      // Rename from the people list; the list and the catalog both update.
      const renamed = `${personName} renamed`
      const listEntry = page.getByTestId("people-list-item").filter({ hasText: personName })
      await listEntry.getByTestId("people-list-rename").click()
      const renameDialog = page.getByRole("dialog")
      await expect(renameDialog).toBeVisible()
      await renameDialog.locator('[data-slot="rename-input"]').fill(renamed)
      await renameDialog.locator('[data-slot="rename-save"]').click()
      await expect(page.getByTestId("people-list-item").filter({ hasText: renamed })).toHaveCount(1)
      await expect
        .poll(async () => (await api.people()).some((entry) => entry.name === renamed))
        .toBe(true)

      // Reassign: the picker lists the whole catalog, including a person who is
      // not on this photo.
      await faceBox.click()
      await expect(page.getByRole("dialog")).toBeVisible()
      await page.locator('[data-slot="face-assign"]').click()
      const picker = page.getByRole("dialog")
      await expect(picker.locator('[data-slot="picker-person"]').filter({ hasText: otherName })).toBeVisible()
      await picker.locator('[data-slot="picker-person"]').filter({ hasText: otherName }).click()
      await expect
        .poll(async () => (await api.imageFaces(photoFace.path)).find((face) => face.id === photoFace.id)?.personId)
        .toBe(other.id)
      await expect(faceBox).toHaveAttribute("data-numbered", "true")

      // Detach: the box becomes unnumbered and the list compacts.
      await faceBox.click()
      await page.locator('[data-slot="face-detach"]').click()
      await expect(faceBox).toHaveAttribute("data-numbered", "false")
      await expect
        .poll(async () => (await api.imageFaces(photoFace.path)).find((face) => face.id === photoFace.id)?.personId)
        .toBe(0)

      // Attach again through the same face action dialog.
      await faceBox.click()
      await expect(page.getByRole("dialog")).toBeVisible()
      await page.locator('[data-slot="face-assign"]').click()
      await page
        .getByRole("dialog")
        .locator('[data-slot="picker-person"]')
        .filter({ hasText: otherName })
        .click()
      await expect(faceBox).toHaveAttribute("data-numbered", "true")
      await expect
        .poll(async () => (await api.imageFaces(photoFace.path)).find((face) => face.id === photoFace.id)?.personId)
        .toBe(other.id)
    } finally {
      for (const id of created) await api.disbandPerson(id).catch(() => undefined)
    }
  })
})
