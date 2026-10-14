// HTTP integration tests for the base "scan then browse" journeys.
//
// Location: tests/webapp/integration/ (shared helpers in ./support.ts).
// Runner:   webapp/vitest.integration.config.ts
//
// The suite boots the real `core` service (compiled Go binary) against the test
// PostgreSQL database, completes bootstrap to obtain an admin session, and then
// drives the gallery through its HTTP API exactly as the webapp does:
//
//   POST /api/folders → POST /api/scan → GET /api/status (wait) → GET /api/gallery
//
// Each scenario uses an isolated temporary gallery root seeded with tiny JPEG
// copies so the content that gets indexed is fully deterministic.

import { copyFileSync, existsSync, mkdtempSync, mkdirSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { afterAll, beforeAll, describe, expect, it } from "vitest"
import {
  REPO_ROOT,
  SESSION_COOKIE_NAME,
  buildCoreService,
  clearGalleryFolders,
  createCoreClient,
  ensureVectorExtension,
  loadConfig,
  pollUntil,
  readCookie,
  resetBootstrapState,
  resetLibraryState,
  startCoreService,
  triggerScanAndWait,
} from "./support"
import type {
  CoreClient,
  GalleryImagesResult,
  IntegrationConfig,
  RunningCore,
} from "./support"

const config: IntegrationConfig = loadConfig()

/** Small deterministic JPEG reused as the indexed image fixture. */
const SOURCE_IMAGE = path.join(REPO_ROOT, "tests", "ocr", "dataset", "image", "dog.jpg")

let workingDir: string
let binaryPath: string
let core: RunningCore
let client: CoreClient
let adminToken: string
const tempRoots: string[] = []

function makeGalleryRoot(prefix: string): string {
  const root = mkdtempSync(path.join(tmpdir(), prefix))
  tempRoots.push(root)
  return root
}

/** Copies the fixture image into `root/name` and returns its absolute path. */
function placeImage(root: string, name: string): string {
  mkdirSync(root, { recursive: true })
  const destination = path.join(root, name)
  copyFileSync(SOURCE_IMAGE, destination)
  return destination
}

/** Fetches the gallery listing scoped to a single directory. */
async function gallery(dirPath: string, extraQuery = ""): Promise<GalleryImagesResult> {
  const params = new URLSearchParams({ dirPath })
  for (const [key, value] of new URLSearchParams(extraQuery)) {
    params.set(key, value)
  }
  const response = await client.getGalleryImages(adminToken, params.toString())
  expect(response.status).toBe(200)
  return (await response.json()) as GalleryImagesResult
}

beforeAll(async () => {
  if (!existsSync(SOURCE_IMAGE)) {
    throw new Error(`missing image fixture: ${SOURCE_IMAGE}`)
  }

  await ensureVectorExtension(config)

  workingDir = mkdtempSync(path.join(tmpdir(), "fb-gallery-it-"))
  binaryPath = path.join(workingDir, "core")
  buildCoreService(binaryPath)

  await resetBootstrapState(config)
  await resetLibraryState(config)
  core = await startCoreService(config, binaryPath)
  client = createCoreClient(core.baseUrl)

  const setup = await client.bootstrapSetup(config.newPassword, "Gallery Admin")
  adminToken = readCookie(setup, SESSION_COOKIE_NAME) ?? ""
  if (setup.status !== 200 || !adminToken) {
    throw new Error(`bootstrap failed: HTTP ${setup.status}`)
  }
}, 300_000)

afterAll(async () => {
  if (core) {
    try {
      await clearGalleryFolders(client, adminToken)
    } catch {
      // Best-effort cleanup; never mask the test result.
    }
    await core.stop()
  }

  try {
    await resetBootstrapState(config)
    await resetLibraryState(config)
  } catch {
    // Best-effort cleanup; never mask the test result.
  }

  for (const root of tempRoots) rmSync(root, { recursive: true, force: true })
  if (workingDir) rmSync(workingDir, { recursive: true, force: true })
})

describe("gallery browsing (core service integration)", () => {
  it("lists images indexed from a scanned folder", async () => {
    const root = makeGalleryRoot("fb-gallery-scan-")
    const imagePath = placeImage(root, "only.jpg")

    const added = await client.addFolder(adminToken, root)
    expect(added.status).toBe(200)

    await triggerScanAndWait(client, adminToken)

    const result = await pollUntil(() => gallery(root), (value) => value.totalImages >= 1)
    expect(result.images.map((image) => image.path)).toContain(imagePath)
  }, 180_000)

  it("paginates the gallery listing", async () => {
    const root = makeGalleryRoot("fb-gallery-page-")
    const firstImage = placeImage(root, "page-a.jpg")
    const secondImage = placeImage(root, "page-b.jpg")
    expect(firstImage).not.toBe(secondImage)

    const added = await client.addFolder(adminToken, root)
    expect(added.status).toBe(200)

    await triggerScanAndWait(client, adminToken)

    const page1 = await pollUntil(
      () => gallery(root, "pageSize=1&page=1"),
      (value) => value.totalImages >= 2,
    )
    expect(page1.totalImages).toBe(2)
    expect(page1.pageSize).toBe(1)
    expect(page1.hasNextPage).toBe(true)
    expect(page1.images).toHaveLength(1)

    const page2 = await gallery(root, "pageSize=1&page=2")
    expect(page2.images).toHaveLength(1)
    expect(page2.images[0]?.path).not.toBe(page1.images[0]?.path)
  }, 180_000)
})
