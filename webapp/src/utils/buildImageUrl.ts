export function buildImageUrl(imagePath: string, apiEndpoint: string, params?: Record<string, string | number>) {
  const baseUrl = import.meta.env.VITE_API_URL || ""
  const searchParams = new URLSearchParams()
  searchParams.set("path", imagePath)
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      searchParams.set(key, String(value))
    })
  }
  return `${baseUrl}${apiEndpoint}?${searchParams.toString()}`
}
