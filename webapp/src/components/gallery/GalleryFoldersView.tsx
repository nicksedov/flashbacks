import { useCallback, useEffect, useState } from "react"
import {
  Folder,
  FolderOpen,
  ArrowUp,
  ChevronRight,
  Home,
  ImageIcon,
} from "lucide-react"
import { useGalleryFolders } from "@/hooks/useGalleryFolders"
import { useGalleryImages } from "@/hooks/useGalleryImages"
import { useIntersectionObserver } from "@/hooks/useIntersectionObserver"
import { fetchSubdirs } from "@/api/endpoints"
import { useTranslation } from "@/i18n"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import type { GalleryImageDTO, SubdirEntry } from "@/types"

interface GalleryFoldersViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
}

interface BreadcrumbSegment {
  name: string
  path: string
}

/**
 * File-manager style folder browser for the gallery.
 * Shows root gallery folders initially, then allows drilling into subdirectories.
 * Displays folders first, then image thumbnails.
 */
export function GalleryFoldersView(props: GalleryFoldersViewProps) {
  const { onImageClick } = props
  const { t } = useTranslation()
  const { folders: rootFolders } = useGalleryFolders()

  // Current directory path: null = root (gallery folders)
  const [currentPath, setCurrentPath] = useState<string | null>(null)

  // Subdirectories in current path
  const [subdirs, setSubdirs] = useState<SubdirEntry[]>([])
  const [subdirsLoading, setSubdirsLoading] = useState(false)

  // Images in current path (using the gallery images hook with dirPath filter)
  const {
    images,
    totalImages,
    hasMore,
    isLoading: imagesLoading,
    error: imagesError,
    initialized: imagesInitialized,
    loadMore,
  } = useGalleryImages("folders", "newest", undefined, currentPath ?? undefined)

  // Clear subdirs when going back to root
  useEffect(() => {
    if (currentPath === null) {
      setSubdirs([])
      setSubdirsLoading(false)
    }
  }, [currentPath])

  // Load subdirectories when currentPath changes to a non-null value
  useEffect(() => {
    if (currentPath === null) return
    let cancelled = false
    setSubdirsLoading(true)
    fetchSubdirs(currentPath)
      .then((res) => {
        if (!cancelled) setSubdirs(res.subdirs)
      })
      .catch(() => {
        if (!cancelled) setSubdirs([])
      })
      .finally(() => {
        if (!cancelled) setSubdirsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [currentPath])

  // Compute breadcrumbs from current path
  const breadcrumbs: BreadcrumbSegment[] = []
  if (currentPath !== null) {
    const parts = currentPath.split("/").filter(Boolean)
    let acc = ""
    for (const part of parts) {
      acc += "/" + part
      breadcrumbs.push({ name: part, path: acc })
    }
  }

  // Navigate into a folder
  const handleEnterFolder = useCallback((path: string) => {
    setCurrentPath(path)
  }, [])

  // Navigate up one level
  const handleGoUp = useCallback(() => {
    if (currentPath === null) return
    const parts = currentPath.split("/").filter(Boolean)
    if (parts.length <= 1) {
      setCurrentPath(null)
    } else {
      parts.pop()
      setCurrentPath("/" + parts.join("/"))
    }
  }, [currentPath])

  // Navigate to a breadcrumb segment
  const handleBreadcrumbClick = useCallback((path: string) => {
    setCurrentPath(path)
  }, [])

  // Navigate to root
  const handleGoRoot = useCallback(() => {
    setCurrentPath(null)
  }, [])

  // Load initial images
  useEffect(() => {
    if (!imagesInitialized && !imagesLoading) {
      loadMore()
    }
  }, [imagesInitialized, imagesLoading, loadMore])

  // Intersection observer sentinel for automatic infinite scroll
  const sentinelRef = useIntersectionObserver({
    onIntersect: loadMore,
    enabled: hasMore && !imagesLoading,
    dependencies: [hasMore, imagesLoading, loadMore],
  })

  const isLoading = (currentPath !== null && subdirsLoading) || !imagesInitialized

  return (
    <div className="space-y-4">
      {/* Breadcrumbs + Up button bar */}
      <div className="flex items-center gap-2 flex-wrap min-h-9">
        {/* Root home button */}
        <button
          type="button"
          onClick={handleGoRoot}
          className={cn(
            "inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors",
            currentPath === null
              ? "bg-primary/10 text-primary font-medium"
              : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          )}
          title={t("gallery.folders.root")}
        >
          <Home className="h-4 w-4" />
        </button>

        {/* Breadcrumbs */}
        {breadcrumbs.map((segment, idx) => {
          const isLast = idx === breadcrumbs.length - 1
          return (
            <span key={segment.path} className="flex items-center gap-1">
              <ChevronRight className="h-3 w-3 text-muted-foreground flex-shrink-0" />
              <button
                type="button"
                onClick={() => handleBreadcrumbClick(segment.path)}
                className={cn(
                  "rounded-md px-2 py-1 text-sm transition-colors truncate max-w-[200px]",
                  isLast
                    ? "bg-primary/10 text-primary font-medium"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                )}
                title={segment.path}
              >
                {segment.name}
              </button>
            </span>
          )
        })}

        {/* Up button (only when inside a folder) */}
        {currentPath !== null && (
          <button
            type="button"
            onClick={handleGoUp}
            className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors ml-auto"
            title={t("gallery.folders.upOneLevel")}
          >
            <ArrowUp className="h-4 w-4" />
            <span className="hidden sm:inline">{t("gallery.folders.up")}</span>
          </button>
        )}
      </div>

      {/* Error state */}
      {imagesError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {imagesError}
        </div>
      )}

      {/* Loading state */}
      {isLoading && currentPath !== null && (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      )}

      {/* Root level: show gallery root folders */}
      {currentPath === null && !isLoading && (
        <RootFoldersGrid
          folders={rootFolders}
          onEnterFolder={handleEnterFolder}
          emptyText={t("gallery.folders.empty")}
          emptyHint={t("gallery.emptyHint")}
        />
      )}

      {/* Inside folder: show subfolders + images */}
      {currentPath !== null && !isLoading && subdirs.length === 0 && images.length === 0 && (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <FolderOpen className="mx-auto h-10 w-10 text-muted-foreground/50" />
          <p className="mt-2 text-sm font-medium text-muted-foreground">
            {t("gallery.folders.emptyFolder")}
          </p>
        </div>
      )}

      {currentPath !== null && !isLoading && (subdirs.length > 0 || images.length > 0) && (
        <div className="space-y-6">
          {/* Subdirectories section */}
          {subdirs.length > 0 && (
            <div>
              <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">
                {t("gallery.folders.subfolders")}
              </h3>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
                {subdirs.map((subdir) => (
                  <FolderTile
                    key={subdir.path}
                    name={subdir.name}
                    path={subdir.path}
                    onEnter={handleEnterFolder}
                  />
                ))}
              </div>
            </div>
          )}

          {/* Images section */}
          {images.length > 0 && (
            <div>
              <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">
                {t("gallery.folders.imagesCount", { count: totalImages })}
              </h3>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
                {images.map((image) => (
                  <ImageTile
                    key={image.id}
                    image={image}
                    onClick={onImageClick}
                  />
                ))}
              </div>

              {/* Sentinel for automatic infinite scroll */}
              <div ref={sentinelRef} className="h-4" />
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ─── Sub-components ────────────────────────────────────────────────

interface RootFoldersGridProps {
  folders: { id: number; path: string; fileCount: number; createdAt: string }[]
  onEnterFolder: (path: string) => void
  emptyText: string
  emptyHint: string
}

function RootFoldersGrid({ folders, onEnterFolder, emptyText, emptyHint }: RootFoldersGridProps) {
  const { t } = useTranslation()

  if (folders.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-12 text-center">
        <Folder className="mx-auto h-10 w-10 text-muted-foreground/50" />
        <p className="mt-2 text-sm font-medium text-muted-foreground">{emptyText}</p>
        <p className="text-xs text-muted-foreground/70">{emptyHint}</p>
      </div>
    )
  }

  return (
    <div>
      <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">
        {t("gallery.folders.rootFolders")}
      </h3>
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
        {folders.map((folder) => (
          <FolderTile
            key={folder.id}
            name={folder.path.split("/").filter(Boolean).pop() || folder.path}
            path={folder.path}
            fileCount={folder.fileCount}
            onEnter={onEnterFolder}
          />
        ))}
      </div>
    </div>
  )
}

interface FolderTileProps {
  name: string
  path: string
  fileCount?: number
  onEnter: (path: string) => void
}

function FolderTile({ name, path, fileCount, onEnter }: FolderTileProps) {
  const { t } = useTranslation()

  return (
    <button
      type="button"
      onDoubleClick={() => onEnter(path)}
      className="flex flex-col items-center gap-2 rounded-lg border border-border bg-card p-4 text-center transition-colors hover:bg-accent hover:border-primary/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      title={path}
    >
      <Folder className="h-10 w-10 text-amber-500 flex-shrink-0" />
      <span className="text-xs font-medium text-foreground line-clamp-2 break-all leading-tight">
        {name}
      </span>
      {fileCount !== undefined && (
        <span className="text-[10px] text-muted-foreground">
          {t("gallery.folderImageCount", { count: fileCount.toString() })}
        </span>
      )}
    </button>
  )
}

interface ImageTileProps {
  image: GalleryImageDTO
  onClick: (image: GalleryImageDTO) => void
}

function ImageTile({ image, onClick }: ImageTileProps) {
  const handleClick = () => onClick(image)

  return (
    <div
      className="group relative flex flex-col rounded-lg border border-border bg-card overflow-hidden transition-colors hover:border-primary/30 cursor-pointer"
      onClick={handleClick}
      onDoubleClick={handleClick}
    >
      {/* Thumbnail */}
      <div className="aspect-square bg-muted flex items-center justify-center overflow-hidden">
        {image.thumbnail ? (
          <img
            src={image.thumbnail}
            alt={image.fileName}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <ImageIcon className="h-8 w-8 text-muted-foreground/50" />
        )}
      </div>

      {/* File name */}
      <div className="p-1.5">
        <p className="text-[11px] text-foreground line-clamp-2 break-all leading-tight" title={image.fileName}>
          {image.fileName}
        </p>
      </div>
    </div>
  )
}
