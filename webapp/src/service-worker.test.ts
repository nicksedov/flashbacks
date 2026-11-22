import { readFileSync, existsSync } from "node:fs"
import path from "node:path"
import { describe, it, expect } from "vitest"

/**
 * Locate the webapp package. The webapp test run starts in `webapp/`, while the
 * root focused-spec Vitest project starts in the repository root, so try both.
 */
function findWebappDir(): string {
  const candidates = [process.cwd(), path.resolve(process.cwd(), "webapp")]
  for (const candidate of candidates) {
    if (existsSync(path.join(candidate, "public", "sw.js"))) return candidate
  }
  throw new Error("could not locate the webapp package directory")
}

const sw = readFileSync(path.join(findWebappDir(), "public", "sw.js"), "utf8")

/** The body of the `/api/` branch of the service worker fetch handler. */
function apiBranch(): string {
  const start = sw.indexOf("url.pathname.startsWith('/api/')")
  expect(start).toBeGreaterThan(-1)
  const open = sw.indexOf("{", start)
  const close = sw.indexOf("}", open)
  expect(open).toBeGreaterThan(-1)
  return sw.slice(open, close + 1)
}

describe("service worker", () => {
  it("serves API requests from the network without touching the cache", () => {
    const branch = apiBranch()

    expect(branch).toContain("fetch(request)")
    expect(branch).not.toContain("caches.")
    expect(branch).not.toContain("cache.")
  })

  it("bumps the cache version so earlier cached payloads are purged", () => {
    expect(sw).toContain("const CACHE_NAME = 'flashbacks-v4';")
    expect(sw).not.toContain("'flashbacks-v3'")
  })
})
