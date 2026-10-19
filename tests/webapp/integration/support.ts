// Shared support utilities for the webapp HTTP integration tests.
//
// The suite lives at `tests/webapp/integration/` (repository level) and is
// executed by Vitest via `webapp/package.json` -> `npm run test:integration`.
//
// These helpers boot the real `core` Go service against the dedicated test
// PostgreSQL database, wait until it answers `/api/auth/status`, and expose
// small helpers for asserting on the responses. PostgreSQL access uses the `pg`
// client so no external `psql`/`docker` binary is required.
//
// Configuration comes from the process environment, then the `.env` base
// profile, then the tracked `test.env` overlay at the repository root.
//
// SAFETY: the production database ("flashbacks") is never touched — `DB_NAME`
// defaults to the dedicated throwaway `flashbacks-test` database.

import { execFileSync, spawn } from "node:child_process"
import type { ChildProcess } from "node:child_process"
import { existsSync, readFileSync } from "node:fs"
import { createServer } from "node:net"
import path from "node:path"
import { fileURLToPath } from "node:url"
import pg from "pg"
import type { Client as PgClient } from "pg"

const HERE = path.dirname(fileURLToPath(import.meta.url))

/** Absolute path to the repository root (three levels above this file). */
export const REPO_ROOT = path.resolve(HERE, "..", "..", "..")

/** Directory of the `core` Go service. */
export const CORE_SERVICE_DIR = path.join(REPO_ROOT, "backend", "core")

/** Base development profile: the single source of the shared configuration. */
export const BASE_ENV_FILE = path.join(REPO_ROOT, ".env")

/** Test overlay layered on top of `.env`; holds only the test overrides. */
export const TEST_ENV_FILE = path.join(REPO_ROOT, "test.env")

/** Database that integration tests must never modify. */
const PRODUCTION_DB = "flashbacks"

export interface IntegrationConfig {
  dbHost: string
  dbPort: number
  dbUser: string
  dbPassword: string
  dbName: string
  superuser: string
  superuserPassword: string
  bootstrapLogin: string
  bootstrapPassword: string
  newPassword: string
  newDisplayName: string
}

export interface RunningCore {
  baseUrl: string
  port: number
  logs: () => string
  stop: () => Promise<void>
}

/** Minimal dotenv parser — reads `KEY=VALUE` lines, ignores comments and blanks. */
function parseEnvFile(file: string): Record<string, string> {
  const result: Record<string, string> = {}
  if (!existsSync(file)) return result

  for (const rawLine of readFileSync(file, "utf8").split("\n")) {
    const line = rawLine.trim()
    if (!line || line.startsWith("#")) continue

    const equals = line.indexOf("=")
    if (equals === -1) continue

    const key = line.slice(0, equals).trim()
    let value = line.slice(equals + 1).trim()
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1)
    }
    result[key] = value
  }

  return result
}

/**
 * Resolves the integration test configuration from the process environment,
 * falling back to the `.env` base profile, then the `test.env` overlay and
 * finally to development defaults.
 */
export function loadConfig(): IntegrationConfig {
  const fileEnv = { ...parseEnvFile(BASE_ENV_FILE), ...parseEnvFile(TEST_ENV_FILE) }
  const pick = (key: string, fallback: string): string => process.env[key] ?? fileEnv[key] ?? fallback

  const dbName = pick("DB_NAME", "flashbacks-test")
  if (dbName === PRODUCTION_DB && process.env.IT_ALLOW_NON_STAGING !== "1") {
    throw new Error(
      `Refusing to run integration tests against the production database "${PRODUCTION_DB}". ` +
        `Set DB_NAME=flashbacks-test in test.env, or IT_ALLOW_NON_STAGING=1 to override.`,
    )
  }

  return {
    dbHost: pick("DB_HOST", "postgreshost"),
    dbPort: Number(pick("DB_PORT", "5432")),
    dbUser: pick("DB_USER", "flashbacks"),
    dbPassword: pick("DB_PASSWORD", ""),
    dbName,
    superuser: pick("POSTGRES_SUPERUSER", "postgres"),
    superuserPassword: pick("POSTGRES_PASSWORD", pick("DB_PASSWORD", "")),
    bootstrapLogin: pick("BOOTSTRAP_LOGIN", "admin"),
    bootstrapPassword: pick("BOOTSTRAP_PASSWORD", "changeme"),
    newPassword: pick("IT_BOOTSTRAP_PASSWORD", "ItBootstrap!2345"),
    newDisplayName: pick("IT_BOOTSTRAP_DISPLAY_NAME", "Integration Admin"),
  }
}

