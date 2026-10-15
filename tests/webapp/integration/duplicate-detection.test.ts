// HTTP integration tests for the base "find and resolve duplicates" journey.
//
// Location: tests/webapp/integration/ (shared helpers in ./support.ts).
// Runner:   webapp/vitest.integration.config.ts
//
// The suite boots the real `core` service against the test database and drives
// duplicate detection over HTTP:
//
//   POST /api/folders → POST /api/scan → GET /api/status (wait)
//     → GET /api/duplicates  [→ POST /api/delete-files → GET /api/duplicates]
//
// Duplicate grouping keys on the file content hash, so duplicates are byte-level
// copies of the same JPEG. Each test truncates the media-library tables first and
// seeds its own fixtures, so every test passes in isolation (focused-spec runs
// each evidence target on its own).

import { copyFileSync, existsSync, mkdtempSync, mkdirSync, rmSync } from "node:fs"
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
  pollUntil,
  readCookie,
  resetBootstrapState,
  resetLibraryState,
  startCoreService,
  triggerScanAndWait,
} from "./support"
import type {
  CoreClient,
  DuplicatesResult,
  IntegrationConfig,
  RunningCore,
} from "./support"

const config: IntegrationConfig = loadConfig()

const SOURCE_DOG = path.join(REPO_ROOT, "tests", "dataset", "image", "dog.jpg")
const SOURCE_LIGHT = path.join(REPO_ROOT, "tests", "dataset", "image", "light-bulb.jpg")

let workingDir: string
let binaryPath: string
let core: RunningCore
let client: CoreClient
let adminToken: string
const tempRoots: string[] = []

function makeTempDir(prefix: string): string {
  const dir = mkdtempSync(path.join(tmpdir(), prefix))
  tempRoots.push(dir)
  return dir
}

/** Copies `source` into `root/name` and returns the destination absolute path. */
function placeImage(root: string, name: string, source: string): string {
  mkdirSync(root, { recursive: true })
  const destination = path.join(root, name)
  copyFileSync(source, destination)
  return destination
}

/** Reads the current duplicate groups from the service. */
async function duplicates(): Promise<DuplicatesResult> {
  const response = await client.getDuplicates(adminToken)
  expect(response.status).toBe(200)
  return (await response.json()) as DuplicatesResult
}

/** Returns true when any group contains the given file path. */
function hasGroupWith(result: DuplicatesResult, filePath: string): boolean {
  return result.groups.some((group) => group.files.some((file) => file.path === filePath))
}

/** Number of indexed images directly under `dirPath`. */
async function galleryCount(dirPath: string): Promise<number> {
  const response = await client.getGalleryImages(
    adminToken,
    `dirPath=${encodeURIComponent(dirPath)}`,
  )
  expect(response.status).toBe(200)
  return ((await response.json()) as { totalImages: number }).totalImages
}

beforeAll(async () => {
  for (const source of [SOURCE_DOG, SOURCE_LIGHT]) {
    if (!existsSync(source)) throw new Error(`missing image fixture: ${source}`)
  }

  await ensureVectorExtension(config)

  workingDir = mkdtempSync(path.join(tmpdir(), "fb-duplicates-it-"))
  binaryPath = path.join(workingDir, "core")
  buildCoreService(binaryPath)

  await resetBootstrapState(config)
  await resetLibraryState(config)
  core = await startCoreService(config, binaryPath)
  client = createCoreClient(core.baseUrl)

  const setup = await client.bootstrapSetup(config.newPassword, "Duplicates Admin")
  adminToken = readCookie(setup, SESSION_COOKIE_NAME) ?? ""
  if (setup.status !== 200 || !adminToken) {
    throw new Error(`bootstrap failed: HTTP ${setup.status}`)
  }
}, 300_000)

afterAll(async () => {
  if (core) await core.stop()

  try {
    await resetBootstrapState(config)
    await resetLibraryState(config)
  } catch {
    // Best-effort cleanup; never mask the test result.
  }

  for (const root of tempRoots) rmSync(root, { recursive: true, force: true })
  if (workingDir) rmSync(workingDir, { recursive: true, force: true })
})

describe("duplicate detection (core service integration)", () => {
  it("returns an empty result when no two files are identical", async () => {
    await resetLibraryState(config)

    const root = makeTempDir("fb-dup-empty-")
    placeImage(root, "distinct-dog.jpg", SOURCE_DOG)
    placeImage(root, "distinct-light.jpg", SOURCE_LIGHT)

    const added = await client.addFolder(adminToken, root)
    expect(added.status).toBe(200)
    await triggerScanAndWait(client, adminToken)

    // Wait until both distinct files are indexed before asserting on duplicates.
    await pollUntil(() => galleryCount(root), (count) => count >= 2)

    const result = await duplicates()
    expect(result.groups).toEqual([])
    expect(result.totalGroups).toBe(0)
    expect(result.totalFiles).toBe(0)
  }, 180_000)

  it("groups files that share identical content", async () => {
    await resetLibraryState(config)

    const root = makeTempDir("fb-dup-group-")
    const firstCopy = placeImage(root, "dog-a.jpg", SOURCE_DOG)
    const secondCopy = placeImage(root, "dog-b.jpg", SOURCE_DOG)

    const added = await client.addFolder(adminToken, root)
    expect(added.status).toBe(200)
    await triggerScanAndWait(client, adminToken)
    await pollUntil(() => galleryCount(root), (count) => count >= 2)

    const result = await pollUntil(
      () => duplicates(),
      (value) => hasGroupWith(value, firstCopy) && hasGroupWith(value, secondCopy),
    )

    const group = result.groups.find((candidate) =>
      candidate.files.some((file) => file.path === firstCopy),
    )
    if (!group) throw new Error(`no duplicate group contains ${firstCopy}`)

    expect(group.files).toHaveLength(2)
    expect(group.files.map((file) => file.path).sort()).toEqual([firstCopy, secondCopy].sort())
  }, 180_000)

  it("removes the group once one duplicate is deleted", async () => {
    await resetLibraryState(config)

    const root = makeTempDir("fb-dup-resolve-")
    const keeper = placeImage(root, "keeper.jpg", SOURCE_DOG)
    const removed = placeImage(root, "removed.jpg", SOURCE_DOG)
    const trashDir = makeTempDir("fb-dup-trash-")

    const added = await client.addFolder(adminToken, root)
    expect(added.status).toBe(200)
    await triggerScanAndWait(client, adminToken)
    await pollUntil(
      () => duplicates(),
      (value) => hasGroupWith(value, keeper) && hasGroupWith(value, removed),
    )

    const deletion = await client.deleteFiles(adminToken, [removed], trashDir)
    expect(deletion.status).toBe(200)
    const deletionBody = (await deletion.json()) as { success: number; failed: number }
    expect(deletionBody.success).toBe(1)
    expect(deletionBody.failed).toBe(0)

    const after = await pollUntil(() => duplicates(), (value) => !hasGroupWith(value, removed))
    expect(hasGroupWith(after, removed)).toBe(false)
  }, 180_000)
})
