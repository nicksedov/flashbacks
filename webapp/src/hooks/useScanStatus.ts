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
    progress: "",
    filesProcessed: 0,
  }

  return {
    status: data ?? defaultStatus,
    isPolling,
    startPolling: start,
    stopPolling: stop,
    setOnScanComplete: setOnComplete,
  }
}
