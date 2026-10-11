// HTTP integration test for the "Gallery folders" settings.
//
// Location: tests/webapp/integration/ (shared helpers in ./support.ts).
// Runner:   webapp/vitest.integration.config.ts
//
// The suite boots the real `core` service (compiled Go binary) against the
// test PostgreSQL database, completes bootstrap to obtain an admin session and
// drives the gallery-folder settings exactly as the webapp does:
//
//   GET /api/folders → POST /api/folders (add) → GET /api/folders → DELETE
//
// As root folders for the test it uses the four sibling subdirectories of
// `tests/ocr/dataset` (eng, image, inscription, rus). They are passed as
// absolute paths so the Go process resolves them independently of its working
// directory.
//
// Run with:  make test-webapp-integration
//            (or, from the webapp directory: npm run test:integration)

import { existsSync, mkdtempSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { afterAll, beforeAll, describe, expect, it } from "vitest"
import {
  REPO_ROOT,
  SESSION_COOKIE_NAME,
  buildCoreService,
  createCoreClient,
  ensureVectorExtension,
  loadConfig,
  readCookie,
  resetBootstrapState,
  startCoreService,
} from "./support"
import type {
  AddFolderResult,
  CoreClient,
  GalleryFoldersResult,
  IntegrationConfig,
  RunningCore,
} from "./support"

const config: IntegrationConfig = loadConfig()

/** Parent of the four subdirectories reused as gallery root folders. */
const DATASET_ROOT = path.join(REPO_ROOT, "tests", "ocr", "dataset")

/** Sibling subdirectories of `tests/ocr/dataset` used as gallery roots. */
const DATASET_SUBFOLDERS = ["eng", "image", "inscription", "rus"] as const

/** Absolute paths of the dataset subfolders — the folders under test. */
const FOLDER_PATHS = DATASET_SUBFOLDERS.map((name) => path.join(DATASET_ROOT, name))

let workingDir: string | undefined
let binaryPath: string
let core: RunningCore
let client: CoreClient
let adminToken: string

/** Lists the gallery folders currently configured on the service. */
async function listFolders(): Promise<GalleryFoldersResult> {
  const response = await client.listFolders(adminToken)
  expect(response.status).toBe(200)
  return (await response.json()) as GalleryFoldersResult
}

/**
 * Removes every configured gallery folder so the suite starts from a clean
 * slate. The `gallery_folders` table is global and is not affected by the
 * users truncation performed by `resetBootstrapState`, so rows left behind by
 * a previous run must be cleared before asserting on exact counts.
 */
async function clearFolders(): Promise<void> {
  const { folders } = await listFolders()
  for (const folder of folders) {
    const response = await client.removeFolder(adminToken, folder.id)
    expect(response.status).toBe(200)
  }
}

beforeAll(async () => {
  for (const folderPath of FOLDER_PATHS) {
    if (!existsSync(folderPath)) {
      throw new Error(`expected the OCR dataset folder to exist: ${folderPath}`)
    }
  }

  // Ensure pgvector is present (idempotent) so the service can boot.
  await ensureVectorExtension(config)

  workingDir = mkdtempSync(path.join(tmpdir(), "flashbacks-folders-it-"))
  binaryPath = path.join(workingDir, "core")
  buildCoreService(binaryPath)

  await resetBootstrapState(config)
  core = await startCoreService(config, binaryPath)
  client = createCoreClient(core.baseUrl)

  const setup = await client.bootstrapSetup(config.newPassword, "Folders Admin")
  adminToken = readCookie(setup, SESSION_COOKIE_NAME) ?? ""
  if (setup.status !== 200 || !adminToken) {
    throw new Error(`bootstrap failed: HTTP ${setup.status}`)
  }

  await clearFolders()
}, 300_000)

afterAll(async () => {
  if (core) await core.stop()

  // Restore bootstrap mode so the suite can run repeatedly.
  try {
    await resetBootstrapState(config)
  } catch {
    // Best-effort cleanup; never mask the test result.
  }

  if (workingDir) rmSync(workingDir, { recursive: true, force: true })
})

describe("gallery folders settings (core service integration)", () => {
  it("starts with no configured folders", async () => {
    const result = await listFolders()
    expect(result.totalFolders).toBe(0)
    expect(result.folders).toEqual([])
  })

  it("adds the four OCR dataset folders as gallery roots", async () => {
    for (const folderPath of FOLDER_PATHS) {
      const response = await client.addFolder(adminToken, folderPath)
      expect(response.status).toBe(200)

      const body = (await response.json()) as AddFolderResult
      expect(body.message).toBe("folder.added")
      expect(body.folder.id).toBeGreaterThan(0)
      expect(body.folder.path).toBe(folderPath)
      expect(typeof body.scanStarted).toBe("boolean")
    }
  })

  it("returns all four folders from the folders endpoint", async () => {
    const result = await listFolders()

    expect(result.totalFolders).toBe(FOLDER_PATHS.length)
    expect(result.folders.map((folder) => folder.path).sort()).toEqual(
      [...FOLDER_PATHS].sort(),
    )
    for (const folder of result.folders) {
      expect(folder.fileCount).toBeGreaterThanOrEqual(0)
      expect(folder.createdAt).not.toBe("")
    }
  })

  it("rejects a folder that does not exist on disk", async () => {
    const missing = path.join(DATASET_ROOT, "no-such-folder")
    const response = await client.addFolder(adminToken, missing)

    expect(response.status).toBe(400)
    await expect(response.json()).resolves.toMatchObject({ error: "folder.invalid_path" })
  })

  it("rejects a folder that is already configured", async () => {
    const response = await client.addFolder(adminToken, FOLDER_PATHS[0])

    expect(response.status).toBe(409)
    await expect(response.json()).resolves.toMatchObject({
      error: "folder.already_in_gallery",
    })
  })

  it("removes a folder and drops it from the listing", async () => {
    const removeTarget = FOLDER_PATHS[1]
    const before = await listFolders()
    const target = before.folders.find((folder) => folder.path === removeTarget)
    if (!target) throw new Error(`folder "${removeTarget}" is not configured`)

    const response = await client.removeFolder(adminToken, target.id)
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toMatchObject({ message: "folder.removed" })

    const after = await listFolders()
    expect(after.totalFolders).toBe(FOLDER_PATHS.length - 1)
    expect(after.folders.some((folder) => folder.path === removeTarget)).toBe(false)
  })
})
