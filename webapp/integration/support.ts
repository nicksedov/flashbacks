// Shared support utilities for webapp HTTP integration tests.
//
// These helpers boot the real `core` Go service against a staging PostgreSQL
// database, wait until it answers `/api/auth/status`, and expose small helpers
// for asserting on the responses. PostgreSQL access uses the `pg` client so no
// external `psql`/`docker` binary is required.
//
// SAFETY: the production database ("flashbacks") is never touched. The target
// database defaults to the one configured in the repository `.env`
// (`DB_NAME=flashbacks-staging`) and can be overridden with `IT_DB_NAME`.

import { execFileSync, spawn } from "node:child_process"
import type { ChildProcess } from "node:child_process"
import { existsSync, readFileSync } from "node:fs"
import { createServer } from "node:net"
import path from "node:path"
import { fileURLToPath } from "node:url"
import pg from "pg"
import type { Client as PgClient } from "pg"

const HERE = path.dirname(fileURLToPath(import.meta.url))

/** Absolute path to the repository root (the parent of `webapp`). */
export const REPO_ROOT = path.resolve(HERE, "..", "..")

/** Directory of the `core` Go service. */
export const CORE_SERVICE_DIR = path.join(REPO_ROOT, "backend", "core")

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
 * falling back to the repository `.env` and finally to development defaults.
 */
export function loadConfig(): IntegrationConfig {
  const fileEnv = parseEnvFile(path.join(REPO_ROOT, ".env"))
  const pick = (key: string, fallback: string): string => process.env[key] ?? fileEnv[key] ?? fallback

  const dbName = pick("IT_DB_NAME", pick("DB_NAME", "flashbacks-staging"))
  if (dbName === PRODUCTION_DB && process.env.IT_ALLOW_NON_STAGING !== "1") {
    throw new Error(
      `Refusing to run integration tests against the production database "${PRODUCTION_DB}". ` +
        `Set IT_DB_NAME (e.g. flashbacks-staging) or IT_ALLOW_NON_STAGING=1 to override.`,
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
 * Ensures the pgvector extension exists in the staging database.
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
 * Launches the compiled `core` service against the staging database on a free
 * port and resolves once `/api/auth/status` responds successfully.
 */
export async function startCoreService(
  config: IntegrationConfig,
  binaryPath: string,
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
      ACCOUNT_CREATION_MODE: "admin_only",
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
