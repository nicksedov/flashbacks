import { API_BASE_URL } from "@/api/client"

/**
 * Triggers a browser download for an image by path.
 * Shared between Gallery and Smart Search tabs.
 */
export function downloadImage(path: string, fileName: string): void {
  const imageUrl = `${API_BASE_URL}/api/image?path=${encodeURIComponent(path)}`
  const a = document.createElement("a")
  a.href = imageUrl
  a.download = fileName
  a.click()
}
