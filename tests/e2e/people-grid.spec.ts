import { expect, test } from "./fixtures"
import { HAS_CREDENTIALS, SKIP_REASON, uniquePersonName } from "./support/env"
import { openPeopleTab, personCard, personTile, searchPeople, selectPerson } from "./support/ui"

/**
 * Workspace → People grid: search, rename, merge and disband against a live
 * deployment.
 *
 * The people are seeded through the API from unassigned faces, so the spec
 * never depends on (or edits) the catalog a human curated. Everything it
 * creates is disbanded afterwards, which returns the faces to unassigned.
 */
test.describe("People catalog — grid tools", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)

  test("search, rename, fold a cluster in, merge and disband a seeded catalog", async ({
    page,
    api,
  }) => {
    const created: number[] = []
    try {
      const faces = await api.unassignedFaces(1, 100)
      test.skip(
        faces.length < 3,
        "The gallery needs at least three unassigned faces to seed the walkthrough.",
      )
      const [faceA, faceB, faceC] = faces

      const personA = uniquePersonName("Anna")
      const personB = uniquePersonName("Boris")
      const cluster = uniquePersonName("Cluster")
      created.push((await api.createPerson(personA, [faceA.id])).id)
      created.push((await api.createPerson(personB, [faceB.id])).id)
      created.push((await api.createPerson(cluster, [faceC.id])).id)

      await openPeopleTab(page)

      // Search narrows the grid to the matching person.
      await searchPeople(page, personA)
      await expect(personTile(page, personA)).toHaveCount(1)
      await expect(personTile(page, personB)).toHaveCount(0)
      await searchPeople(page, "")

      // Rename updates the tile without a reload.
      const renamed = `${personA} renamed`
      await personCard(page, personA).locator('[data-slot="person-rename"]').click()
      const renameDialog = page.getByRole("dialog")
      await expect(renameDialog).toBeVisible()
      await renameDialog.locator('[data-slot="rename-input"]').fill(renamed)
      await renameDialog.locator('[data-slot="rename-save"]').click()
      await expect(personTile(page, renamed)).toHaveCount(1)
      await expect
        .poll(async () => (await api.people()).some((person) => person.name === renamed))
        .toBe(true)

      // Fold a cluster into an already-known person: the per-tile action opens
      // the person picker, and the picked person absorbs the cluster.
      await personCard(page, cluster)
        .locator('[data-slot="person-merge-into"]')
        .click()
      const pickerDialog = page.getByRole("dialog")
      await expect(pickerDialog).toBeVisible()
      await pickerDialog.locator('[data-slot="picker-person"]').filter({ hasText: renamed }).click()

      await expect(personTile(page, cluster)).toHaveCount(0)
      await expect
        .poll(async () => (await api.people()).some((person) => person.name === cluster))
        .toBe(false)

      // Merge: the merge action needs two selections and previews the result.
      await selectPerson(page, renamed)
      await selectPerson(page, personB)
      const mergeButton = page.locator('[data-slot="people-merge"]')
      await expect(mergeButton).toBeEnabled()
      await mergeButton.click()

      const mergeDialog = page.getByRole("dialog")
      await expect(mergeDialog.locator('[data-slot="merge-preview"]')).toContainText(/\d+ photos/)
      const merged = `${personA} merged`
      await mergeDialog.locator('[data-slot="merge-input"]').fill(merged)
      await expect(mergeDialog.locator('[data-slot="merge-preview"]')).toContainText(merged)
      await mergeDialog.locator('[data-slot="merge-confirm"]').click()

      await expect(personTile(page, merged)).toHaveCount(1)
      await expect(personTile(page, renamed)).toHaveCount(0)
      await expect(personTile(page, personB)).toHaveCount(0)
      await expect
        .poll(async () => (await api.people()).some((person) => person.name === merged))
        .toBe(true)

      // Disband: the photos stay and the faces return to unassigned.
      await personCard(page, merged).locator('[data-slot="person-disband"]').click()
      const disbandDialog = page.getByRole("dialog")
      await expect(disbandDialog).toBeVisible()
      // The confirmation is the destructive-styled action inside the dialog.
      await disbandDialog.locator("button.bg-destructive").click()
      await expect(personTile(page, merged)).toHaveCount(0)

      await expect
        .poll(async () => {
          const remaining = await api.unassignedFaces(1, 100)
          const ids = new Set(remaining.map((face) => face.id))
          return ids.has(faceA.id) && ids.has(faceB.id)
        })
        .toBe(true)
    } finally {
      for (const id of created) await api.disbandPerson(id).catch(() => undefined)
    }
  })
})
