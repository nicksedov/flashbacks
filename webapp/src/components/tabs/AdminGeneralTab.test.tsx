import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { AdminGeneralTab } from "./AdminGeneralTab"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/hooks/useGalleryFolders", () => ({
  useGalleryFolders: () => ({
    folders: [],
    isLoading: false,
    add: vi.fn(),
    remove: vi.fn(),
    refetch: vi.fn(),
  }),
}))

vi.mock("@/hooks/useScanStatus", () => ({
  useScanStatus: () => ({
    status: {
      scanning: false,
      phase: "",
      progress: "",
      currentDirectory: "",
      totalFiles: 0,
      filesProcessed: 0,
      filesAdded: 0,
      filesDeleted: 0,
      filesUpdated: 0,
    },
    startPolling: vi.fn(),
    setOnScanComplete: vi.fn(),
  }),
}))

vi.mock("@/providers/useSettings", () => ({
  useSettings: () => ({ trashDir: "", setTrashDir: vi.fn() }),
}))

vi.mock("@/api/endpoints", () => ({
  fetchTrashInfo: vi.fn().mockResolvedValue({ fileCount: 0, totalSizeHuman: "" }),
  cleanTrash: vi.fn().mockResolvedValue({ deleted: 0 }),
  fetchSettings: vi.fn().mockResolvedValue({
    trashDir: "",
    exifBackupDir: "",
    thumbnailCachePath: "",
    syncDays: "1,2,3,4,5",
    dailySyncHour: 3,
    dailySyncMinute: 30,
    syncTimezoneOffset: 0,
  }),
  updateSettings: vi.fn(),
  fetchThumbnailCacheStats: vi.fn().mockResolvedValue({
    enabled: false,
    cacheDir: "",
    totalFiles: 0,
    totalSize: 0,
  }),
  enableThumbnailCache: vi.fn(),
  disableThumbnailCache: vi.fn(),
  invalidateAllThumbnails: vi.fn(),
  triggerScan: vi.fn(),
  triggerFastScan: vi.fn(),
  fetchSyncStatus: vi.fn().mockResolvedValue({ running: false, syncInProgress: false }),
}))

vi.mock("@/components/settings/SyncHistoryDialog", () => ({
  SyncHistoryDialog: () => null,
}))

describe("AdminGeneralTab gallery folders", () => {
  it("groups the add and scan actions into a single row", () => {
    render(<AdminGeneralTab />)

    const addButton = screen.getByRole("button", { name: "addFolder.button" })
    const fastScanButton = screen.getByRole("button", { name: "settings.fastScanChanges" })
    const rescanButton = screen.getByRole("button", { name: "settings.rescanAll" })

    const row = addButton.parentElement
    expect(row).not.toBeNull()
    expect(fastScanButton.parentElement).toBe(row)
    expect(rescanButton.parentElement).toBe(row)
    expect(hasClass(row, "flex-wrap")).toBe(true)
  })
})
