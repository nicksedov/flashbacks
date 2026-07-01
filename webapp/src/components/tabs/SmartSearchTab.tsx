import { useState, useCallback, useRef, useEffect } from "react"
import { Search, Loader2, X, AlertTriangle, Zap } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { SmartSearchResult, EmbeddingBackfillStatus } from "@/types"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import { SmartSearchTile } from "@/components/gallery/SmartSearchTile"
import { DeleteConfirmDialog } from "@/components/gallery/DeleteConfirmDialog"
import { BulkDeleteDialog } from "@/components/gallery/BulkDeleteDialog"
import { useSmartSearch } from "@/hooks/useSmartSearch"
import { fetchEmbeddingStatus, startEmbeddingBackfill, stopEmbeddingBackfill, deleteFiles } from "@/api/endpoints"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { usePolling } from "@/hooks/usePolling"
import { useSettings } from "@/providers/useSettings"
import { useGallerySelection } from "@/providers/useGallerySelection"
import { downloadImage } from "@/lib/downloadImage"

const DEBOUNCE_MS = 600
const EMBEDDING_POLL_INTERVAL = 3000

export function SmartSearchTab() {
  const { t } = useTranslation()
  const { trashDir } = useSettings()
  const { results, total, query, isLoading, error, searched, search, removeResults, reset } = useSmartSearch()
  const [selectedImage, setSelectedImage] = useState<string | null>(null)
  const [inputValue, setInputValue] = useState("")
  const [limit, setLimit] = useState(50)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Selection state
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const { registerActions } = useGallerySelection()

  // Delete confirmation dialogs
  const [deleteConfirm, setDeleteConfirm] = useState<{ id: number; fileName: string; path: string } | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [bulkDeleteResults, setBulkDeleteResults] = useState<SmartSearchResult[] | null>(null)
  const [bulkUseTrash, setBulkUseTrash] = useState(true)

  // Embedding status polling
  const {
    data: embeddingStatus,
    start: startPollingEmbeddings,
    stop: stopPollingEmbeddings,
    setOnComplete,
  } = usePolling<EmbeddingBackfillStatus>({
    pollFn: fetchEmbeddingStatus,
    interval: EMBEDDING_POLL_INTERVAL,
    onCompleteCheck: (s) => !s.running,
  })

  // Register onComplete callback after stop is available
  useEffect(() => {
    setOnComplete(() => stopPollingEmbeddings())
  }, [setOnComplete, stopPollingEmbeddings])

  // Load embedding status on mount
  useEffect(() => {
    fetchEmbeddingStatus().then((status) => {
      if (status.running) {
        startPollingEmbeddings()
      }
    }).catch(() => { /* ignore - banner won't show */ })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleStartBackfill = useCallback(async () => {
    try {
      await startEmbeddingBackfill()
      startPollingEmbeddings()
    } catch {
      // error will surface on next status poll
    }
  }, [startPollingEmbeddings])

  const handleStopBackfill = useCallback(async () => {
    try {
      await stopEmbeddingBackfill()
      stopPollingEmbeddings()
    } catch {
      // ignore
    }
  }, [stopPollingEmbeddings])

  // Debounced search
  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
    }

    if (!inputValue.trim()) {
      reset()
      return
    }

    debounceRef.current = setTimeout(() => {
      search(inputValue, limit)
    }, DEBOUNCE_MS)

    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current)
      }
    }
  }, [inputValue, limit, search, reset])

  const handleClear = useCallback(() => {
    setInputValue("")
    reset()
    setSelectedIds(new Set())
  }, [reset])

  const handleResultClick = useCallback((result: SmartSearchResult) => {
    setSelectedImage(result.path)
  }, [])

  // --- Selection handlers ---

  const handleToggleSelection = useCallback((_e: React.MouseEvent | React.KeyboardEvent, result: SmartSearchResult) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(result.id)) {
        next.delete(result.id)
      } else {
        next.add(result.id)
      }
      return next
    })
  }, [])

  const selectedCount = selectedIds.size

  const handleDeleteSelected = useCallback(() => {
    const selectedResults = results.filter((r) => selectedIds.has(r.id))
    setBulkDeleteResults(selectedResults)
    setBulkUseTrash(true)
  }, [results, selectedIds])

  // Register selection actions in context for Header display
  useEffect(() => {
    if (selectedCount > 0) {
      registerActions({
        count: selectedCount,
        clear: () => {
          setSelectedIds(new Set())
        },
        del: handleDeleteSelected,
      })
    } else {
      registerActions(null)
    }
    return () => {
      registerActions(null)
    }
  }, [selectedCount, handleDeleteSelected, registerActions])

  // --- Download handler ---

  const handleImageDownload = useCallback((result: SmartSearchResult) => {
    downloadImage(result.path, result.fileName)
  }, [])

  // --- Single delete handler ---

  const handleImageDelete = useCallback((result: SmartSearchResult) => {
    setDeleteConfirm({ id: result.id, fileName: result.fileName, path: result.path })
  }, [])

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteConfirm) return
    setIsDeleting(true)
    try {
      await deleteFiles({
        filePaths: [deleteConfirm.path],
        trashDir: trashDir || "",
      })
      removeResults([deleteConfirm.id])
      setDeleteConfirm(null)
    } catch (err) {
      console.error("Failed to delete file:", err)
      alert("Failed to delete file")
    } finally {
      setIsDeleting(false)
    }
  }, [deleteConfirm, trashDir, removeResults])

  // --- Bulk delete handler ---

  const handleConfirmBulkDelete = useCallback(async () => {
    if (!bulkDeleteResults || bulkDeleteResults.length === 0) return

    if (!bulkUseTrash || !trashDir) {
      if (!window.confirm(t("deleteFiles.confirmPermanent"))) {
        return
      }
    }

    setIsDeleting(true)
    try {
      const result = await deleteFiles({
        filePaths: bulkDeleteResults.map((r) => r.path),
        trashDir: bulkUseTrash ? trashDir : "",
      })
      removeResults(bulkDeleteResults.map((r) => r.id))
      setBulkDeleteResults(null)
      setSelectedIds(new Set())
      if (result.failed > 0) {
        alert(t("deleteFiles.successWithFailed", { count: result.success, failed: result.failed }))
      }
    } catch (err) {
      console.error("Failed to delete files:", err)
      alert(t("deleteFiles.errorFailed"))
    } finally {
      setIsDeleting(false)
    }
  }, [bulkDeleteResults, bulkUseTrash, trashDir, t, removeResults])

  const hasEmbeddings = embeddingStatus && embeddingStatus.progress.total > 0
  const needsBackfill = embeddingStatus && !embeddingStatus.running && embeddingStatus.progress.remaining > 0
  const isBackfillRunning = embeddingStatus?.running ?? false
  const backfillProgress = embeddingStatus?.progress

  return (
    <div className="space-y-4">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-bold">{t("smartSearch.title")}</h2>
        <p className="text-muted-foreground">{t("smartSearch.description")}</p>
      </div>

      {/* Embedding status banner */}
      {embeddingStatus && !hasEmbeddings && !isBackfillRunning && (
        <div className="rounded-lg border border-yellow-500/50 bg-yellow-500/10 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-yellow-600" />
            <div className="flex-1 space-y-1">
              <p className="text-sm font-medium text-yellow-700 dark:text-yellow-400">
                {t("smartSearch.noEmbeddings")}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("smartSearch.configureHint")}
              </p>
            </div>
          </div>
        </div>
      )}

      {needsBackfill && hasEmbeddings && (
        <div className="rounded-lg border border-blue-500/50 bg-blue-500/10 p-4">
          <div className="flex items-center gap-3">
            <Zap className="h-5 w-5 shrink-0 text-blue-600" />
            <p className="flex-1 text-sm text-blue-700 dark:text-blue-400">
              {t("smartSearch.needsBackfill", { count: backfillProgress?.remaining ?? 0 })}
            </p>
            <Button size="sm" onClick={handleStartBackfill}>
              {t("smartSearch.startBackfill")}
            </Button>
          </div>
        </div>
      )}

      {isBackfillRunning && backfillProgress && (
        <div className="rounded-lg border border-blue-500/50 bg-blue-500/10 p-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin text-blue-600" />
                <p className="text-sm font-medium text-blue-700 dark:text-blue-400">
                  {t("smartSearch.backfillRunning")}
                </p>
              </div>
              <Button size="sm" variant="outline" onClick={handleStopBackfill}>
                {t("smartSearch.stopBackfill")}
              </Button>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-blue-200 dark:bg-blue-900">
              <div
                className="h-full bg-blue-600 transition-all"
                style={{
                  width: backfillProgress.total > 0
                    ? `${(backfillProgress.processed / backfillProgress.total) * 100}%`
                    : "0%",
                }}
              />
            </div>
            <p className="text-xs text-muted-foreground">
              {t("smartSearch.backfillProgress", {
                processed: backfillProgress.processed,
                total: backfillProgress.total,
              })}
            </p>
            {backfillProgress.lastError && (
              <p className="text-xs text-destructive">{backfillProgress.lastError}</p>
            )}
          </div>
        </div>
      )}

      {/* Search bar */}
      <div className="relative flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder={t("smartSearch.placeholder")}
            className="pl-9 pr-9"
          />
          {inputValue && (
            <button
              onClick={handleClear}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          <span className="text-xs text-muted-foreground whitespace-nowrap">{t("smartSearch.limitLabel")}</span>
          <Input
            type="number"
            min={1}
            max={200}
            value={limit}
            onChange={(e) => {
              const val = parseInt(e.target.value, 10)
              if (!isNaN(val) && val > 0) {
                setLimit(val)
              }
            }}
            className="w-16 h-9 text-xs text-center"
          />
        </div>
      </div>

      {/* Error state */}
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      {/* Loading state */}
      {isLoading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin" />
          <span className="ml-2 text-muted-foreground">{t("smartSearch.searching")}</span>
        </div>
      )}

      {/* Results count */}
      {!isLoading && searched && total > 0 && (
        <p className="text-sm text-muted-foreground">
          {total === 1
            ? t("smartSearch.resultCountOne", { count: total })
            : t("smartSearch.resultCount", { count: total })}
          {query && <span> — "{query}"</span>}
        </p>
      )}

      {/* Empty state (searched but no results) */}
      {!isLoading && searched && total === 0 && !error && (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Search className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium">{t("smartSearch.empty")}</h3>
          <p className="text-sm text-muted-foreground mt-1">{t("smartSearch.emptyHint")}</p>
        </div>
      )}

      {/* Initial empty state */}
      {!searched && !isLoading && (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Search className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium">{t("smartSearch.startTitle")}</h3>
          <p className="text-sm text-muted-foreground mt-1">{t("smartSearch.startHint")}</p>
        </div>
      )}

      {/* Results grid */}
      {!isLoading && results.length > 0 && (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {results.map((result) => (
            <SmartSearchTile
              key={result.id}
              result={result}
              onClick={handleResultClick}
              onDownload={handleImageDownload}
              onDelete={handleImageDelete}
              selected={selectedIds.has(result.id)}
              selectionModeActive={selectedIds.size > 0}
              onSelectToggle={handleToggleSelection}
            />
          ))}
        </div>
      )}

      {/* Lightbox */}
      <UnifiedLightbox
        imagePath={selectedImage}
        initialMode="exif"
        onClose={() => setSelectedImage(null)}
      />

      {/* Single delete confirmation dialog */}
      <DeleteConfirmDialog
        fileName={deleteConfirm?.fileName}
        open={!!deleteConfirm}
        onCancel={() => setDeleteConfirm(null)}
        onConfirm={handleConfirmDelete}
        loading={isDeleting}
      />

      {/* Bulk delete dialog */}
      <BulkDeleteDialog
        count={bulkDeleteResults?.length ?? 0}
        open={!!bulkDeleteResults}
        onCancel={() => setBulkDeleteResults(null)}
        onConfirm={handleConfirmBulkDelete}
        useTrash={bulkUseTrash}
        onUseTrashChange={setBulkUseTrash}
        trashDir={trashDir}
        loading={isDeleting}
        idSuffix="-smart"
      />
    </div>
  )
}
