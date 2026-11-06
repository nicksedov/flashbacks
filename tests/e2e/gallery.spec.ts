import { test, expect } from "./fixtures"
import { HAS_CREDENTIALS, SKIP_REASON } from "./support/env"
import {
  GALLERY_LABELS,
  galleryTile,
  lightbox,
  lightboxMode,
  openNav,
  type LightboxMode,
} from "./support/ui"

/** Modes the walkthrough switches through besides Details. */
const MODES: LightboxMode[] = ["ai", "ocr", "people", "tags"]

test.describe("Gallery browsing", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)

  test("lists the library and keeps the photo while switching lightbox modes", async ({ page, api }) => {
    const library = await api.galleryImages({ pageSize: 60 })
    test.skip(library.images.length === 0, "The library has no photos to browse.")
    const photo = library.images[0]

    // The all-images grid renders what the API reports for the library.
    await openNav(page, "gallery-all-images")
    for (const seeded of library.images.slice(0, 3)) {
      await expect(galleryTile(page, seeded.fileName)).toBeVisible()
    }

    // Search narrows the grid to the matching photo, and clearing it restores
    // the grid: the same tile is back without a reload.
    const search = page.getByPlaceholder(GALLERY_LABELS.search)
    await search.fill(photo.fileName)
    await expect(galleryTile(page, photo.fileName)).toBeVisible()
    await page.getByRole("button", { name: GALLERY_LABELS.clearSearch }).click()
    await expect(search).toHaveValue("")
    await expect(galleryTile(page, photo.fileName)).toBeVisible()

    // A tile opens the shared lightbox on that photo.
    await galleryTile(page, photo.fileName).click()
    const dialog = lightbox(page)
    await expect(dialog).toBeVisible()
    await expect(dialog.getByTestId("lightbox-close")).toBeVisible()

    // Details carries the file section for the opened photo.
    await lightboxMode(page, "exif").click()
    await expect(lightboxMode(page, "exif")).toHaveAttribute("data-state", "active")
    await expect(dialog.getByText(photo.path, { exact: true }).first()).toBeVisible()

    // Switching modes never closes the lightbox or drops the photo.
    for (const mode of MODES) {
      await lightboxMode(page, mode).click()
      await expect(lightboxMode(page, mode)).toHaveAttribute("data-state", "active")
      await expect(dialog.getByTestId("lightbox-close")).toBeVisible()
    }

    // Back on Details the same photo is still the one on screen.
    await lightboxMode(page, "exif").click()
    await expect(dialog.getByText(photo.path, { exact: true }).first()).toBeVisible()

    // Closing returns to the grid.
    await dialog.getByTestId("lightbox-close").click()
    await expect(dialog).toHaveCount(0)
    await expect(galleryTile(page, photo.fileName)).toBeVisible()
  })

  test("lists the registered folder in the folders view", async ({ page, api }) => {
    const folders = await api.galleryFolders()
    test.skip(folders.length === 0, "The deployment has no registered gallery folder.")
    const folder = folders[0]
    // The grid labels a root folder with its last path segment.
    const folderName = folder.path.split("/").filter(Boolean).pop() ?? folder.path

    await openNav(page, "gallery-folders")
    await expect(page.getByText(GALLERY_LABELS.rootFolders)).toBeVisible()
    await expect(
      page.getByRole("button").filter({ hasText: folderName }).first(),
    ).toBeVisible()
  })

  test("shows a dated photo in the calendar view", async ({ page, api }) => {
    const calendar = await api.calendar()
    test.skip(
      calendar.groups.length === 0,
      "The deployment has no photo with a capture date, so the calendar has nothing to group.",
    )
    const datedPhoto = calendar.groups[0].images[0]

    await openNav(page, "gallery-calendar")
    await expect(galleryTile(page, datedPhoto.fileName)).toBeVisible()
  })
})
