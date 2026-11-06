import { copyFile, mkdir, mkdtemp, rm } from "node:fs/promises"
import { tmpdir } from "node:os"
import { basename, dirname, join } from "node:path"
import { expect } from "@playwright/test"
import type { FlashbacksApi, GalleryFolderDTO, GalleryImageDTO } from "./api"
import { uniqueFileName, uniqueFolderName } from "./env"

/**
 * One throwaway gallery folder for the destructive flows.
 *
 * The folder lives in the OS temp directory, is registered with the running
 * stack through the public API and is removed again in teardown, so a run
 * never depends on — or leaves anything in — the real photo library.
 */

/** Where the checked-in sample photos live; overridable for unusual layouts. */
const DATASET_ROOT = process.env.E2E_DATASET_ROOT ?? join(process.cwd(), "tests", "dataset")

/** A plain photo with EXIF-free content, used as the primary seed. */
export const SEED_PHOTO = join(DATASET_ROOT, "image", "dog.jpg")

/** A second distinct photo, used to build a duplicate pair for the run. */
export const DUPLICATE_SOURCE = join(DATASET_ROOT, "image", "light-bulb.jpg")

/** How long the stack may take to index the copied files. */
const INDEX_TIMEOUT_MS = 90_000

export interface GalleryPhotoLayoutEntry {
  /** Absolute path of the source file. */
  source: string
  /** Path inside the temporary folder, e.g. `photo.jpg` or `pair/b.jpg`. */
  target: string
}

export interface TempGallery {
  /** Absolute path of the temporary folder on this host. */
  dir: string
  /** The folder registration returned by `POST /api/folders`. */
  folder: GalleryFolderDTO
  /** The photos the gallery reported for the folder after indexing. */
  images: GalleryImageDTO[]
  /** The first seeded photo, the one the maintenance flows act on. */
  primary: GalleryImageDTO
}

/**
 * The default layout: a single photo at the root of the folder, named after the
 * run so a spec never confuses it with a library photo.
 */
export function defaultLayout(): GalleryPhotoLayoutEntry[] {
  return [{ source: SEED_PHOTO, target: uniqueFileName("photo") }]
}

/**
 * Create a temporary folder, copy the requested photos into it, register it
 * with the stack and wait until every photo is indexed.
 *
 * The caller owns the result: pass it to {@link destroyTempGallery} in a
 * `finally` block so a failing test still removes the folder and the files.
 */
export async function createTempGallery(
  api: FlashbacksApi,
  layout: GalleryPhotoLayoutEntry[] = defaultLayout(),
): Promise<TempGallery> {
  const dir = await mkdtemp(join(tmpdir(), `${uniqueFolderName("gallery")}-`))

  for (const entry of layout) {
    const target = join(dir, entry.target)
    await mkdir(dirname(target), { recursive: true })
    await copyFile(entry.source, target)
  }

  const folder = await api.addFolder(dir)
  const expected = layout.map((entry) => basename(entry.target)).sort()

  await expect
    .poll(
      async () => {
        const page = await api.galleryImages({ dirPath: dir, pageSize: 100 })
        return page.images.map((image) => image.fileName).sort()
      },
      {
        message: `the stack to index ${expected.length} photo(s) from ${dir}`,
        timeout: INDEX_TIMEOUT_MS,
        intervals: [500, 1000, 2000, 5000],
      },
    )
    .toEqual(expected)

  const page = await api.galleryImages({ dirPath: dir, pageSize: 100 })
  const primary = page.images.find((image) => image.fileName === basename(layout[0].target))
  if (!primary) {
    throw new Error(`the gallery indexed ${dir} but did not report ${layout[0].target}`)
  }

  return { dir, folder, images: page.images, primary }
}

/**
 * Drop the folder registration and delete the temporary directory, tolerating
 * a stack that is already gone so teardown never masks the real failure.
 */
export async function destroyTempGallery(api: FlashbacksApi, gallery: TempGallery | undefined): Promise<void> {
  if (!gallery) return
  try {
    await api.removeFolder(gallery.folder.id)
  } catch (error) {
    console.warn(`could not remove gallery folder ${gallery.folder.id}: ${String(error)}`)
  } finally {
    await rm(gallery.dir, { recursive: true, force: true })
  }
}

/** Wait until the gallery reports a photo with this file name anywhere. */
export async function waitForGalleryFile(api: FlashbacksApi, fileName: string, timeout = INDEX_TIMEOUT_MS): Promise<void> {
  await expect
    .poll(
      async () => (await api.galleryImagePaths()).some((path) => basename(path) === fileName),
      { message: `the gallery to report ${fileName}`, timeout, intervals: [500, 1000, 2000, 5000] },
    )
    .toBe(true)
}

/** Wait until the gallery no longer reports a photo with this file name. */
export async function waitForGalleryFileGone(api: FlashbacksApi, fileName: string, timeout = INDEX_TIMEOUT_MS): Promise<void> {
  await expect
    .poll(
      async () => (await api.galleryImagePaths()).some((path) => basename(path) === fileName),
      { message: `the gallery to drop ${fileName}`, timeout, intervals: [500, 1000, 2000, 5000] },
    )
    .toBe(false)
}
