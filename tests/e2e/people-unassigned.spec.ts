import { expect, test } from "./fixtures"
import { HAS_CREDENTIALS, SKIP_REASON, uniquePersonName } from "./support/env"
import { openPeopleTab, personTile } from "./support/ui"

/**
 * Workspace → People → Unassigned faces: attach a face to an existing person and
 * create a named person from a multi-face selection.
 */
test.describe("People catalog — unassigned faces", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)

  test("attaches a face to a person and creates one from selected faces", async ({ page, api }) => {
    const created: number[] = []
    try {
      const faces = await api.unassignedFaces(1, 100)
      test.skip(
        faces.length < 4,
        "The gallery needs at least four unassigned faces to seed the walkthrough.",
      )
      const [targetFace, attachedFace, firstFace, secondFace] = faces

      const targetName = uniquePersonName("Target")
      const target = await api.createPerson(targetName, [targetFace.id])
      created.push(target.id)

      await openPeopleTab(page)
      await page.locator('[data-slot="people-view-unassigned"]').click()
      await expect(page.locator('[data-slot="unassigned-view"]')).toBeVisible()

      // Attach one unassigned face to the existing person.
      await page.locator(`[data-slot="unassigned-face"][data-face-id="${attachedFace.id}"]`).click()
      await page.locator('[data-slot="unassigned-attach"]').click()
      const picker = page.getByRole("dialog")
      await expect(picker).toBeVisible()
      await picker.locator('[data-slot="picker-person"]').filter({ hasText: targetName }).click()

      await expect(
        page.locator(`[data-slot="unassigned-face"][data-face-id="${attachedFace.id}"]`),
      ).toHaveCount(0)
      await expect
        .poll(async () => {
          const facesOnPhoto = await api.imageFaces(attachedFace.path)
          return facesOnPhoto.find((face) => face.id === attachedFace.id)?.personId
        })
        .toBe(target.id)

      // Create a new named person from two selected faces.
      const createdName = uniquePersonName("Created")
      await page.locator(`[data-slot="unassigned-face"][data-face-id="${firstFace.id}"]`).click()
      await page.locator(`[data-slot="unassigned-face"][data-face-id="${secondFace.id}"]`).click()
      await page.locator('[data-slot="unassigned-create"]').click()
      const createDialog = page.getByRole("dialog")
      await expect(createDialog).toBeVisible()
      await createDialog.locator('[data-slot="create-input"]').fill(createdName)
      await createDialog.locator('[data-slot="create-confirm"]').click()

      await expect(
        page.locator(`[data-slot="unassigned-face"][data-face-id="${firstFace.id}"]`),
      ).toHaveCount(0)
      await expect
        .poll(async () => (await api.people()).find((person) => person.name === createdName)?.id)
        .toBeTruthy()
      const createdPerson = (await api.people()).find((person) => person.name === createdName)
      if (createdPerson) created.push(createdPerson.id)

      // The new person is a normal tile in the catalog grid.
      await page.locator('[data-slot="people-view-grid"]').click()
      await expect(personTile(page, createdName)).toHaveCount(1)
    } finally {
      for (const id of created) await api.disbandPerson(id).catch(() => undefined)
    }
  })
})
