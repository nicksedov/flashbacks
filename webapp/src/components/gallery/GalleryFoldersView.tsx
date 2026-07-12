import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Folder,
  FolderOpen,
  ArrowUp,
  ChevronRight,
  Home,
} from "lucide-react"
import { useGalleryFolders } from "@/hooks/useGalleryFolders"
import { useGalleryImages } from "@/hooks/useGalleryImages"
import { useIntersectionObserver } from "@/hooks/useIntersectionObserver"
import { fetchSubdirs } from "@/api/endpoints"
import { useTranslation } from "@/i18n"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import { ImageTile } from "./ImageTile"
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
 * Compute the longest common path prefix from a list of absolute paths.
 * Returns empty string if fewer than 2 paths or no common prefix beyond "/".
 * Example: ["/storage/gallery/photo", "/storage/gallery/camera"] → "/storage/gallery"
 */
function getCommonPathPrefix(paths: string[]): string {
  if (paths.length < 2) return ""

  const parts = paths.map((p) => p.split("/").filter(Boolean))
  const minLen = Math.min(...parts.map((p) => p.length))

  if (minLen === 0) return ""

  let commonCount = 0
  for (let i = 0; i < minLen; i++) {
    const segment = parts[0][i]
    if (parts.every((p) => p[i] === segment)) {
      commonCount++
    } else {
      break
    }
  }

  if (commonCount === 0) return ""
  return "/" + parts[0].slice(0, commonCount).join("/")
}

/**
 * Compute the relative path by stripping the base prefix.
 * Returns the path unchanged if basePath is empty or path doesn't start with it.
 */
function relativePath(path: string, basePath: string): string {
  if (!basePath) return path
  if (!path.startsWith(basePath)) return path
  const rel = path.slice(basePath.length)
  return rel.startsWith("/") ? rel.slice(1) : rel
}

/**
 * File-manager style folder browser for the gallery.
 * Shows root gallery folders initially, then allows drilling into subdirectories.
 * When root folders share a common path prefix, it becomes the virtual base —
 * intermediate directories are hidden from breadcrumbs and root folder names.
 * Displays folders first, then image thumbnails.
 */
export function GalleryFoldersView(props: GalleryFoldersViewProps) {
  const { onImageClick, onImageDownload, onImageDelete } = props
  const { t } = useTranslation()
  const { folders: rootFolders } = useGalleryFolders()

  // Compute common base path from root gallery folders
  const basePath = useMemo(
    () => getCommonPathPrefix(rootFolders.map((f) => f.path)),
    [rootFolders]
  )

  // Current directory path: null = root (gallery folders / virtual base)
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
    removeImage,
  } = useGalleryImages("folders", "newest", undefined, currentPath ?? undefined)

  // Clear subdirs when going back to root
  useEffect(() => {
    if (currentPath !== null) return
    /* eslint-disable react-hooks/set-state-in-effect */
    setSubdirs([])
    setSubdirsLoading(false)
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [currentPath])

  // Load subdirectories when currentPath changes to a non-null value
  useEffect(() => {
    if (currentPath === null) return
    let cancelled = false
    /* eslint-disable react-hooks/set-state-in-effect */
    setSubdirsLoading(true)
    /* eslint-enable react-hooks/set-state-in-effect */
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

  // Compute breadcrumbs relative to basePath
  const breadcrumbs: BreadcrumbSegment[] = []
  if (currentPath !== null && basePath) {
    // When basePath exists, build breadcrumbs only from the relative portion
    const rel = relativePath(currentPath, basePath)
    if (rel) {
      const parts = rel.split("/").filter(Boolean)
      let acc = basePath
      for (const part of parts) {
        acc += "/" + part
        breadcrumbs.push({ name: part, path: acc })
      }
    }
  } else if (currentPath !== null) {
    // No common base path — use full path for breadcrumbs (original behavior)
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
    // When basePath exists, go back to root if we are at basePath + 1 segment
    if (basePath) {
      const baseParts = basePath.split("/").filter(Boolean)
      if (parts.length <= baseParts.length + 1) {
        setCurrentPath(null)
        return
      }
    } else if (parts.length <= 1) {
      setCurrentPath(null)
      return
    }
    parts.pop()
    setCurrentPath("/" + parts.join("/"))
  }, [currentPath, basePath])

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

  // Home button tooltip: show base path when applicable
  const homeTitle = basePath
    ? basePath
    : t("gallery.folders.root")

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
          title={homeTitle}
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
          basePath={basePath}
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
              <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
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
              <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
                {images.map((image) => (
                  <ImageTile
                    key={image.id}
                    image={image}
                    onClick={onImageClick}
                    onImageDownload={onImageDownload}
                    onImageDelete={onImageDelete ? () => onImageDelete(image, () => removeImage(image.id)) : undefined}
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
  basePath: string
  onEnterFolder: (path: string) => void
  emptyText: string
  emptyHint: string
}

function RootFoldersGrid({ folders, basePath, onEnterFolder, emptyText, emptyHint }: RootFoldersGridProps) {
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
      <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
        {folders.map((folder) => {
          const displayName = basePath
            ? relativePath(folder.path, basePath) || folder.path.split("/").filter(Boolean).pop() || folder.path
            : folder.path.split("/").filter(Boolean).pop() || folder.path
          return (
            <FolderTile
              key={folder.id}
              name={displayName}
              path={folder.path}
              fileCount={folder.fileCount}
              onEnter={onEnterFolder}
            />
          )
        })}
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
  return (
    <div
      role="button"
      tabIndex={0}
      className="group flex flex-col cursor-pointer"
      onClick={() => onEnter(path)}
      onDoubleClick={() => onEnter(path)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onEnter(path); } }}
      title={path}
    >
      <div className="relative aspect-square overflow-hidden rounded-lg border bg-card transition-all hover:ring-2 hover:ring-ring flex items-center justify-center">
        <Folder className="h-12 w-12 text-amber-500" />
        {fileCount !== undefined && (
          <span className="absolute top-1.5 right-1.5 text-[10px] font-medium text-muted-foreground bg-background/80 rounded px-1.5 py-0.5">
            {fileCount}
          </span>
        )}
      </div>
      <p className="text-[11px] text-muted-foreground truncate mt-1 px-0.5 w-full text-center" title={name}>
        {name}
      </p>
    </div>
  )
}

