import { useCallback, useEffect, useState } from "react"
import { GalleryImageGrid } from "@/components/gallery/GalleryImageGrid"
import { useGalleryImages } from "@/hooks/useGalleryImages"
import { useGalleryFolders } from "@/hooks/useGalleryFolders"
import { Skeleton } from "@/components/ui/skeleton"
import { ImageIcon, ArrowDown, ArrowUp, Search, X } from "lucide-react"
import { useTranslation } from "@/i18n"
import { useIntersectionObserver } from "@/hooks/useIntersectionObserver"
import { PaginationFooter } from "@/components/ui/pagination-footer"
import { ViewHeader } from "@/components/ui/view-header"
import { useGallerySelection } from "@/providers/useGallerySelection"
import { BulkMoveDialog } from "@/components/gallery/BulkMoveDialog"
import { moveFiles } from "@/api/endpoints"
import type { GalleryImageDTO } from "@/types"

interface GalleryFoldersViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
  onBulkDelete?: (selectedImages: GalleryImageDTO[], cleanup: () => void) => void
}

export function GalleryFoldersView({ onImageClick, onImageDownload, onImageDelete, onBulkDelete }: GalleryFoldersViewProps) {
  const [sortOrder, setSortOrder] = useState<"newest" | "oldest">("newest")
  const [searchInput, setSearchInput] = useState("")
  const [searchQuery, setSearchQuery] = useState("")
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const { images, totalImages, hasMore, isLoading, error, initialized, loadMore, removeImage } =
    useGalleryImages("folders", sortOrder, searchQuery || undefined)
  const { folders: rootFolders } = useGalleryFolders()
  const { t } = useTranslation()
  const { registerActions } = useGallerySelection()

  // Debounce search input (500ms delay)
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput)
    }, 500)

    return () => clearTimeout(timer)
  }, [searchInput])

  const sentinelRef = useIntersectionObserver({
    onIntersect: loadMore,
    enabled: hasMore && !isLoading,
    dependencies: [hasMore, isLoading, loadMore],
  })

  useEffect(() => {
    if (!initialized && !isLoading) {
      loadMore()
    }
  }, [initialized, isLoading, loadMore])

  const handleSortToggle = () => {
    setSortOrder(prev => prev === "newest" ? "oldest" : "newest")
  }

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value)
  }

  const handleClearSearch = () => {
    setSearchInput("")
    setSearchQuery("")
  }

  const handleToggleSelection = useCallback((image: GalleryImageDTO) => {
    const next = new Set(selectedIds)
    if (next.has(image.id)) {
      next.delete(image.id)
    } else {
      next.add(image.id)
    }
    setSelectedIds(next)
  }, [selectedIds])

  const handleImageClick = useCallback((image: GalleryImageDTO) => {
    onImageClick(image)
  }, [onImageClick])

  const handleRangeSelection = useCallback((startImage: GalleryImageDTO, endImage: GalleryImageDTO) => {
    if (startImage.dirPath !== endImage.dirPath) return

    const folderImages = images.filter((img) => img.dirPath === endImage.dirPath)
    const startIndex = folderImages.findIndex((img) => img.id === startImage.id)
    const endIndex = folderImages.findIndex((img) => img.id === endImage.id)
    if (startIndex === -1 || endIndex === -1) return

    const [minIndex, maxIndex] = startIndex < endIndex ? [startIndex, endIndex] : [endIndex, startIndex]
    const next = new Set(selectedIds)
    for (let i = minIndex; i <= maxIndex; i++) {
      next.add(folderImages[i].id)
    }
    setSelectedIds(next)
  }, [images, selectedIds])

  const handleDeleteSelected = useCallback(() => {
    const selectedImages = images.filter((img) => selectedIds.has(img.id))
    const cleanup = () => {
      for (const img of selectedImages) {
        removeImage(img.id)
      }
      setSelectedIds(new Set())
    }
    onBulkDelete?.(selectedImages, cleanup)
  }, [images, selectedIds, removeImage, onBulkDelete])

  // Move state
  const [moveDialogOpen, setMoveDialogOpen] = useState(false)
  const [isMoving, setIsMoving] = useState(false)

  const handleMoveSelected = useCallback(() => {
    setMoveDialogOpen(true)
  }, [])

  const handleConfirmMove = useCallback(async (targetDir: string) => {
    const selectedImages = images.filter((img) => selectedIds.has(img.id))
    if (selectedImages.length === 0) return

    setIsMoving(true)
    try {
      const result = await moveFiles({
        filePaths: selectedImages.map((img) => img.path),
        targetDir,
      })
      // Remove moved files from view
      for (const img of selectedImages) {
        removeImage(img.id)
      }
      setSelectedIds(new Set())
      setMoveDialogOpen(false)
      if (result.failed > 0) {
        alert(t("moveFiles.successWithFailed", { count: result.success, failed: result.failed }))
      }
    } catch (err) {
      console.error("Failed to move files:", err)
      alert(t("moveFiles.errorFailed"))
    } finally {
      setIsMoving(false)
    }
  }, [images, selectedIds, removeImage, t])

  const selectedCount = selectedIds.size

  // Register selection actions in context for Header display
  useEffect(() => {
    if (selectedCount > 0) {
      registerActions({
        count: selectedCount,
        clear: () => {
          setSelectedIds(new Set())
        },
        del: handleDeleteSelected,
        move: handleMoveSelected,
      })
    } else {
      registerActions(null)
    }
    return () => {
      registerActions(null)
    }
  }, [selectedCount, handleDeleteSelected, handleMoveSelected, registerActions])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <ViewHeader
          icon={ImageIcon}
          textKey={totalImages === 1 ? "gallery.imageCountOne" : "gallery.imageCount"}
          textValues={{ count: totalImages.toLocaleString() }}
          isLoading={!initialized}
        />

        {/* Right side: search and sort */}
        <div className="flex items-center gap-2">
          {/* Search input */}
          <div className="relative">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              value={searchInput}
              onChange={handleSearchChange}
              placeholder={t("gallery.search.placeholder")}
              className="h-9 w-70 rounded-md border bg-background pl-8 pr-8 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            />
            {searchInput && (
              <button
                onClick={handleClearSearch}
                className="absolute right-1 top-1/2 -translate-y-1/2 h-6 w-6 rounded-sm hover:bg-accent flex items-center justify-center"
                title={t("gallery.search.clear")}
              >
                <X className="h-3 w-3 text-muted-foreground" />
              </button>
            )}
          </div>

          {/* Sort button */}
          <button
            onClick={handleSortToggle}
            className="inline-flex items-center gap-2 rounded-md bg-transparent px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
            title={sortOrder === "newest" ? t("gallery.sortNewest") : t("gallery.sortOldest")}
          >
            {sortOrder === "newest" ? (
              <ArrowDown className="h-4 w-4" />
            ) : (
              <ArrowUp className="h-4 w-4" />
            )}
            <span>{sortOrder === "newest" ? t("gallery.sortNewest") : t("gallery.sortOldest")}</span>
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {!initialized ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      ) : images.length === 0 && !isLoading ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <ImageIcon className="mx-auto h-10 w-10 text-muted-foreground/50" />
          <p className="mt-2 text-sm font-medium text-muted-foreground">
            {t("gallery.empty")}
          </p>
          <p className="text-xs text-muted-foreground/70">
            {t("gallery.emptyHint")}
          </p>
        </div>
      ) : (
        <>
          {searchQuery && images.length === 0 && !isLoading ? (
            <div className="rounded-lg border border-dashed p-12 text-center">
              <Search className="mx-auto h-10 w-10 text-muted-foreground/50" />
              <p className="mt-2 text-sm font-medium text-muted-foreground">
                {t("gallery.search.noResults", { query: searchQuery })}
              </p>
              <p className="text-xs text-muted-foreground/70">
                {t("gallery.search.noResultsHint")}
              </p>
            </div>
          ) : (
            <>
              {searchQuery && (
                <div className="text-xs text-muted-foreground px-0.5">
                  {t("gallery.search.resultsCount", { shown: images.length, total: totalImages })}
                </div>
              )}
              <GalleryImageGrid
                images={images}
                onImageClick={handleImageClick}
                onImageDownload={onImageDownload}
                onImageDelete={(image) => onImageDelete?.(image, () => removeImage(image.id))}
                rootFolders={rootFolders}
                selectedIds={selectedIds}
                selectionModeActive={selectedIds.size > 0}
                onToggleSelection={handleToggleSelection}
                onRangeSelection={handleRangeSelection}
              />

              <div ref={sentinelRef} className="h-4" />

              <PaginationFooter
                isLoading={isLoading}
                hasMore={hasMore}
                totalCount={totalImages}
              />
            </>
          )}
        </>
      )}
      {/* Bulk move dialog */}
      <BulkMoveDialog
        count={selectedIds.size}
        open={moveDialogOpen}
        onCancel={() => setMoveDialogOpen(false)}
        onConfirm={handleConfirmMove}
        loading={isMoving}
      />
    </div>
  )
}
