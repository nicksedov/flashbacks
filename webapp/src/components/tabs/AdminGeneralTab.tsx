import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { AddFolderForm } from "@/components/settings/AddFolderForm"
import { FolderList } from "@/components/settings/FolderList"
import { ScanProgressBanner } from "@/components/ScanProgressBanner"
import { useGalleryFolders } from "@/hooks/useGalleryFolders"
import { useScanStatus } from "@/hooks/useScanStatus"
import { fetchTrashInfo, cleanTrash, fetchSettings, updateSettings, fetchThumbnailCacheStats, enableThumbnailCache, disableThumbnailCache, invalidateAllThumbnails, triggerScan, triggerFastScan, fetchSyncStatus } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { RefreshCw, Trash2, Loader2, Zap, DatabaseZap, DatabaseBackup, Database, Clock } from "lucide-react"
import { useTranslation, type TranslationKey } from "@/i18n"
import type { SyncStatusResponse } from "@/types"

// Weekday order for UI: Mon, Tue, Wed, Thu, Fri, Sat, Sun (Go time.Weekday: 1,2,3,4,5,6,0)
const WEEKDAY_ORDER = [1, 2, 3, 4, 5, 6, 0] as const
const WEEKDAY_KEYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] as const

function parseSyncDaysString(s: string): boolean[] {
  const days = new Array(7).fill(false) as boolean[]
  if (!s) return days
  for (const c of s) {
    const n = Number(c)
    if (n >= 0 && n <= 6) days[n] = true
  }
  return days
}

function syncDaysToString(days: boolean[]): string {
  return days
    .map((checked, i) => (checked ? String(i) : ""))
    .filter(Boolean)
    .join(",")
}

function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return ""
  try {
    // If the string has no timezone offset (naive ISO from backend pre-formatted
    // in user's timezone), parse as local time to avoid double-conversion.
    let d: Date
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(iso)) {
      const [datePart, timePart] = iso.split("T")
      const [y, m, day] = datePart.split("-").map(Number)
      const [h, min, s] = timePart.split(":").map(Number)
      d = new Date(y, m - 1, day, h, min, s)
    } else {
      d = new Date(iso)
    }
    return d.toLocaleString()
  } catch {
    return iso
  }
}

