import { useCallback, useMemo, useState } from "react"
import { toast } from "sonner"
import { Toolbar } from "@/components/layout/Toolbar"
import { DuplicateGroupList } from "@/components/duplicates/DuplicateGroupList"
import { Pagination } from "@/components/pagination/Pagination"
import { EmptyState } from "@/components/EmptyState"
import { ScanProgressBanner } from "@/components/ScanProgressBanner"
import { DeleteFilesModal } from "@/components/modals/DeleteFilesModal"
import { BatchDeduplicationModal } from "@/components/modals/BatchDeduplicationModal"
import { useDuplicates } from "@/hooks/useDuplicates"
import { useSelection } from "@/hooks/useSelection"
import { useScanStatus } from "@/hooks/useScanStatus"
import { triggerScan } from "@/api/endpoints"
import { DEFAULT_PAGE_SIZE } from "@/lib/constants"
import { Skeleton } from "@/components/ui/skeleton"
import { useTranslation } from "@/i18n"
import type { FileDTO } from "@/types"

export function DeduplicationTab() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const { data, isLoading, error, refetch } = useDuplicates(page, pageSize)
  const selection = useSelection()
  const { status, startPolling, setOnScanComplete } = useScanStatus()
  const { t } = useTranslation()

  // Modals
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [batchModalOpen, setBatchModalOpen] = useState(false)

  // Collect all files from current page for folder selection
  const allFiles: FileDTO[] = useMemo(() => {
    if (!data) return []
    return data.groups.flatMap((g) => g.files)
  }, [data])

  const handleRescan = useCallback(async () => {
    try {
      await triggerScan()
      toast.success(t("dedup.toastScanStarted"))
      setOnScanComplete(() => {
        refetch()
        selection.reset()
        toast.success(t("dedup.toastScanComplete"))
      })
      startPolling()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("dedup.toastScanFailed"))
    }
  }, [startPolling, setOnScanComplete, refetch, selection, t])

  const handlePageSizeChange = useCallback((size: number) => {
    setPageSize(size)
    setPage(1)
  }, [])

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  const handleSelectFolder = useCallback(
    (dirPath: string, files: FileDTO[]) => {
      selection.selectByFolder(dirPath, files)
    },
    [selection]
  )

  const handleMutationComplete = useCallback(() => {
    selection.reset()
    refetch()
  }, [selection, refetch])

  const handleSuccess = useCallback((message: string) => {
    toast.success(message)
  }, [])

  const handleError = useCallback((message: string) => {
    toast.error(message)
  }, [])

  return (
    <div className="space-y-4">
      <Toolbar
        selectedCount={selection.selectedCount}
        pageSize={pageSize}
        onPageSizeChange={handlePageSizeChange}
        onRescan={handleRescan}
        onResetSelection={selection.reset}
        onOpenDeleteFiles={() => {
          if (selection.selectedCount === 0) {
            toast.error(t("dedup.toastSelectFile"))
            return
          }
          setDeleteModalOpen(true)
        }}
        onOpenBatchDedup={() => setBatchModalOpen(true)}
        isScanning={status.scanning}
      />

      <ScanProgressBanner status={status} />

      {error && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      ) : data && data.groups.length > 0 ? (
        <>
          <DuplicateGroupList
            groups={data.groups}
            allFiles={allFiles}
            isSelected={selection.isSelected}
            onToggleFile={selection.toggle}
            onSelectFolder={handleSelectFolder}
          />
          <Pagination
            currentPage={data.currentPage}
            totalPages={data.totalPages}
            hasPrevPage={data.hasPrevPage}
            hasNextPage={data.hasNextPage}
            onPageChange={handlePageChange}
          />
        </>
      ) : (
        <EmptyState />
      )}

      <DeleteFilesModal
        open={deleteModalOpen}
        onOpenChange={setDeleteModalOpen}
        selectedPaths={selection.selectedPaths}
        onSuccess={handleSuccess}
        onError={handleError}
        onComplete={handleMutationComplete}
      />

      <BatchDeduplicationModal
        open={batchModalOpen}
        onOpenChange={setBatchModalOpen}
        onSuccess={handleSuccess}
        onError={handleError}
        onComplete={handleMutationComplete}
      />
    </div>
  )
}
