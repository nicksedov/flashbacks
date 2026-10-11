// HTTP integration tests for the account-creation scenarios.
//
// Location: tests/webapp/integration/ (shared helpers in ./support.ts).
// Runner:   webapp/vitest.integration.config.ts
//
// Each `describe` boots the real `core` service against the test database
// with a specific `ACCOUNT_CREATION_MODE`, completes bootstrap to obtain an
// admin session, and drives the flow over HTTP exactly as the webapp does.
//
// Covered scenarios:
//   - self_service:              registration blocked during bootstrap; active
//                                account + auto-login; validation errors
//   - self_service_with_approval: pending account; blocked login; admin approve
//                                → login allowed; admin reject → login blocked;
//                                reject without reason; non-pending guard
//   - admin_only:                registration disabled; admin-created account
//
// Run with:  make test-webapp-integration
//            (or, from the webapp directory: npm run test:integration)

import { mkdtempSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { afterAll, beforeAll, describe, expect, it } from "vitest"
import {
  SESSION_COOKIE_NAME,
  buildCoreService,
  createCoreClient,
  ensureVectorExtension,
  loadConfig,
  readCookie,
  resetBootstrapState,
  startCoreService,
} from "./support"
import type { CoreClient, IntegrationConfig, RunningCore, UserSummary } from "./support"

const config: IntegrationConfig = loadConfig()

let workingDir: string | undefined
let binaryPath: string

/** Shared helper: boot the service in `mode` and create the root admin. */
interface Bootstrapped {
  core: RunningCore
  client: CoreClient
  adminToken: string
}

async function bootstrapped(mode: string): Promise<Bootstrapped> {
  await resetBootstrapState(config)
  const core = await startCoreService(config, binaryPath, { accountCreationMode: mode })
  const client = createCoreClient(core.baseUrl)

  const setup = await client.bootstrapSetup(config.newPassword, "Root Admin")
  const adminToken = readCookie(setup, SESSION_COOKIE_NAME)
  if (setup.status !== 200 || !adminToken) {
    await core.stop()
    throw new Error(`bootstrap failed for mode "${mode}": HTTP ${setup.status}, cookie=${adminToken ?? "none"}`)
  }

  return { core, client, adminToken }
}

async function shutDown(service: Bootstrapped): Promise<void> {
  await service.core.stop()
  await resetBootstrapState(config).catch(() => {
    // Best-effort: leave the test database in bootstrap mode for the next run.
  })
}

beforeAll(async () => {
  // The `core` service refuses to boot unless pgvector is already present and
  // only a superuser may create it. Ensure it here as well so this file does
  // not depend on another suite having run first (the call is idempotent).
  await ensureVectorExtension(config)

  workingDir = mkdtempSync(path.join(tmpdir(), "flashbacks-accounts-it-"))
  binaryPath = path.join(workingDir, "core")
  buildCoreService(binaryPath)
}, 300_000)

afterAll(() => {
  if (workingDir) rmSync(workingDir, { recursive: true, force: true })
})

describe("self_service: registration is blocked during bootstrap", () => {
  let core: RunningCore
  let client: CoreClient

  beforeAll(async () => {
    await resetBootstrapState(config)
    core = await startCoreService(config, binaryPath, { accountCreationMode: "self_service" })
    client = createCoreClient(core.baseUrl)
  }, 180_000)

  afterAll(async () => {
    await core.stop()
    await resetBootstrapState(config).catch(() => {
      // Best-effort cleanup.
    })
  })

  it("refuses registration until the first admin is created", async () => {
    const response = await client.register("earlybird", "Early Bird", "password-123")
    expect(response.status).toBe(409)
  })
})

describe("self_service mode", () => {
  let service: Bootstrapped

  beforeAll(async () => {
    service = await bootstrapped("self_service")
  }, 180_000)

  afterAll(async () => {
    await shutDown(service)
  })

  it("exposes the active creation mode in the status response", async () => {
    const response = await service.client.getStatus()
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toMatchObject({ accountCreationMode: "self_service" })
  })

  it("activates a self-registered account and signs it in immediately", async () => {
    const response = await service.client.register("alice", "Alice", "alice-password-1")
    expect(response.status).toBe(201)
    await expect(response.json()).resolves.toMatchObject({
      user: { login: "alice", role: "user", accountStatus: "active" },
    })

    const token = readCookie(response, SESSION_COOKIE_NAME)
    if (!token) throw new Error("expected an auto-login session cookie")
    const me = await service.client.getMe(token)
    expect(me.status).toBe(200)
    await expect(me.json()).resolves.toMatchObject({ user: { login: "alice" } })
  })

  it("rejects a password that is too short", async () => {
    const response = await service.client.register("shorty", "Shorty", "short")
    expect(response.status).toBe(400)
  })

  it("rejects a duplicate login case-insensitively", async () => {
    const response = await service.client.register("ALICE", "Alice Clone", "alice-password-2")
    expect(response.status).toBe(400)
  })
})

describe("self_service_with_approval mode", () => {
  let service: Bootstrapped

  beforeAll(async () => {
    service = await bootstrapped("self_service_with_approval")
  }, 180_000)

  afterAll(async () => {
    await shutDown(service)
  })

  /** Registers a user and returns its pending record from the admin queue. */
  async function registerPending(login: string): Promise<UserSummary> {
    const response = await service.client.register(login, login, `${login}-password-1`)
    expect(response.status).toBe(201)
    await expect(response.json()).resolves.toMatchObject({ pending: true })
    expect(readCookie(response, SESSION_COOKIE_NAME)).toBeUndefined()

    const queue = await service.client.listUsers(service.adminToken, "pending")
    expect(queue.status).toBe(200)
    const { users } = (await queue.json()) as { users: UserSummary[] }
    const user = users.find((candidate) => candidate.login === login)
    if (!user) throw new Error(`pending user "${login}" was not returned by the admin queue`)
    return user
  }

  it("exposes the active creation mode in the status response", async () => {
    const response = await service.client.getStatus()
    await expect(response.json()).resolves.toMatchObject({
      accountCreationMode: "self_service_with_approval",
    })
  })

  it("creates a pending account without a session and blocks its login", async () => {
    const user = await registerPending("bob")
    expect(user.accountStatus).toBe("pending")

    const login = await service.client.login("bob", "bob-password-1")
    expect(login.status).toBe(403)
  })

  it("activates a pending account when an admin approves it", async () => {
    const user = await registerPending("carol")

    const approval = await service.client.approve(service.adminToken, user.id)
    expect(approval.status).toBe(200)
    await expect(approval.json()).resolves.toMatchObject({
      user: { id: user.id, accountStatus: "active" },
    })

    const login = await service.client.login("carol", "carol-password-1")
    expect(login.status).toBe(200)
    expect(readCookie(login, SESSION_COOKIE_NAME)).toBeTruthy()
  })

  it("rejects a pending account with a reason and blocks its login", async () => {
    const user = await registerPending("dave")

    const withoutReason = await service.client.reject(service.adminToken, user.id, "")
    expect(withoutReason.status).toBe(400)

    const rejection = await service.client.reject(service.adminToken, user.id, "Duplicate account")
    expect(rejection.status).toBe(200)
    await expect(rejection.json()).resolves.toMatchObject({
      user: { id: user.id, accountStatus: "rejected", rejectionReason: "Duplicate account" },
    })

    const login = await service.client.login("dave", "dave-password-1")
    expect(login.status).toBe(403)
  })

  it("refuses to approve an account that is not pending", async () => {
    const user = await registerPending("erin")

    const first = await service.client.approve(service.adminToken, user.id)
    expect(first.status).toBe(200)

    const second = await service.client.approve(service.adminToken, user.id)
    expect(second.status).toBe(409)
  })

  it("lists only pending accounts for the approval queue", async () => {
    const response = await service.client.listUsers(service.adminToken, "pending")
    expect(response.status).toBe(200)
    const { users } = (await response.json()) as { users: UserSummary[] }

    expect(users.length).toBeGreaterThan(0)
    expect(users.every((user) => user.accountStatus === "pending")).toBe(true)
    expect(users.some((user) => user.login === "bob")).toBe(true)
  })
})

describe("admin_only mode", () => {
  let service: Bootstrapped

  beforeAll(async () => {
    service = await bootstrapped("admin_only")
  }, 180_000)

  afterAll(async () => {
    await shutDown(service)
  })

  it("exposes the active creation mode and disables self-registration", async () => {
    const status = await service.client.getStatus()
    await expect(status.json()).resolves.toMatchObject({ accountCreationMode: "admin_only" })

    const registration = await service.client.register("nobody", "Nobody", "nobody-password-1")
    expect(registration.status).toBe(403)
  })

  it("lets the admin create an active account that can log in", async () => {
    const created = await service.client.createUser(service.adminToken, {
      login: "frank",
      displayName: "Frank",
      role: "user",
      password: "frank-password-1",
    })
    expect(created.status).toBe(201)
    await expect(created.json()).resolves.toMatchObject({
      user: { login: "frank", role: "user", accountStatus: "active" },
    })

    const login = await service.client.login("frank", "frank-password-1")
    expect(login.status).toBe(200)
    await expect(login.json()).resolves.toMatchObject({ user: { login: "frank" } })
  })
})