export function AdminGeneralTab() {
  const { folders, isLoading, add, remove, refetch } = useGalleryFolders()
  const { status, startPolling, setOnScanComplete } = useScanStatus()
  const { trashDir, setTrashDir } = useSettings()
  const { t } = useTranslation()

  const [trashInput, setTrashInput] = useState(trashDir)
  const [trashFileCount, setTrashFileCount] = useState(0)
  const [trashTotalSize, setTrashTotalSize] = useState("")
  const [isCleaning, setIsCleaning] = useState(false)
  const [isSavingTrash, setIsSavingTrash] = useState(false)

  // EXIF Backup Directory state
  const [exifBackupDir, setExifBackupDir] = useState("")
  const [exifBackupInput, setExifBackupInput] = useState("")
  const [isSavingExifBackup, setIsSavingExifBackup] = useState(false)

  // Thumbnail Cache Settings state
  const [thumbnailCacheStats, setThumbnailCacheStats] = useState<{ enabled: boolean; cacheDir: string; totalFiles: number; totalSize: number } | null>(null)
  const [isThumbnailLoading, setIsThumbnailLoading] = useState(false)
  const [isSavingThumbnailCache, setIsSavingThumbnailCache] = useState(false)
  const [thumbnailCachePath, setThumbnailCachePath] = useState("")

  // Daily Sync Schedule state
  const [syncDays, setSyncDays] = useState<boolean[]>([false, true, true, true, true, true, false]) // index 0=Sun,1=Mon..6=Sat
  const [dailySyncHour, setDailySyncHour] = useState(3)
  const [dailySyncMinute, setDailySyncMinute] = useState(30)
  const [syncTimezoneOffset, setSyncTimezoneOffset] = useState(new Date().getTimezoneOffset())
  const [isSavingSchedule, setIsSavingSchedule] = useState(false)
  const [syncStatus, setSyncStatus] = useState<SyncStatusResponse | null>(null)

  const loadTrashInfo = useCallback(() => {
    fetchTrashInfo()
      .then((info) => {
        setTrashFileCount(info.fileCount)
        setTrashTotalSize(info.totalSizeHuman)
      })
      .catch(() => {
        setTrashFileCount(0)
        setTrashTotalSize("")
      })
  }, [])

  const loadThumbnailCacheStats = useCallback(async (skipPathUpdate = false) => {
    try {
      setIsThumbnailLoading(true)
      const stats = await fetchThumbnailCacheStats()
      setThumbnailCacheStats(stats)
      if (!skipPathUpdate) {
        setThumbnailCachePath(stats.cacheDir || "")
      }
    } catch {
      setThumbnailCacheStats(null)
    } finally {
      setIsThumbnailLoading(false)
    }
  }, [])

  // Load app settings to sync trashDir and thumbnailCachePath
  useEffect(() => {
    fetchSettings().then((settings) => {
      setTrashInput(settings.trashDir || "")
      setTrashDir(settings.trashDir || "")
      setExifBackupDir(settings.exifBackupDir || "")
      setExifBackupInput(settings.exifBackupDir || "")
      setThumbnailCachePath(settings.thumbnailCachePath || "")
      setSyncDays(parseSyncDaysString(settings.syncDays ?? "1,2,3,4,5"))
      setDailySyncHour(settings.dailySyncHour ?? 3)
      setDailySyncMinute(settings.dailySyncMinute ?? 30)
      if (settings.syncTimezoneOffset !== undefined) {
        setSyncTimezoneOffset(settings.syncTimezoneOffset)
      }
    }).catch(() => {
      // Use local state values
    })
  }, [setTrashDir])

  // Poll sync status — fast (1s) during sync, slow (30s) when idle.
  // Uses a ref to avoid restarting the timer when the in-progress flag
  // transitions from undefined (initial null) to false on first fetch.
  const syncInProgressRef = useRef<boolean | undefined>(undefined)
  useEffect(() => {
    const inProgress = syncStatus?.syncInProgress ?? false
    if (syncInProgressRef.current === inProgress) return // no actual change, skip
    syncInProgressRef.current = inProgress

    const interval = inProgress ? 3000 : 30000
    const load = () => fetchSyncStatus().then(setSyncStatus).catch(() => {})
    load()
    const timer = setInterval(load, interval)
    return () => clearInterval(timer)
  }, [syncStatus?.syncInProgress])

  useEffect(() => {
    loadTrashInfo()

    // Inline the initial thumbnail cache stats load to avoid calling
    // loadThumbnailCacheStats() (which sets isThumbnailLoading synchronously)
    // from within an effect body — React discourages synchronous setState in effects
    // as it triggers cascading renders. The loadThumbnailCacheStats callback itself
    // remains available for event-driven callers (button clicks, save handlers).
    const loadCacheStats = async () => {
      try {
        const stats = await fetchThumbnailCacheStats()
        setThumbnailCacheStats(stats)
        setThumbnailCachePath(stats.cacheDir || "")
      } catch {
        setThumbnailCacheStats(null)
      }
    }
    loadCacheStats()
  }, [loadTrashInfo])

  // Thumbnail Cache handlers
  const handleSaveThumbnailCachePath = useCallback(async () => {
    setIsSavingThumbnailCache(true)
    try {
      const result = await updateSettings({ thumbnailCachePath: thumbnailCachePath.trim() })
      setThumbnailCachePath(result.thumbnailCachePath || "")
      toast.success(t("adminPanel.thumbnailCache.saved"))
      // Refresh stats without overwriting the path — the saved value is authoritative
      loadThumbnailCacheStats(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("adminPanel.thumbnailCache.saveFailed"))
    } finally {
      setIsSavingThumbnailCache(false)
    }
  }, [thumbnailCachePath, setThumbnailCachePath, loadThumbnailCacheStats, t])

  const handleEnableThumbnailCache = useCallback(async () => {
    try {
      await enableThumbnailCache()
      toast.success(t("adminPanel.thumbnailCache.enabled"))
      loadThumbnailCacheStats()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("adminPanel.thumbnailCache.enableFailed"))
    }
  }, [loadThumbnailCacheStats, t])

  const handleDisableThumbnailCache = useCallback(async () => {
    try {
      await disableThumbnailCache()
      toast.success(t("adminPanel.thumbnailCache.disabled"))
      loadThumbnailCacheStats()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("adminPanel.thumbnailCache.disableFailed"))
    }
  }, [loadThumbnailCacheStats, t])

  const handleClearThumbnailCache = useCallback(async () => {
    if (!thumbnailCacheStats?.totalFiles) return
    if (!window.confirm(t("adminPanel.thumbnailCache.clearConfirm", { count: thumbnailCacheStats.totalFiles }))) return

    try {
      await invalidateAllThumbnails()
      toast.success(t("adminPanel.thumbnailCache.cleared"))
      loadThumbnailCacheStats()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("adminPanel.thumbnailCache.clearFailed"))
    }
  }, [thumbnailCacheStats, loadThumbnailCacheStats, t])

  const handleSaveSchedule = useCallback(async () => {
    setIsSavingSchedule(true)
    try {
      await updateSettings({
        syncDays: syncDaysToString(syncDays),
        dailySyncHour,
        dailySyncMinute,
        syncTimezoneOffset,
      })
      toast.success(t("settings.dailySync.saved"))
      // Refresh status after saving
      fetchSyncStatus().then(setSyncStatus).catch(() => {})
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.dailySync.saveFailed"))
    } finally {
      setIsSavingSchedule(false)
    }
  }, [syncDays, dailySyncHour, dailySyncMinute, syncTimezoneOffset, t])

  const toggleDay = useCallback((dayIndex: number) => {
    setSyncDays((prev) => {
      const next = [...prev]
      next[dayIndex] = !next[dayIndex]
      return next
    })
  }, [])

  const handleSaveTrashDir = useCallback(async () => {
    setIsSavingTrash(true)
    try {
      const result = await updateSettings({ trashDir: trashInput.trim() })
      setTrashDir(result.trashDir)
      setTrashInput(result.trashDir)
      toast.success(t("trash.saved"))
      loadTrashInfo()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("trash.saveFailed"))
    } finally {
      setIsSavingTrash(false)
    }
  }, [trashInput, setTrashDir, loadTrashInfo, t])

  const handleSaveExifBackupDir = useCallback(async () => {
    setIsSavingExifBackup(true)
    try {
      const result = await updateSettings({ exifBackupDir: exifBackupInput.trim() })
      setExifBackupDir(result.exifBackupDir)
      setExifBackupInput(result.exifBackupDir)
      toast.success(t("exifBackup.saved"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("exifBackup.saveFailed"))
    } finally {
      setIsSavingExifBackup(false)
    }
  }, [exifBackupInput, t])

  const handleCleanTrash = useCallback(async () => {
    if (trashFileCount === 0) return
    if (!window.confirm(t("trash.cleanConfirm", { count: trashFileCount }))) return

    setIsCleaning(true)
    try {
      const result = await cleanTrash()
      toast.success(t("trash.cleanSuccess", { deleted: result.deleted }))
      loadTrashInfo()
    } catch {
      toast.error(t("trash.cleanFailed"))
    } finally {
      setIsCleaning(false)
    }
  }, [trashFileCount, loadTrashInfo, t])

  const handleAdd = useCallback(
    async (path: string) => {
      try {
        const result = await add(path)
        const message = result.message as string
        toast.success(message.includes(".") ? t(message as TranslationKey) : message)
        if (result.scanStarted) {
          setOnScanComplete(() => {
            refetch()
            toast.success(t("settings.toastScanComplete"))
          })
          startPolling()
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("settings.toastAddFailed"))
      }
    },
    [add, startPolling, setOnScanComplete, refetch, t]
  )

  const handleRemove = useCallback(
    async (id: number) => {
      try {
        const result = await remove(id)
        const message = result.message as string
        toast.success(t("settings.toastFilesRemoved", { message: message.includes(".") ? t(message as TranslationKey) : message, count: result.filesRemoved }))
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("settings.toastRemoveFailed"))
      }
    },
    [remove, t]
  )

  const handleRescanAll = useCallback(async () => {
    if (folders.length === 0) {
      toast.error(t("settings.toastNoFolders"))
      return
    }
    try {
      await triggerScan()
      toast.success(t("settings.toastRescanStarted"))
      setOnScanComplete(() => {
        refetch()
        toast.success(t("settings.toastRescanComplete"))
      })
      startPolling()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.toastRescanFailed"))
    }
  }, [folders.length, startPolling, setOnScanComplete, refetch, t])

  const handleFastRescanAll = useCallback(async () => {
    if (folders.length === 0) {
      toast.error(t("settings.toastNoFolders"))
      return
    }
    try {
      const result = await triggerFastScan()
      toast.success(t("settings.toastFastScanStarted"))
      setOnScanComplete(() => {
        refetch()
        const statsMsg = [
          t("settings.fastScanStats", { unchanged: result.unchanged }),
          t("settings.fastScanModified", { modified: result.modified }),
          t("settings.fastScanCreated", { created: result.created }),
          t("settings.fastScanDeleted", { deleted: result.deleted }),
        ].join(", ")
        toast.success(t("settings.toastFastScanComplete") + " (" + statsMsg + ")")
      })
      startPolling()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.toastRescanFailed"))
    }
  }, [folders.length, startPolling, setOnScanComplete, refetch, t])

  // Generate hour options (0-23) and minute options (0, 5, 10, ..., 55)
  const hourOptions = Array.from({ length: 24 }, (_, i) => i)
  const minuteOptions = Array.from({ length: 12 }, (_, i) => i * 5)

  return (
    <div className="space-y-6">
      {/* Gallery Folder Management */}
      <Card>
        <CardHeader>
          <CardTitle>{t("settings.galleryFolders")}</CardTitle>
          <CardDescription>{t("settings.galleryFoldersDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <AddFolderForm onAdd={handleAdd} disabled={status.scanning} />

          <ScanProgressBanner status={status} />

          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-medium text-muted-foreground">
              {folders.length === 1
                ? t("settings.folderCountOne", { count: folders.length })
                : t("settings.folderCount", { count: folders.length })}
            </h3>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleFastRescanAll}
                disabled={status.scanning || folders.length === 0}
              >
                <Zap className={`mr-1.5 h-3.5 w-3.5 ${status.scanning ? "animate-spin" : ""}`} />
                {t("settings.fastScanChanges")}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={handleRescanAll}
                disabled={status.scanning || folders.length === 0}
              >
                <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${status.scanning ? "animate-spin" : ""}`} />
                {t("settings.rescanAll")}
              </Button>
            </div>
          </div>

          <FolderList
            folders={folders}
            onRemove={handleRemove}
            isLoading={isLoading}
          />
        </CardContent>
      </Card>

      {/* Sync Schedule */}
      <Card>
        <CardHeader>
          <CardTitle>{t("settings.dailySync.title")}</CardTitle>
          <CardDescription>{t("settings.dailySync.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Day checkboxes */}
          <div className="space-y-2">
            <Label>{t("settings.dailySync.days")}</Label>
            <div className="flex flex-wrap gap-3">
              {WEEKDAY_ORDER.map((dayIdx, uiIdx) => (
                <div key={dayIdx} className="flex items-center space-x-1.5">
                  <Checkbox
                    id={`sync-day-${dayIdx}`}
                    checked={syncDays[dayIdx]}
                    onCheckedChange={() => toggleDay(dayIdx)}
                  />
                  <Label htmlFor={`sync-day-${dayIdx}`} className="text-sm cursor-pointer">
                    {t(`settings.dailySync.day.${WEEKDAY_KEYS[uiIdx]}` as TranslationKey)}
                  </Label>
                </div>
              ))}
            </div>
          </div>

          {/* Time selectors */}
          <div className="flex items-center gap-4">
            <div className="space-y-2">
              <Label htmlFor="daily-sync-hour">{t("settings.dailySync.hour")}</Label>
              <Select
                value={String(dailySyncHour)}
                onValueChange={(val) => setDailySyncHour(Number(val))}
              >
                <SelectTrigger id="daily-sync-hour" className="w-24">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {hourOptions.map((h) => (
                    <SelectItem key={h} value={String(h)}>
                      {String(h).padStart(2, "0")}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="daily-sync-minute">{t("settings.dailySync.minute")}</Label>
              <Select
                value={String(dailySyncMinute)}
                onValueChange={(val) => setDailySyncMinute(Number(val))}
              >
                <SelectTrigger id="daily-sync-minute" className="w-24">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {minuteOptions.map((m) => (
                    <SelectItem key={m} value={String(m)}>
                      {String(m).padStart(2, "0")}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-end">
              <Button
                onClick={handleSaveSchedule}
                disabled={isSavingSchedule}
                size="sm"
              >
                {isSavingSchedule ? t("common.saving") : t("settings.dailySync.save")}
              </Button>
            </div>
          </div>

          {/* Sync Status */}
          <div className="rounded-md border p-3 space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Clock className="h-4 w-4" />
              {t("settings.dailySync.status")}
              {syncStatus?.syncInProgress && (
                <span className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  {t("settings.dailySync.statusRunning")}
                </span>
              )}
            </div>

            {/* Progress bar during sync */}
            {syncStatus?.syncInProgress && syncStatus.totalFiles > 0 && (
              <div className="space-y-1">
                <div className="h-2 w-full rounded-full bg-muted">
                  <div
                    className="h-2 rounded-full bg-blue-500 transition-all duration-300"
                    style={{ width: `${Math.round((syncStatus.processedFiles / syncStatus.totalFiles) * 100)}%` }}
                  />
                </div>
                <p className="text-xs text-muted-foreground">
                  {t("settings.dailySync.syncProgress", {
                    processed: syncStatus.processedFiles,
                    total: syncStatus.totalFiles,
                  })}
                </p>
              </div>
            )}

            <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm text-muted-foreground">
              <div>
                <span className="font-medium text-foreground">{t("settings.dailySync.lastRun")}: </span>
                {syncStatus?.lastSyncAt
                  ? formatDateTime(syncStatus.lastSyncAt)
                  : t("settings.dailySync.lastRunNever")}
              </div>
              <div>
                <span className="font-medium text-foreground">{t("settings.dailySync.nextRun")}: </span>
                {syncStatus?.syncInProgress
                  ? t("settings.dailySync.statusRunning")
                  : syncStatus?.nextRunAt
                    ? formatDateTime(syncStatus.nextRunAt)
                    : "—"}
              </div>
              {/* Show stats when sync is in progress or after last sync completed */}
              {(syncStatus?.syncInProgress || syncStatus?.lastSyncAt) && (
                <div className="col-span-2">
                  {t("settings.dailySync.lastStats", {
                    newFiles: syncStatus.lastSyncNew,
                    updatedFiles: syncStatus.lastSyncUpdated,
                    deletedFiles: syncStatus.lastSyncDeleted,
                    thumbnails: syncStatus.lastSyncThumbnails,
                  })}
                </div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Trash Settings */}
      <Card>
        <CardHeader>
          <CardTitle>{t("trash.title")}</CardTitle>
          <CardDescription>{t("trash.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="trash-dir-input">{t("trash.dirLabel")}</Label>
            <div className="flex gap-2">
              <Input
                id="trash-dir-input"
                placeholder={t("trash.dirPlaceholder")}
                value={trashInput}
                onChange={(e) => setTrashInput(e.target.value)}
                className="flex-1"
              />
              <Button
                onClick={handleSaveTrashDir}
                disabled={isSavingTrash || trashInput === trashDir}
                size="default"
              >
                {isSavingTrash ? t("trash.saving") : t("trash.save")}
              </Button>
            </div>
          </div>

          <div className="flex items-center justify-between rounded-md border p-3">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Trash2 className="h-4 w-4" />
              {!trashDir ? (
                <span>{t("trash.notConfigured")}</span>
              ) : trashFileCount === 0 ? (
                <span>{t("trash.empty")}</span>
              ) : (
                <span>{t("trash.fileCountWithSize", { count: trashFileCount, size: trashTotalSize })}</span>
              )}
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleCleanTrash}
              disabled={isCleaning || trashFileCount === 0 || !trashDir}
            >
              {isCleaning ? t("trash.cleaning") : t("trash.cleanButton")}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* EXIF Backup Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <DatabaseBackup className="h-5 w-5" />
            {t("exifBackup.title")}
          </CardTitle>
          <CardDescription>{t("exifBackup.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="exif-backup-dir-input">{t("exifBackup.dirLabel")}</Label>
            <div className="flex gap-2">
              <Input
                id="exif-backup-dir-input"
                placeholder={t("exifBackup.dirPlaceholder")}
                value={exifBackupInput}
                onChange={(e) => setExifBackupInput(e.target.value)}
                className="flex-1"
              />
              <Button
                onClick={handleSaveExifBackupDir}
                disabled={isSavingExifBackup || exifBackupInput === exifBackupDir}
                size="default"
              >
                {isSavingExifBackup ? t("exifBackup.saving") : t("exifBackup.save")}
              </Button>
            </div>
          </div>

          {!exifBackupDir && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <DatabaseBackup className="h-4 w-4" />
              <span>{t("exifBackup.notConfigured")}</span>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Thumbnail Cache Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <DatabaseZap className="h-5 w-5" />
            {t("adminPanel.thumbnailCache.title")}
          </CardTitle>
          <CardDescription>{t("adminPanel.thumbnailCache.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Cache Status */}
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-1">
              <div className="text-sm font-medium">{t("adminPanel.thumbnailCache.status")}</div>
              {isThumbnailLoading ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t("common.loading")}
                </div>
              ) : thumbnailCacheStats ? (
                <div className="space-y-1">
                  <div className="flex items-center gap-2 text-sm">
                    <span
                      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        thumbnailCacheStats.enabled
                          ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                          : "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                      }`}
                    >
                      {thumbnailCacheStats.enabled
                        ? t("adminPanel.thumbnailCache.enabled")
                        : t("adminPanel.thumbnailCache.disabled")}
                    </span>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t("adminPanel.thumbnailCache.stats", {
                      files: thumbnailCacheStats.totalFiles,
                      size: thumbnailCacheStats.totalSize,
                    })}
                  </p>
                </div>
              ) : (
                <div className="text-sm text-muted-foreground">{t("adminPanel.thumbnailCache.notConfigured")}</div>
              )}
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => loadThumbnailCacheStats()}
              disabled={isThumbnailLoading}
            >
              {isThumbnailLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </Button>
          </div>

          {/* Cache Path */}
          <div className="space-y-2">
            <Label htmlFor="thumbnail-cache-path">
              {t("adminPanel.thumbnailCache.pathLabel")}
            </Label>
            <div className="flex gap-2">
              <Input
                id="thumbnail-cache-path"
                placeholder={t("adminPanel.thumbnailCache.pathPlaceholder")}
                value={thumbnailCachePath}
                onChange={(e) => setThumbnailCachePath(e.target.value)}
                className="flex-1"
              />
              <Button
                onClick={handleSaveThumbnailCachePath}
                disabled={isSavingThumbnailCache || thumbnailCachePath === (thumbnailCacheStats?.cacheDir || "")}
                size="default"
              >
                {isSavingThumbnailCache ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </div>

          {/* Cache Actions */}
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleClearThumbnailCache}
              disabled={thumbnailCacheStats?.enabled !== true || thumbnailCacheStats.totalFiles === 0}
            >
              <Trash2 className="mr-1.5 h-3.5 w-3.5" />
              {t("adminPanel.thumbnailCache.clearButton")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleEnableThumbnailCache}
              disabled={thumbnailCacheStats?.enabled === true}
            >
              <DatabaseBackup className="mr-1.5 h-3.5 w-3.5" />
              {t("adminPanel.thumbnailCache.enableButton")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleDisableThumbnailCache}
              disabled={thumbnailCacheStats?.enabled !== true}
            >
              <Database className="mr-1.5 h-3.5 w-3.5" />
              {t("adminPanel.thumbnailCache.disableButton")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