async function withClient<T>(
  config: IntegrationConfig,
  credentials: { user: string; password: string },
  run: (client: PgClient) => Promise<T>,
): Promise<T> {
  const client = new pg.Client({
    host: config.dbHost,
    port: config.dbPort,
    user: credentials.user,
    password: credentials.password,
    database: config.dbName,
  })

  await client.connect()
  try {
    return await run(client)
  } finally {
    await client.end()
  }
}

/**
 * Ensures the pgvector extension exists in the test database.
 *
 * The `core` service runs `CREATE EXTENSION IF NOT EXISTS vector` on startup
 * and treats a failure as fatal. Because the extension is not "trusted" and the
 * application role is not a superuser, it must be created once by an admin —
 * this helper performs that idempotent bootstrap as the Postgres superuser.
 */
export async function ensureVectorExtension(config: IntegrationConfig): Promise<void> {
  await withClient(
    config,
    { user: config.superuser, password: config.superuserPassword },
    async (client) => {
      await client.query("CREATE EXTENSION IF NOT EXISTS vector")
    },
  )
}

/**
 * Resets the authentication state so the service starts in bootstrap mode.
 *
 * Bootstrap mode is active only while the `users` table is empty, so any rows
 * left over from a previous run are truncated (CASCADE removes dependent rows).
 * When the schema has not been created yet there is nothing to reset.
 */
export async function resetBootstrapState(config: IntegrationConfig): Promise<void> {
  await withClient(
    config,
    { user: config.dbUser, password: config.dbPassword },
    async (client) => {
      const { rows } = await client.query<{ present: boolean }>(
        "SELECT to_regclass('public.users') IS NOT NULL AS present",
      )
      if (!rows[0]?.present) return
      await client.query("TRUNCATE TABLE users RESTART IDENTITY CASCADE")
    },
  )
}

/**
 * Truncates the media-library tables so a suite starts from a clean slate.
 *
 * `image_files` is global (not scoped per user) and survives the `users`
 * truncation performed by `resetBootstrapState`, so rows left by a previous run
 * would otherwise create phantom duplicates or gallery counts. Tables that do not
 * exist yet (before the service has created the schema) are skipped.
 */
export async function resetLibraryState(config: IntegrationConfig): Promise<void> {
  await withClient(
    config,
    { user: config.dbUser, password: config.dbPassword },
    async (client) => {
      for (const table of ["image_files", "gallery_folders", "trash_items"]) {
        const { rows } = await client.query<{ present: boolean }>(
          "SELECT to_regclass($1) IS NOT NULL AS present",
          [`public.${table}`],
        )
        if (rows[0]?.present) {
          await client.query(`TRUNCATE TABLE ${table} RESTART IDENTITY CASCADE`)
        }
      }
    },
  )
}

/** Compiles the `core` service binary. Throws with the build output on failure. */
export function buildCoreService(binaryPath: string): void {
  try {
    execFileSync("go", ["build", "-o", binaryPath, "./cmd/server"], {
      cwd: CORE_SERVICE_DIR,
      stdio: ["ignore", "pipe", "pipe"],
      encoding: "utf8",
    })
  } catch (error) {
    const failure = error as { stdout?: string; stderr?: string }
    throw new Error(
      `go build of the core service failed:\n${failure.stdout ?? ""}\n${failure.stderr ?? ""}`,
    )
  }
}

/** Allocates an unused TCP port on the loopback interface. */
export function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer()
    server.once("error", reject)
    server.listen(0, "127.0.0.1", () => {
      const address = server.address()
      if (address && typeof address === "object") {
        const { port } = address
        server.close(() => resolve(port))
        return
      }
      server.close(() => reject(new Error("could not allocate a free TCP port")))
    })
  })
}

/**
 * Launches the compiled `core` service against the test database on a free port
 * and resolves once `/api/auth/status` responds successfully.
 */
