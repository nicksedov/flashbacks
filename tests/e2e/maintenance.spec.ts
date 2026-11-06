import { copyFile, mkdir, mkdtemp, rm, access } from "node:fs/promises"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { destructive as test, expect } from "./fixtures"
import {
  DESTRUCTIVE,
  DESTRUCTIVE_SKIP_REASON,
  HAS_CREDENTIALS,
  SKIP_REASON,
  uniqueFileName,
} from "./support/env"
import { DUPLICATE_SOURCE, SEED_PHOTO, type GalleryPhotoLayoutEntry } from "./support/gallery"
import {
  addGalleryFolder,
  applyBatchDedup,
  deletePhotoThroughGallery,
  emptyTrash,
  galleryTile,
  openNav,
  removeGalleryFolder,
  rescanGallery,
  restoreTrashedPhoto,
  saveTrashDir,
} from "./support/ui"

/** True when the file exists on this host (the stack runs beside the tests). */
async function exists(path: string): Promise<boolean> {
  return access(path).then(
    () => true,
    () => false,
  )
}

test.describe("Destructive maintenance", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)
  test.skip(!DESTRUCTIVE, DESTRUCTIVE_SKIP_REASON)

  test("trashes a photo, restores it and deletes it permanently", async ({ page, api, gallery }) => {
    const original = await api.appSettings()
    // A trash directory must live outside the registered folder: the stack
    // rejects a trash path inside the gallery ("trash_conflict").
    const trashDir = await mkdtemp(join(tmpdir(), "e2e-trash-"))
    const photoPath = gallery.primary.path
    const photoName = gallery.primary.fileName

    try {
      // Point the stack at a throwaway trash directory, so the round-trip never
      // touches the deployment's own trash.
      await saveTrashDir(page, trashDir)
      await expect.poll(async () => (await api.appSettings()).trashDir).toBe(trashDir)

      // 1. Move the photo to the trash through the grid.
      await deletePhotoThroughGallery(page, photoName)
      await expect
        .poll(async () => (await api.trashItemByOriginalPath(photoPath)) !== undefined, {
          message: "the deleted photo to appear in the trash",
        })
        .toBe(true)
      await expect.poll(async () => exists(photoPath)).toBe(false)

      // 2. The trash lists it, and Restore puts it back on disk.
      await openNav(page, "gallery-trash")
      await expect(galleryTile(page, photoName)).toBeVisible()
      await restoreTrashedPhoto(page, photoName)
      await expect
        .poll(async () => (await api.trashItemByOriginalPath(photoPath)) === undefined)
        .toBe(true)
      await expect.poll(async () => exists(photoPath)).toBe(true)

      // 3. Index it again so the grid can delete it a second time, then empty
      //    the trash for good.
      await rescanGallery(page)
      await expect
        .poll(async () => (await api.galleryImagePaths()).includes(photoPath), {
          message: "the restored photo to be re-indexed",
        })
        .toBe(true)
      await deletePhotoThroughGallery(page, photoName)
      await expect.poll(async () => (await api.trashItemByOriginalPath(photoPath)) !== undefined).toBe(true)
      await emptyTrash(page)

      // 4. Nothing is left behind.
      await expect.poll(async () => (await api.trash()).totalItems).toBe(0)
      expect(await api.trashItemByOriginalPath(photoPath)).toBeUndefined()
      await expect.poll(async () => exists(photoPath)).toBe(false)
      expect(await api.galleryImagePaths()).not.toContain(photoPath)
    } finally {
      await api.updateAppSettings({ trashDir: original.trashDir })
      await rm(trashDir, { recursive: true, force: true })
    }
  })

  test("adds and removes a gallery folder and restores a changed setting", async ({ page, api }) => {
    const original = await api.appSettings()
    const folder = await mkdtemp(join(tmpdir(), "e2e-folder-"))
    const photoName = uniqueFileName("folder")
    await copyFile(SEED_PHOTO, join(folder, photoName))

    try {
      // 1. Register the folder through the admin settings dialog.
      await addGalleryFolder(page, folder)
      await expect.poll(async () => (await api.folderByPath(folder)) !== undefined).toBe(true)

      // 2. Drop the registration again, which also removes its photos.
      await removeGalleryFolder(page, folder)
      await expect.poll(async () => (await api.folderByPath(folder)) === undefined).toBe(true)
      expect(await api.galleryImagePaths()).not.toContain(join(folder, photoName))

      // 3. A setting saved through the UI is read back by the API, and the
      //    previous value is restored the same way.
      const changedTrashDir = await mkdtemp(join(tmpdir(), "e2e-trash-"))
      try {
        await saveTrashDir(page, changedTrashDir)
        await expect.poll(async () => (await api.appSettings()).trashDir).toBe(changedTrashDir)
        await saveTrashDir(page, original.trashDir)
        await expect.poll(async () => (await api.appSettings()).trashDir).toBe(original.trashDir)
      } finally {
        await rm(changedTrashDir, { recursive: true, force: true })
      }
    } finally {
      await api.updateAppSettings({ trashDir: original.trashDir })
      await rm(folder, { recursive: true, force: true })
    }
  })

  test("deletes one copy of a duplicated pair through the batch dedup wizard", async ({ page, api, gallery }) => {
    const original = await api.appSettings()
    // Batch dedup moves the dropped copy to the trash, which needs a configured
    // trash outside the gallery — and lets the test verify the trashed copy.
    const trashDir = await mkdtemp(join(tmpdir(), "e2e-trash-"))
    const photoName = uniqueFileName("pair")
    const layout: GalleryPhotoLayoutEntry[] = [
      { source: DUPLICATE_SOURCE, target: join("keep", photoName) },
      { source: DUPLICATE_SOURCE, target: join("drop", photoName) },
    ]
    const keepDir = join(gallery.dir, "keep")
    const dropDir = join(gallery.dir, "drop")

    // The pair lives in the fixture folder, so it is indexed and cleaned up with
    // it. Recreate the pair inside that folder and let the stack index it.
    for (const entry of layout) {
      const target = join(gallery.dir, entry.target)
      await mkdir(dirname(target), { recursive: true })
      await copyFile(entry.source, target)
    }
    await rescanGallery(page)
    const keepPath = join(keepDir, photoName)
    const dropPath = join(dropDir, photoName)
    await expect
      .poll(async () => {
        const paths = await api.galleryImagePaths()
        return paths.includes(keepPath) && paths.includes(dropPath)
      }, { message: "the duplicated pair to be indexed" })
      .toBe(true)

    try {
      await saveTrashDir(page, trashDir)

      // The wizard keeps the copy in `keep` and removes the one in `drop`.
      await applyBatchDedup(page, keepDir)

      await expect
        .poll(async () => exists(dropPath), { message: "the dropped copy to be deleted" })
        .toBe(false)
      expect(await exists(keepPath)).toBe(true)
      // The dropped copy is recoverable from the trash the wizard was told to use.
      await expect
        .poll(async () => (await api.trashItemByOriginalPath(dropPath)) !== undefined)
        .toBe(true)

      // Empty the throwaway trash so the run leaves no rows behind.
      await emptyTrash(page)
      await expect.poll(async () => (await api.trashItemByOriginalPath(dropPath))).toBeUndefined()
    } finally {
      await api.updateAppSettings({ trashDir: original.trashDir })
      await rm(trashDir, { recursive: true, force: true })
    }
  })
})

