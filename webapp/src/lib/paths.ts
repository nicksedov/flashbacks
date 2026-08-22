/**
 * Shared filesystem path helpers for gallery views.
 * Centralizes normalization and relative-name logic that was previously
 * duplicated across GalleryImageGrid, ExifImageGrid, and GalleryFoldersView.
 */

/** Normalize a path: convert backslashes to forward slashes and strip trailing slashes. */
export function normalizePath(p: string): string {
  return p.replace(/\\/g, "/").replace(/\/+$/, "")
}

/** Return the last path segment (the directory or file name). */
export function getBaseName(path: string): string {
  const normalized = normalizePath(path)
  const lastSlash = normalized.lastIndexOf("/")
  return lastSlash >= 0 ? normalized.substring(lastSlash + 1) : normalized
}

/**
 * Compute the relative folder display name for an image directory.
 * If a matching root folder is found, the root name is prefixed for subfolders.
 * Falls back to the last path segment.
 */
export function getRelativeFolderName(dirPath: string, rootFolders?: { path: string }[]): string {
  const normalized = normalizePath(dirPath)

  if (rootFolders?.length) {
    // Find the matching root folder (longest match first)
    const sorted = [...rootFolders].sort((a, b) => b.path.length - a.path.length)
    for (const root of sorted) {
      const rootNorm = normalizePath(root.path)
      if (normalized === rootNorm) {
        // Image is directly in the root folder — show root folder name
        return getBaseName(rootNorm)
      }
      if (normalized.startsWith(rootNorm + "/")) {
        // Image is in a subfolder — show relative path including root name
        const rootName = getBaseName(rootNorm)
        const relative = normalized.substring(rootNorm.length + 1)
        return rootName + "/" + relative
      }
    }
  }

  // Fallback: return last segment
  return getBaseName(normalized)
}

/**
 * Compute the longest common path prefix from a list of absolute paths.
 * Returns empty string if fewer than 2 paths or no common prefix beyond "/".
 * Example: ["/storage/gallery/photo", "/storage/gallery/camera"] → "/storage/gallery"
 */
export function getCommonPathPrefix(paths: string[]): string {
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
export function relativePath(path: string, basePath: string): string {
  if (!basePath) return path
  if (!path.startsWith(basePath)) return path
  const rel = path.slice(basePath.length)
  return rel.startsWith("/") ? rel.slice(1) : rel
}