export interface CoreServiceOptions {
  /** Overrides `ACCOUNT_CREATION_MODE` for the booted service. */
  accountCreationMode?: string
}

export async function startCoreService(
  config: IntegrationConfig,
  binaryPath: string,
  options: CoreServiceOptions = {},
): Promise<RunningCore> {
  const port = await getFreePort()
  const chunks: string[] = []

  const child = spawn(binaryPath, [], {
    cwd: CORE_SERVICE_DIR,
    env: {
      ...process.env,
      DB_HOST: config.dbHost,
      DB_PORT: String(config.dbPort),
      DB_USER: config.dbUser,
      DB_PASSWORD: config.dbPassword,
      DB_NAME: config.dbName,
      SERVER_HOST: "127.0.0.1",
      SERVER_PORT: String(port),
      CORS_ORIGINS: "*",
      BOOTSTRAP_LOGIN: config.bootstrapLogin,
      BOOTSTRAP_PASSWORD: config.bootstrapPassword,
      ACCOUNT_CREATION_MODE: options.accountCreationMode ?? "admin_only",
      // Keep background workers out of the way for a fast, deterministic boot.
      OCR_ENABLED: "false",
      BACKGROUND_SYNC_ENABLED: "false",
      THUMBNAIL_CACHE_ENABLED: "false",
    },
    stdio: ["ignore", "pipe", "pipe"],
  })

  child.stdout?.on("data", (data: Buffer) => chunks.push(data.toString()))
  child.stderr?.on("data", (data: Buffer) => chunks.push(data.toString()))

  const baseUrl = `http://127.0.0.1:${port}`

  try {
    await waitForReady(baseUrl, child, chunks)
  } catch (error) {
    await stopChild(child)
    throw error
  }

  return {
    baseUrl,
    port,
    logs: () => chunks.join(""),
    stop: () => stopChild(child),
  }
}

async function waitForReady(
  baseUrl: string,
  child: ChildProcess,
  logs: string[],
  timeoutMs = 60_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  let lastError = "no response yet"

  while (Date.now() < deadline) {
    if (child.exitCode !== null || child.signalCode !== null) {
      throw new Error(
        `core service exited during startup (code=${child.exitCode}, signal=${child.signalCode})\n${logs.join("")}`,
      )
    }

    try {
      const response = await fetch(`${baseUrl}/api/auth/status`)
      if (response.ok) return
      lastError = `HTTP ${response.status}`
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error)
    }

    await delay(250)
  }

  throw new Error(`core service was not ready after ${timeoutMs}ms (last: ${lastError})\n${logs.join("")}`)
}

function stopChild(child: ChildProcess): Promise<void> {
  if (child.exitCode !== null || child.signalCode !== null) return Promise.resolve()

  return new Promise((resolve) => {
    const force = setTimeout(() => {
      child.kill("SIGKILL")
      resolve()
    }, 5_000)

    child.once("exit", () => {
      clearTimeout(force)
      resolve()
    })

    child.kill("SIGTERM")
  })
}

export function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** Reads a cookie value from a response's `Set-Cookie` header. */
export function readCookie(response: Response, name: string): string | undefined {
  const headers = response.headers as Headers & { getSetCookie?: () => string[] }
  const multiple = headers.getSetCookie?.() ?? []
  const single = response.headers.get("set-cookie")
  const cookies = multiple.length > 0 ? multiple : single ? [single] : []

  for (const cookie of cookies) {
    const [pair] = cookie.split(";")
    const separator = pair.indexOf("=")
    if (separator === -1) continue
    if (pair.slice(0, separator).trim() === name) {
      return pair.slice(separator + 1).trim()
    }
  }

  return undefined
}

/** Name of the session cookie set by the core auth handlers. */
export const SESSION_COOKIE_NAME = "session_id"

/** Subset of the user DTO fields the integration tests assert on. */
export interface UserSummary {
  id: number
  login: string
  displayName: string
  role: string
  accountStatus: string
  rejectionReason: string
  isActive: boolean
}

export interface CreateUserInput {
  login: string
  displayName: string
  role: string
  password: string
}

/** Subset of the gallery-folder DTO fields the integration tests assert on. */
export interface GalleryFolderSummary {
  id: number
  path: string
  fileCount: number
  createdAt: string
}

