// HTTP integration tests for the base "move files" journey.
//
// Location: tests/webapp/integration/ (shared helpers in ./support.ts).
// Runner:   webapp/vitest.integration.config.ts
//
// The suite boots the real `core` service against the test database, completes
// bootstrap for an admin session, then drives the move endpoint:
//
//   POST /api/move-files { filePaths, targetDir }
//
// Assertions are made against the filesystem, because moving files is the
// observable behavior; name conflicts must be suffixed, never overwritten.

import { copyFileSync, existsSync, mkdtempSync, mkdirSync, readdirSync, rmSync, statSync } from "node:fs"
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
  resetLibraryState,
  startCoreService,
} from "./support"
import type { CoreClient, IntegrationConfig, MoveFilesResult, RunningCore } from "./support"

const config: IntegrationConfig = loadConfig()

const SOURCE_DOG = path.join(REPO_ROOT, "tests", "dataset", "image", "dog.jpg")
const SOURCE_LIGHT = path.join(REPO_ROOT, "tests", "dataset", "image", "light-bulb.jpg")

let workingDir: string
let binaryPath: string
let core: RunningCore
let client: CoreClient
let adminToken: string
const tempRoots: string[] = []

function makeDir(prefix: string): string {
  const dir = mkdtempSync(path.join(tmpdir(), prefix))
  tempRoots.push(dir)
  return dir
}

/** Copies `source` into `dir/name` and returns the destination absolute path. */
function placeImage(dir: string, name: string, source: string): string {
  mkdirSync(dir, { recursive: true })
  const destination = path.join(dir, name)
  copyFileSync(source, destination)
  return destination
}

beforeAll(async () => {
  for (const source of [SOURCE_DOG, SOURCE_LIGHT]) {
    if (!existsSync(source)) throw new Error(`missing image fixture: ${source}`)
  }

  await ensureVectorExtension(config)

  workingDir = mkdtempSync(path.join(tmpdir(), "fb-move-it-"))
  binaryPath = path.join(workingDir, "core")
  buildCoreService(binaryPath)

  await resetBootstrapState(config)
  await resetLibraryState(config)
  core = await startCoreService(config, binaryPath)
  client = createCoreClient(core.baseUrl)

  const setup = await client.bootstrapSetup(config.newPassword, "Move Admin")
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

describe("file move (core service integration)", () => {
  it("moves a file into the target directory", async () => {
    const source = makeDir("fb-move-source-")
    const target = makeDir("fb-move-target-")
    const filePath = placeImage(source, "move-me.jpg", SOURCE_DOG)

    const response = await client.moveFiles(adminToken, [filePath], target)
    expect(response.status).toBe(200)

    const body = (await response.json()) as MoveFilesResult
    expect(body.success).toBe(1)
    expect(body.failed).toBe(0)

    expect(existsSync(path.join(target, "move-me.jpg"))).toBe(true)
    expect(existsSync(filePath)).toBe(false)
  }, 60_000)

  it("resolves a name conflict without overwriting", async () => {
    const source = makeDir("fb-move-conflict-source-")
    const target = makeDir("fb-move-conflict-target-")
    const existing = placeImage(target, "dup.jpg", SOURCE_LIGHT)
    const incoming = placeImage(source, "dup.jpg", SOURCE_DOG)

    const response = await client.moveFiles(adminToken, [incoming], target)
    expect(response.status).toBe(200)

    const body = (await response.json()) as MoveFilesResult
    expect(body.success).toBe(1)

    // The pre-existing file survives with its original content…
    expect(existsSync(existing)).toBe(true)
    expect(statSync(existing).size).toBe(statSync(SOURCE_LIGHT).size)
    // …the incoming file was moved out of the source…
    expect(existsSync(incoming)).toBe(false)
    // …and both files now coexist in the target.
    const files = readdirSync(target).sort()
    expect(files).toHaveLength(2)
    const movedName = files.find((name) => name !== "dup.jpg")
    if (!movedName) throw new Error("the moved file was not suffixed into the target directory")
    expect(statSync(path.join(target, movedName)).size).toBe(statSync(SOURCE_DOG).size)
  }, 60_000)
})
