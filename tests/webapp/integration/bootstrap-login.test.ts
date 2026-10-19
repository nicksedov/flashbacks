// HTTP integration test for the bootstrap login flow.
//
// Location: tests/webapp/integration/ (shared helpers in ./support.ts).
// Runner:   webapp/vitest.integration.config.ts
//
// The suite boots the real `core` service (compiled Go binary) against the
// test PostgreSQL database, then drives the bootstrap authentication flow
// over HTTP exactly as the webapp does:
//
//   status (bootstrap mode) → reject bad creds → bootstrap login
//   → bootstrap/setup (creates admin) → normal login with the new password
//
// Run with:  make test-webapp-integration
//            (or, from the webapp directory: npm run test:integration)
//
// The database name comes from the `test.env` overlay applied on top of `.env`
// (default `flashbacks-test`); the production database is never touched.

import { mkdtempSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { afterAll, beforeAll, describe, expect, it } from "vitest"
import {
  buildCoreService,
  ensureVectorExtension,
  loadConfig,
  readCookie,
  resetBootstrapState,
  startCoreService,
} from "./support"
import type { IntegrationConfig, RunningCore } from "./support"

const config: IntegrationConfig = loadConfig()

/** Session cookie set by the core auth handlers. */
const SESSION_COOKIE = "session_id"

let core: RunningCore | undefined
let workingDir: string | undefined

function request(pathname: string, init?: RequestInit): Promise<Response> {
  if (!core) throw new Error("core service is not running")
  return fetch(`${core.baseUrl}${pathname}`, init)
}

function postJson(pathname: string, body: unknown): Promise<Response> {
  return request(pathname, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
}

beforeAll(async () => {
  // Provision the test database schema prerequisites and force bootstrap mode.
  await ensureVectorExtension(config)
  await resetBootstrapState(config)

  workingDir = mkdtempSync(path.join(tmpdir(), "flashbacks-core-it-"))
  const binaryPath = path.join(workingDir, "core")
  buildCoreService(binaryPath)
  core = await startCoreService(config, binaryPath)
}, 180_000)

afterAll(async () => {
  if (core) await core.stop()

  // Restore bootstrap mode so the suite can run repeatedly.
  try {
    await resetBootstrapState(config)
  } catch {
    // Cleanup is best-effort; never mask the test result.
  }

  if (workingDir) rmSync(workingDir, { recursive: true, force: true })
})

describe("bootstrap login (core service integration)", () => {
  it("starts in bootstrap mode while no users exist", async () => {
    const response = await request("/api/auth/status")
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toMatchObject({
      isAuthenticated: false,
      isBootstrapMode: true,
    })
  })

  it("rejects invalid bootstrap credentials", async () => {
    const response = await postJson("/api/auth/login", {
      login: config.bootstrapLogin,
      password: `${config.bootstrapPassword}-wrong`,
    })
    expect(response.status).toBe(401)
  })

  it("accepts the bootstrap admin and issues the bootstrap cookie", async () => {
    const response = await postJson("/api/auth/login", {
      login: config.bootstrapLogin,
      password: config.bootstrapPassword,
    })

    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toMatchObject({ isBootstrap: true })
    expect(readCookie(response, SESSION_COOKIE)).toBe("bootstrap")
  })

  it("completes bootstrap and creates a usable admin session", async () => {
    const response = await postJson("/api/auth/bootstrap/setup", {
      newPassword: config.newPassword,
      displayName: config.newDisplayName,
    })

    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toMatchObject({
      user: {
        login: config.bootstrapLogin,
        displayName: config.newDisplayName,
        role: "admin",
      },
    })

    const token = readCookie(response, SESSION_COOKIE)
    expect(token).toBeTruthy()
    expect(token).not.toBe("bootstrap")

    // The freshly minted session must authenticate a protected endpoint.
    const me = await request("/api/auth/me", {
      headers: { Cookie: `${SESSION_COOKIE}=${token}` },
    })
    expect(me.status).toBe(200)
    await expect(me.json()).resolves.toMatchObject({
      user: { login: config.bootstrapLogin, role: "admin" },
    })
  })

  it("leaves bootstrap mode and only accepts the new password", async () => {
    const status = await request("/api/auth/status")
    await expect(status.json()).resolves.toMatchObject({ isBootstrapMode: false })

    const staleLogin = await postJson("/api/auth/login", {
      login: config.bootstrapLogin,
      password: config.bootstrapPassword,
    })
    expect(staleLogin.status).toBe(401)

    const freshLogin = await postJson("/api/auth/login", {
      login: config.bootstrapLogin,
      password: config.newPassword,
    })
    expect(freshLogin.status).toBe(200)
    await expect(freshLogin.json()).resolves.toMatchObject({
      user: { login: config.bootstrapLogin, role: "admin" },
    })
    expect(readCookie(freshLogin, SESSION_COOKIE)).toBeTruthy()
  })
})
