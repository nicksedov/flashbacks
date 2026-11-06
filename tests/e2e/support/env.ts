/**
 * Environment inputs for the end-to-end suite.
 *
 * The suite targets a live deployment and a real account; neither is committed.
 * Without credentials every spec skips with an explicit reason instead of
 * failing, so a plain checkout stays green.
 *
 * Read-only flows (authentication, gallery, navigation) run against any
 * deployment. Flows that mutate data are opt-in: they need `E2E_DESTRUCTIVE=1`
 * and a locally booted stack, and they refuse a remote target unless
 * `E2E_ALLOW_REMOTE_DESTRUCTIVE=1` is set as well.
 */
export const BASE_URL = process.env.E2E_BASE_URL ?? "https://staging.flashbacks.life"
export const LOGIN = process.env.E2E_LOGIN ?? ""
export const PASSWORD = process.env.E2E_PASSWORD ?? ""
export const HAS_CREDENTIALS = LOGIN.length > 0 && PASSWORD.length > 0

export const SKIP_REASON =
  "Set E2E_LOGIN and E2E_PASSWORD (a Flashbacks account with catalog permissions) to run the Playwright end-to-end suite."

/** Opt-in switch for specs that delete, trash or move data. */
export const DESTRUCTIVE = process.env.E2E_DESTRUCTIVE === "1"

/** Escape hatch for running destructive flows against a non-local deployment. */
export const ALLOW_REMOTE_DESTRUCTIVE = process.env.E2E_ALLOW_REMOTE_DESTRUCTIVE === "1"

const LOCAL_HOSTS = new Set(["localhost", "127.0.0.1", "[::1]", "0.0.0.0"])

/** True when the target is a stack booted on this machine. */
export const IS_LOCAL_TARGET = (() => {
  try {
    return LOCAL_HOSTS.has(new URL(BASE_URL).hostname)
  } catch {
    return false
  }
})()

export const DESTRUCTIVE_SKIP_REASON =
  "Destructive maintenance flows only run with E2E_DESTRUCTIVE=1 against a locally booted stack (E2E_BASE_URL=http://localhost:5170)."

export const REMOTE_GUARD_REASON = `${BASE_URL} is not a local deployment: destructive flows would mutate a shared environment. Point E2E_BASE_URL at the local stack, or set E2E_ALLOW_REMOTE_DESTRUCTIVE=1 if that is really intended.`

/**
 * Throws when a destructive run is not allowed. Called by the temporary-gallery
 * fixture before it creates anything, so a refused run leaves no trace.
 */
export function assertDestructiveAllowed(): void {
  if (!DESTRUCTIVE) {
    throw new Error(`Destructive fixture requested without E2E_DESTRUCTIVE=1. ${DESTRUCTIVE_SKIP_REASON}`)
  }
  if (!IS_LOCAL_TARGET && !ALLOW_REMOTE_DESTRUCTIVE) {
    throw new Error(REMOTE_GUARD_REASON)
  }
}

/** Short random suffix so repeated runs never collide on names. */
const RUN_SUFFIX = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`

/** A unique display name for throwaway people created by a spec. */
export function uniquePersonName(label: string): string {
  return `E2E ${label} ${RUN_SUFFIX}`
}

/** A unique directory name for a throwaway gallery folder created by a spec. */
export function uniqueFolderName(label: string): string {
  return `e2e-${label}-${RUN_SUFFIX}`
}

/** A unique photo file name, so a spec never matches a photo from the library. */
export function uniqueFileName(label: string): string {
  return `e2e-${label}-${RUN_SUFFIX}.jpg`
}
