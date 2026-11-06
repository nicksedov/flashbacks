/**
 * Environment inputs for the catalog walkthrough.
 *
 * The suite targets a live deployment and a real account; neither is committed.
 * Without credentials every spec skips with an explicit reason instead of
 * failing, so a plain checkout stays green.
 */
export const BASE_URL = process.env.E2E_BASE_URL ?? "https://staging.flashbacks.life"
export const LOGIN = process.env.E2E_LOGIN ?? ""
export const PASSWORD = process.env.E2E_PASSWORD ?? ""
export const HAS_CREDENTIALS = LOGIN.length > 0 && PASSWORD.length > 0

export const SKIP_REASON =
  "Set E2E_LOGIN and E2E_PASSWORD (a Flashbacks account with catalog permissions) to run the Playwright catalog walkthrough."

/** Short random suffix so repeated runs never collide on names. */
const RUN_SUFFIX = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`

/** A unique display name for throwaway people created by a spec. */
export function uniquePersonName(label: string): string {
  return `E2E ${label} ${RUN_SUFFIX}`
}
