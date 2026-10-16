import { fetchScanStatus } from "@/api/endpoints"
import type { ScanStatusResponse } from "@/types"
import { SCAN_POLL_INTERVAL } from "@/lib/constants"
import { usePolling } from "./usePolling"

export function useScanStatus() {
  const { data, isPolling, start, stop, setOnComplete } = usePolling<ScanStatusResponse>({
    pollFn: fetchScanStatus,
    interval: SCAN_POLL_INTERVAL,
    onCompleteCheck: (result) => !result.scanning,
  })

  const defaultStatus: ScanStatusResponse = {
    scanning: false,
    phase: "",
    progress: "",
    currentDirectory: "",
    totalFiles: 0,
    filesProcessed: 0,
    filesAdded: 0,
    filesDeleted: 0,
    filesUpdated: 0,
  }

  return {
    status: data ?? defaultStatus,
    isPolling,
    startPolling: start,
    stopPolling: stop,
    setOnScanComplete: setOnComplete,
  }
}
