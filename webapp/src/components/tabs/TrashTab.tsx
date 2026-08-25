import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "@/i18n"
import { ErrorBanner } from "@/components/ui/error-banner"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { ViewHeader } from "@/components/ui/view-header"
import { PaginationFooter } from "@/components/ui/pagination-footer"
import { EmptyState } from "@/components/EmptyState"
import { Trash2, FolderOpen } from "lucide-react"
import { cleanTrash, restoreTrashFile } from "@/api/endpoints"
import { useTrashItems } from "@/hooks/useTrashItems"
import { TrashTileGrid } from "@/components/trash/TrashTileGrid"
import { TrashLightbox } from "@/components/trash/TrashLightbox"
import type { TrashItemDTO } from "@/types"

export function TrashTab() {
  const { t } = useTranslation()
  const trash = useTrashItems()
  const [cleanAllOpen, setCleanAllOpen] = useState(false)
  const [isCleaning, setIsCleaning] = useState(false)
  const [lightboxItem, setLightboxItem] = useState<TrashItemDTO | null>(null)
  const [restoringId, setRestoringId] = useState<number | null>(null)
  const sentinelRef = useRef<HTMLDivElement>(null)

  // Infinite scroll observer
  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && trash.hasMore && !trash.isLoading) {
          void trash.loadMore()
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trash.hasMore, trash.isLoading, trash.loadMore])

  const handleRestore = useCallback(
    async (item: TrashItemDTO) => {
      if (restoringId !== null) return
      setRestoringId(item.id)
      try {
        await restoreTrashFile({ id: item.id })
        // Optimistic removal
        trash.removeItem(item.id)
        toast.success(t("trashTab.restored"))
      } catch (err) {
        console.error("Failed to restore:", err)
        toast.error(t("trashTab.restoreFailed"))
      } finally {
        setRestoringId(null)
      }
    },
    [restoringId, trash, t]
  )

  const handleConfirmCleanAll = useCallback(async () => {
    setIsCleaning(true)
    try {
      await cleanTrash()
      trash.reset()
      setCleanAllOpen(false)
      toast.success(t("trashTab.cleanSuccess"))
    } catch (err) {
      console.error("Failed to clean trash:", err)
      toast.error(t("trashTab.cleanFailed"))
    } finally {
      setIsCleaning(false)
    }
  }, [trash, t])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <ViewHeader
          icon={Trash2}
          textKey="trashTab.fileCount"
          textValues={{ count: trash.totalItems.toLocaleString() }}
          fallbackText={t("trashTab.fileCount", { count: "0" })}
          isLoading={trash.isLoading && !trash.initialized}
        />
        {trash.totalItems > 0 && (
          <button
            onClick={() => setCleanAllOpen(true)}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-destructive/10 hover:bg-destructive/20 text-destructive rounded transition-colors"
          >
            <Trash2 className="h-4 w-4" />
            {t("trashTab.cleanAll")}
          </button>
        )}
      </div>

      {trash.error && <ErrorBanner message={trash.error} />}

      {!trash.initialized ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      ) : trash.groups.length === 0 ? (
        <EmptyState
          bordered
          icon={FolderOpen}
          title={t("trashTab.empty")}
          description={t("trashTab.emptyHint")}
        />
      ) : (
        <>
          <TrashTileGrid
            groups={trash.groups}
            onView={setLightboxItem}
            onRestore={(item) => void handleRestore(item)}
          />

          <div ref={sentinelRef} className="h-4" />

          <PaginationFooter
            isLoading={trash.isLoading}
            hasMore={trash.hasMore}
            totalCount={trash.totalItems}
          />
        </>
      )}

      <ConfirmDialog
        open={cleanAllOpen}
        onOpenChange={(open) => !isCleaning && setCleanAllOpen(open)}
        title={t("trashTab.cleanAll")}
        description={t("trashTab.cleanAllConfirm")}
        confirmLabel={isCleaning ? t("trashTab.cleaning") : t("trashTab.cleanAll")}
        cancelLabel={t("common.cancel")}
        onConfirm={() => void handleConfirmCleanAll()}
        loading={isCleaning}
        destructive
      />

      <TrashLightbox item={lightboxItem} onOpenChange={(open) => !open && setLightboxItem(null)} />
    </div>
  )
}
