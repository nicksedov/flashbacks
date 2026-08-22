/**
 * Shared client-side file download helpers.
 * Centralizes the anchor/Blob download logic that was duplicated across
 * downloadImage, useFileExport, and inline handlers.
 */

/** Trigger a browser download for a URL with the given filename. */
export function downloadUrl(url: string, fileName: string): void {
  const a = document.createElement("a")
  a.href = url
  a.download = fileName
  a.click()
}

/** Trigger a browser download for a Blob with the given filename. */
export function downloadBlob(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob)
  downloadUrl(url, fileName)
  URL.revokeObjectURL(url)
}

/**
 * Download an image by path via the backend image endpoint.
 * Shared between Gallery, EXIF, and Smart Search tabs.
 */
export function downloadImage(path: string, fileName: string): void {
  const imageUrl = `${import.meta.env.VITE_API_URL || ""}/api/image?path=${encodeURIComponent(path)}`
  downloadUrl(imageUrl, fileName)
}
