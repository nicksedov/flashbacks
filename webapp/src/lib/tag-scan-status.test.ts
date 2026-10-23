import { describe, expect, it } from "vitest"
import { deriveTagScanState } from "./tag-scan-status"
import type { TagScanStatusResponse } from "@/types"

// status builds a tag scan status payload with sane defaults for one assertion.
function status(overrides: Partial<TagScanStatusResponse>): TagScanStatusResponse {
  return {
    running: true,
    paused: false,
    enabled: true,
    schedule: "23:00 - 07:00",
    windowOpen: true,
    scanned: 0,
    remaining: 0,
    total: 0,
    ...overrides,
  }
}

describe("deriveTagScanState", () => {
  it("reports running inside the schedule window", () => {
    expect(deriveTagScanState(status({ running: true, windowOpen: true }))).toBe("running")
  })

  it("reports waiting outside the schedule window even while the loop is started", () => {
    expect(deriveTagScanState(status({ running: true, windowOpen: false }))).toBe("waiting")
  })

  it("keeps an explicit pause over the schedule state", () => {
    expect(deriveTagScanState(status({ paused: true, windowOpen: false }))).toBe("paused")
    expect(deriveTagScanState(status({ paused: true, windowOpen: true }))).toBe("paused")
  })

  it("reports stopped when the manager is not running inside an open window", () => {
    expect(deriveTagScanState(status({ running: false, windowOpen: true }))).toBe("stopped")
  })
})
