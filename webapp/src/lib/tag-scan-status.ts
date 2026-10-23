import type { TagScanStatusResponse } from "@/types"

/** States of the automatic image tagging job shown in the Analysis tab. */
export type TagScanState = "running" | "paused" | "waiting" | "stopped"

/**
 * deriveTagScanState maps the backend tag scan status onto the status the
 * administrator sees. An explicit pause wins over the schedule; outside the
 * configured schedule window the job is waiting rather than running, because the
 * scan loop only works inside the selected weekdays and hours.
 */
export function deriveTagScanState(status: TagScanStatusResponse): TagScanState {
  if (status.paused) return "paused"
  if (!status.windowOpen) return "waiting"
  return status.running ? "running" : "stopped"
}
