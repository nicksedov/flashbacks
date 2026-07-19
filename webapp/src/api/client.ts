export const API_BASE_URL = import.meta.env.VITE_API_URL || ""

function handleUnauthorized(): never {
  window.dispatchEvent(new CustomEvent("navigate-to-login"))
  throw new Error("Authorization required")
}

// Function to handle API error messages
// Backend now sends human-readable messages resolved from i18n keys
export function translateApiMessage(message: string | undefined): string {
  if (!message) return "Unknown error"
  // Backend sends translated messages directly, so we just return them
  return message
}

/**
 * Translates raw fetch/network errors into user-friendly messages.
 * Browser fetch throws TypeError with messages like "Failed to fetch",
 * "NetworkError", "Network request failed" when the server is unreachable.
 * These raw messages are meaningless to end users.
 */
export function translateNetworkError(err: unknown): string {
  if (!(err instanceof TypeError)) {
    return err instanceof Error ? err.message : String(err)
  }
  const msg = err.message.toLowerCase()
  if (msg.includes("failed to fetch") || msg.includes("networkerror") || msg.includes("network request failed")) {
    return "Server is unreachable. Please check that the backend service is running and your network connection is stable."
  }
  if (msg.includes("timeout") || msg.includes("abort")) {
    return "Request timed out. The server may be overloaded or the operation is taking too long."
  }
  if (msg.includes("load failed")) {
    return "Connection failed. The server may be down or the address is incorrect."
  }
  return err.message
}

interface ParsedResponse {
  error?: string
  message?: string
  [key: string]: unknown
}

async function safeParseJson(response: Response): Promise<ParsedResponse> {
  const text = await response.text()
  try {
    return JSON.parse(text) as ParsedResponse
  } catch {
    throw new Error(text || `HTTP ${response.status}`)
  }
}

async function handleResponse<T>(response: Response): Promise<T> {
  const data = await safeParseJson(response)

  if (!response.ok) {
    if (response.status === 401) {
      handleUnauthorized()
    }
    const errorMessage = translateApiMessage(data.error || data.message)
    throw new Error(errorMessage)
  }

  return data as T
}

export async function apiGet<T>(path: string, params?: Record<string, string>, signal?: AbortSignal): Promise<T> {
  const url = new URL(`${API_BASE_URL}${path}`, window.location.origin)
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      url.searchParams.set(key, value)
    })
  }

  try {
    const response = await fetch(url.toString(), {
      credentials: "include",
      signal,
    })

    return handleResponse<T>(response)
  } catch (err) {
    throw new Error(translateNetworkError(err))
  }
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: body ? JSON.stringify(body) : undefined,
    })

    return handleResponse<T>(response)
  } catch (err) {
    throw new Error(translateNetworkError(err))
  }
}

export async function apiDelete<T>(path: string): Promise<T> {
  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: "DELETE",
      credentials: "include",
    })

    return handleResponse<T>(response)
  } catch (err) {
    throw new Error(translateNetworkError(err))
  }
}

export async function apiPut<T>(path: string, body?: unknown): Promise<T> {
  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: body ? JSON.stringify(body) : undefined,
    })

    return handleResponse<T>(response)
  } catch (err) {
    throw new Error(translateNetworkError(err))
  }
}

export async function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: body ? JSON.stringify(body) : undefined,
    })

    return handleResponse<T>(response)
  } catch (err) {
    throw new Error(translateNetworkError(err))
  }
}

export async function apiUpload<T>(path: string, formData: FormData): Promise<T> {
  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      credentials: "include",
      body: formData,
    })

    return handleResponse<T>(response)
  } catch (err) {
    throw new Error(translateNetworkError(err))
  }
}