export interface GalleryFoldersResult {
  folders: GalleryFolderSummary[]
  totalFolders: number
}

export interface AddFolderResult {
  message: string
  folder: GalleryFolderSummary
  scanStarted: boolean
}

/** Subset of gallery-image DTO fields the integration tests assert on. */
export interface GalleryImageSummary {
  id: number
  path: string
  fileName: string
  dirPath: string
  size: number
  modTime: string
}

export interface GalleryImagesResult {
  images: GalleryImageSummary[]
  totalImages: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

export interface DuplicateFileSummary {
  id: number
  path: string
  fileName: string
  dirPath: string
  modTime: string
}

export interface DuplicateGroupSummary {
  index: number
  hash: string
  size: number
  files: DuplicateFileSummary[]
}

export interface DuplicatesResult {
  groups: DuplicateGroupSummary[]
  totalFiles: number
  pageFiles: number
  totalGroups: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

/** Subset of application-settings DTO fields the integration tests assert on. */
export interface AppSettings {
  trashDir: string
  exifBackupDir: string
  syncDays: string
  dailySyncHour: number
  dailySyncMinute: number
  syncTimezoneOffset: number
}

export interface ScanStatus {
  scanning: boolean
  progress: string
  filesProcessed: number
}

export interface DeleteFilesResult {
  success: number
  failed: number
  failedFiles?: string[]
}

export interface MoveFilesResult {
  success: number
  failed: number
  failedFiles?: string[]
}

/** Thin wrapper around the core HTTP API used by the integration tests. */
export interface CoreClient {
  getStatus(): Promise<Response>
  login(login: string, password: string): Promise<Response>
  register(login: string, displayName: string, password: string): Promise<Response>
  bootstrapSetup(newPassword: string, displayName: string): Promise<Response>
  getMe(token: string): Promise<Response>
  listUsers(token: string, status?: string): Promise<Response>
  approve(token: string, id: number): Promise<Response>
  reject(token: string, id: number, reason: string): Promise<Response>
  createUser(token: string, input: CreateUserInput): Promise<Response>
  listFolders(token: string): Promise<Response>
  addFolder(token: string, path: string): Promise<Response>
  removeFolder(token: string, id: number): Promise<Response>
  startScan(token: string): Promise<Response>
  startFastScan(token: string): Promise<Response>
  getScanStatus(token: string): Promise<Response>
  getSettings(token: string): Promise<Response>
  updateSettings(token: string, patch: Record<string, unknown>): Promise<Response>
  getGalleryImages(token: string, query?: string): Promise<Response>
  getDuplicates(token: string, query?: string): Promise<Response>
  deleteFiles(token: string, filePaths: string[], trashDir: string): Promise<Response>
  moveFiles(token: string, filePaths: string[], targetDir: string): Promise<Response>
}

export function createCoreClient(baseUrl: string): CoreClient {
  const jsonHeaders = { "Content-Type": "application/json" }
  const withAuth = (token: string) => ({ Cookie: `${SESSION_COOKIE_NAME}=${token}` })

  return {
    getStatus: () => fetch(`${baseUrl}/api/auth/status`),
    login: (login, password) =>
      fetch(`${baseUrl}/api/auth/login`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ login, password }),
      }),
    register: (login, displayName, password) =>
      fetch(`${baseUrl}/api/auth/register`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ login, displayName, password }),
      }),
    bootstrapSetup: (newPassword, displayName) =>
      fetch(`${baseUrl}/api/auth/bootstrap/setup`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ newPassword, displayName }),
      }),
    getMe: (token) => fetch(`${baseUrl}/api/auth/me`, { headers: withAuth(token) }),
    listUsers: (token, status) =>
      fetch(
        status
          ? `${baseUrl}/api/admin/users?status=${encodeURIComponent(status)}`
          : `${baseUrl}/api/admin/users`,
        { headers: withAuth(token) },
      ),
    approve: (token, id) =>
      fetch(`${baseUrl}/api/admin/users/${id}/approve`, { method: "POST", headers: withAuth(token) }),
    reject: (token, id, reason) =>
      fetch(`${baseUrl}/api/admin/users/${id}/reject`, {
        method: "POST",
        headers: { ...jsonHeaders, ...withAuth(token) },
        body: JSON.stringify({ reason }),
      }),
    createUser: (token, input) =>
      fetch(`${baseUrl}/api/admin/users`, {
        method: "POST",
        headers: { ...jsonHeaders, ...withAuth(token) },
        body: JSON.stringify(input),
      }),
    listFolders: (token) => fetch(`${baseUrl}/api/folders`, { headers: withAuth(token) }),
    addFolder: (token, path) =>
      fetch(`${baseUrl}/api/folders`, {
        method: "POST",
        headers: { ...jsonHeaders, ...withAuth(token) },
        body: JSON.stringify({ path }),
      }),
    removeFolder: (token, id) =>
      fetch(`${baseUrl}/api/folders/${id}`, { method: "DELETE", headers: withAuth(token) }),
    startScan: (token) =>
      fetch(`${baseUrl}/api/scan`, { method: "POST", headers: withAuth(token) }),
    startFastScan: (token) =>
      fetch(`${baseUrl}/api/fast-scan`, { method: "POST", headers: withAuth(token) }),
    getScanStatus: (token) => fetch(`${baseUrl}/api/status`, { headers: withAuth(token) }),
    getSettings: (token) => fetch(`${baseUrl}/api/settings`, { headers: withAuth(token) }),
    updateSettings: (token, patch) =>
      fetch(`${baseUrl}/api/settings`, {
        method: "PUT",
        headers: { ...jsonHeaders, ...withAuth(token) },
        body: JSON.stringify(patch),
      }),
    getGalleryImages: (token, query) =>
      fetch(`${baseUrl}/api/gallery${query ? `?${query}` : ""}`, { headers: withAuth(token) }),
    getDuplicates: (token, query) =>
      fetch(`${baseUrl}/api/duplicates${query ? `?${query}` : ""}`, { headers: withAuth(token) }),
    deleteFiles: (token, filePaths, trashDir) =>
      fetch(`${baseUrl}/api/delete-files`, {
        method: "POST",
        headers: { ...jsonHeaders, ...withAuth(token) },
        body: JSON.stringify({ filePaths, trashDir }),
      }),
    moveFiles: (token, filePaths, targetDir) =>
      fetch(`${baseUrl}/api/move-files`, {
        method: "POST",
        headers: { ...jsonHeaders, ...withAuth(token) },
        body: JSON.stringify({ filePaths, targetDir }),
      }),
  }
}

/**
 * Polls `probe` until `isDone` reports true or the timeout elapses. Returns the
 * last observed value either way so callers can assert on the outcome.
 */
export async function pollUntil<T>(
  probe: () => Promise<T>,
  isDone: (value: T) => boolean,
  options: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<T> {
  const timeoutMs = options.timeoutMs ?? 120_000
  const intervalMs = options.intervalMs ?? 250
  const deadline = Date.now() + timeoutMs

  let last = await probe()
  while (!isDone(last)) {
    if (Date.now() >= deadline) return last
    await delay(intervalMs)
    last = await probe()
  }
  return last
}

/**
 * Triggers a full gallery scan and resolves once the service stops reporting a
 * running scan. A concurrent scan (HTTP 409) is accepted — the caller then waits
 * for the in-flight scan to finish.
 */
export async function triggerScanAndWait(
  client: CoreClient,
  token: string,
  timeoutMs = 120_000,
): Promise<ScanStatus> {
  const response = await client.startScan(token)
  if (response.status !== 202 && response.status !== 409) {
    throw new Error(`unexpected POST /api/scan response: HTTP ${response.status}`)
  }
  return pollUntil(
    async () => (await client.getScanStatus(token)).json() as Promise<ScanStatus>,
    (status) => status.scanning === false,
    { timeoutMs },
  )
}

/** Removes every configured gallery folder so a suite starts from a clean slate. */
export async function clearGalleryFolders(client: CoreClient, token: string): Promise<void> {
  const response = await client.listFolders(token)
  if (response.status !== 200) {
    throw new Error(`failed to list gallery folders: HTTP ${response.status}`)
  }
  const body = (await response.json()) as GalleryFoldersResult
  for (const folder of body.folders) {
    const removed = await client.removeFolder(token, folder.id)
    if (removed.status !== 200) {
      throw new Error(`failed to remove gallery folder ${folder.id}: HTTP ${removed.status}`)
    }
  }
}
