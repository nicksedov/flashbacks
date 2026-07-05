import { apiGet, apiPost, apiDelete, apiPut, apiPatch, apiUpload } from "./client"
import type {
  DuplicatesResponse,
  ScanResponse,
  FastScanResponse,
  ScanStatusResponse,
  ThumbnailResponse,
  DeleteFilesRequest,
  DeleteFilesResponse,
  FolderPatternsResponse,
  BatchDeleteRequest,
  BatchDeleteResponse,
  MoveFilesRequest,
  MoveFilesResponse,
  GalleryFoldersResponse,
  AddFolderRequest,
  AddFolderResponse,
  RemoveFolderResponse,
  GalleryImagesResponse,
  GalleryCalendarResponse,
  CalendarAllDatesResponse,
  CalendarSeekResponse,
  GeoClustersResponse,
  GeoClusterRequest,
  GeoImagesResponse,
  AppSettingsDTO,
  UserSettingsDTO,
  UpdateSettingsRequest,
  UpdateUserSettingsRequest,
  TrashInfoResponse,
  CleanTrashResponse,
  TrashFileDTO,
  RestoreTrashFileRequest,
  DeleteTrashFileRequest,
  ImageMetadataResponse,
  AuthStatusResponse,
  LoginRequest,
  LoginResponse,
  ChangePasswordRequest,
  ChangePasswordResponse,
  BootstrapSetupRequest,
  UpdateProfileRequest,
  CreateUserRequest,
  UpdateUserRequest,
  ResetPasswordRequest,
  UsersListResponse,
  AuditLogsResponse,
  OCRStatusResponse,
  OcrDocumentsResponse,
  OcrDataResponse,
  OcrClassificationStatusResponse,
  LlmSettingsResponse,
  LlmProviderDTO,
  UpdateLlmSettingsRequest,
  LlmOcrRequest,
  LlmRecognizeStatusResponse,
  LlmOcrDataResponse,
  LlmModelsResponse,
  ThumbnailCacheStatsResponse,
  WarmupThumbnailsRequest,
  TagScanStatusResponse,
  GeocodeSearchResponse,
  UpdateGpsRequest,
  UpdateGpsResponse,
  LocationCandidatesResponse,
  BatchUpdateGpsRequest,
  BatchUpdateGpsResponse,
  Conversation,
  ChatMessage,
  CreateConversationRequest,
  SSEEvent,
  TagSearchResponse,
  SmartSearchResponse,
  EmbeddingBackfillStatus,
  SyncStatusResponse,
  ExifServiceStatus,
  CreateFolderRequest,
  CreateFolderResponse,
  SubdirsResponse,
} from "@/types"

export function fetchDuplicates(page: number, pageSize: number): Promise<DuplicatesResponse> {
  return apiGet<DuplicatesResponse>("/api/duplicates", {
    page: String(page),
    pageSize: String(pageSize),
  })
}

export function triggerScan(): Promise<ScanResponse> {
  return apiPost<ScanResponse>("/api/scan")
}

export function triggerFastScan(): Promise<FastScanResponse> {
  return apiPost<FastScanResponse>("/api/fast-scan")
}

export function fetchScanStatus(): Promise<ScanStatusResponse> {
  return apiGet<ScanStatusResponse>("/api/status")
}

export function fetchThumbnail(path: string): Promise<ThumbnailResponse> {
  return apiGet<ThumbnailResponse>("/api/thumbnail", { path })
}

export function deleteFiles(req: DeleteFilesRequest): Promise<DeleteFilesResponse> {
  return apiPost<DeleteFilesResponse>("/api/delete-files", req)
}

export function fetchFolderPatterns(): Promise<FolderPatternsResponse> {
  return apiGet<FolderPatternsResponse>("/api/folder-patterns")
}

export function batchDelete(req: BatchDeleteRequest): Promise<BatchDeleteResponse> {
  return apiPost<BatchDeleteResponse>("/api/batch-delete", req)
}

export function moveFiles(req: MoveFilesRequest): Promise<MoveFilesResponse> {
  return apiPost<MoveFilesResponse>("/api/move-files", req)
}

export function createFolder(req: CreateFolderRequest): Promise<CreateFolderResponse> {
  return apiPost<CreateFolderResponse>("/api/folders/mkdir", req)
}

export function fetchSubdirs(path: string, signal?: AbortSignal): Promise<SubdirsResponse> {
  return apiGet<SubdirsResponse>("/api/folders/subdirs", { path }, signal)
}

// --- Gallery Folders ---

export function fetchFolders(): Promise<GalleryFoldersResponse> {
  return apiGet<GalleryFoldersResponse>("/api/folders")
}

export function addFolder(req: AddFolderRequest): Promise<AddFolderResponse> {
  return apiPost<AddFolderResponse>("/api/folders", req)
}

export function removeFolder(id: number): Promise<RemoveFolderResponse> {
  return apiDelete<RemoveFolderResponse>(`/api/folders/${id}`)
}

// --- Gallery Images ---

export function fetchGalleryImages(
  page: number,
  pageSize: number,
  view: string,
  sortOrder: string = "newest",
  search?: string
): Promise<GalleryImagesResponse> {
  const params: Record<string, string> = {
    page: String(page),
    pageSize: String(pageSize),
    view,
    sortOrder,
  }
  if (search) {
    params.search = search
  }
  return apiGet<GalleryImagesResponse>("/api/gallery", params)
}

// --- Gallery Calendar ---

export function fetchGalleryCalendar(
  page: number,
  pageSize: number,
  startDate?: string,
  endDate?: string,
  monthYear?: string,
  sortOrder?: string,
  cursor?: string  // Cursor-based pagination support
): Promise<GalleryCalendarResponse> {
  const params: Record<string, string> = {
    pageSize: String(pageSize),
  }
  
  // Use cursor if provided (overrides page)
  if (cursor) {
    params.cursor = cursor
  } else {
    params.page = String(page)
  }
  
  if (startDate) params.startDate = startDate
  if (endDate) params.endDate = endDate
  if (monthYear) params.monthYear = monthYear
  if (sortOrder) params.sortOrder = sortOrder
  return apiGet<GalleryCalendarResponse>("/api/gallery/calendar", params)
}

export function fetchCalendarSeek(date: string): Promise<CalendarSeekResponse> {
  return apiGet<CalendarSeekResponse>("/api/gallery/calendar/seek", { date })
}

// --- Gallery Calendar Month Info (lightweight) ---

export interface CalendarMonthDayCount {
  day: number
  count: number
}

export interface CalendarMonthData {
  year: number
  month: number
  days: number[]
  dayCounts: CalendarMonthDayCount[]
  total: number
}

export function fetchCalendarMonthInfo(monthYear: string): Promise<CalendarMonthData> {
  return apiGet<CalendarMonthData>("/api/gallery/calendar/month", { monthYear })
}

export function fetchCalendarAllDates(): Promise<CalendarAllDatesResponse> {
  return apiGet<CalendarAllDatesResponse>("/api/gallery/calendar/dates")
}

// --- Gallery Geolocation ---

export interface GeoBounds {
  minLat: number
  maxLat: number
  minLng: number
  maxLng: number
}

export interface GeoImagesRequest {
  clusterId?: string
  bounds?: GeoBounds
}

export function fetchGalleryClusters(
  params: GeoClusterRequest,
  signal?: AbortSignal
): Promise<GeoClustersResponse> {
  return apiGet<GeoClustersResponse>("/api/gallery/clusters", {
    minLat: String(params.minLat),
    maxLat: String(params.maxLat),
    minLng: String(params.minLng),
    maxLng: String(params.maxLng),
    zoom: String(params.zoom),
    width: String(params.width),
    height: String(params.height),
  }, signal)
}

export function fetchGeoImages(
  page: number,
  pageSize: number,
  request: GeoImagesRequest
): Promise<GeoImagesResponse> {
  const params: Record<string, string> = {
    page: String(page),
    pageSize: String(pageSize),
  }

  if (request.clusterId) {
    params.clusterId = request.clusterId
  } else if (request.bounds) {
    params.minLat = String(request.bounds.minLat)
    params.maxLat = String(request.bounds.maxLat)
    params.minLng = String(request.bounds.minLng)
    params.maxLng = String(request.bounds.maxLng)
  }

  return apiGet<GeoImagesResponse>("/api/gallery/geo-images", params)
}

// --- App Settings ---

export function fetchSettings(): Promise<AppSettingsDTO> {
  return apiGet<AppSettingsDTO>("/api/settings")
}

export function updateSettings(req: UpdateSettingsRequest): Promise<AppSettingsDTO> {
  return apiPut<AppSettingsDTO>("/api/settings", req)
}

export function fetchSyncStatus(): Promise<SyncStatusResponse> {
  return apiGet<SyncStatusResponse>("/api/sync-status")
}

// --- User Settings ---

export function fetchUserSettings(): Promise<UserSettingsDTO> {
  return apiGet<UserSettingsDTO>("/api/user-settings")
}

export function updateUserSettings(req: UpdateUserSettingsRequest): Promise<UserSettingsDTO> {
  return apiPut<UserSettingsDTO>("/api/user-settings", req)
}

// --- Trash ---

export function fetchTrashInfo(): Promise<TrashInfoResponse> {
  return apiGet<TrashInfoResponse>("/api/trash-info")
}

export function cleanTrash(): Promise<CleanTrashResponse> {
  return apiPost<CleanTrashResponse>("/api/trash-clean")
}

export function fetchTrashList(): Promise<TrashFileDTO[]> {
  return apiGet<TrashFileDTO[]>("/api/trash-list")
}

export function restoreTrashFile(req: RestoreTrashFileRequest): Promise<{ success: boolean; restoredPath: string }> {
  return apiPost<{ success: boolean; restoredPath: string }>("/api/trash-restore", req)
}

export function deleteTrashFile(req: DeleteTrashFileRequest): Promise<{ success: boolean }> {
  return apiPost<{ success: boolean }>("/api/trash-delete", req)
}

// --- Image Metadata ---

export function fetchImageMetadata(path: string): Promise<ImageMetadataResponse> {
  return apiGet<ImageMetadataResponse>("/api/image-metadata", { path })
}

// --- EXIF Tool ---

export function fetchImagesMissingExif(
  page: number,
  pageSize: number
): Promise<GalleryImagesResponse> {
  const params: Record<string, string> = {
    page: String(page),
    pageSize: String(pageSize),
  }
  return apiGet<GalleryImagesResponse>("/api/gallery/exif-images", params)
}

// --- Auth ---

export function fetchAuthStatus(): Promise<AuthStatusResponse> {
  return apiGet<AuthStatusResponse>("/api/auth/status")
}

export function login(req: LoginRequest): Promise<LoginResponse> {
  return apiPost<LoginResponse>("/api/auth/login", req)
}

export function logout(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/auth/logout")
}

export function fetchCurrentUser(): Promise<{ user: import("@/types").UserDTO }> {
  return apiGet<{ user: import("@/types").UserDTO }>("/api/auth/me")
}

export function changePassword(req: ChangePasswordRequest): Promise<ChangePasswordResponse> {
  return apiPost<ChangePasswordResponse>("/api/auth/change-password", req)
}

export function bootstrapSetup(req: BootstrapSetupRequest): Promise<{ user: import("@/types").UserDTO; message: string }> {
  return apiPost<{ user: import("@/types").UserDTO; message: string }>("/api/auth/bootstrap/setup", req)
}

export function updateProfile(req: UpdateProfileRequest): Promise<{ user: import("@/types").UserDTO }> {
  return apiPatch<{ user: import("@/types").UserDTO }>("/api/users/me", req)
}

// --- Avatar ---

export function uploadAvatar(file: Blob): Promise<{ user: import("@/types").UserDTO }> {
  const formData = new FormData()
  formData.append("avatar", file)
  return apiUpload<{ user: import("@/types").UserDTO }>("/api/users/me/avatar", formData)
}

export function deleteAvatar(): Promise<{ user: import("@/types").UserDTO }> {
  return apiDelete<{ user: import("@/types").UserDTO }>("/api/users/me/avatar")
}

export function getAvatarUrl(userId: number): string {
  const baseUrl = import.meta.env.VITE_API_URL || ""
  return `${baseUrl}/api/users/${userId}/avatar`
}

// --- Admin ---

export function fetchUsers(): Promise<UsersListResponse> {
  return apiGet<UsersListResponse>("/api/admin/users")
}

export function createUser(req: CreateUserRequest): Promise<{ user: import("@/types").UserDTO; message: string }> {
  return apiPost<{ user: import("@/types").UserDTO; message: string }>("/api/admin/users", req)
}

export function updateUser(id: number, req: UpdateUserRequest): Promise<{ user: import("@/types").UserDTO; message: string }> {
  return apiPatch<{ user: import("@/types").UserDTO; message: string }>(`/api/admin/users/${id}`, req)
}

export function deleteUser(id: number): Promise<{ message: string }> {
  return apiDelete<{ message: string }>(`/api/admin/users/${id}`)
}

export function resetUserPassword(id: number, req: ResetPasswordRequest): Promise<{ message: string }> {
  return apiPost<{ message: string }>(`/api/admin/users/${id}/reset-password`, req)
}

export function fetchAuditLogs(page: number): Promise<AuditLogsResponse> {
  return apiGet<AuditLogsResponse>("/api/admin/audit", { page: String(page) })
}

// --- OCR Status ---

export function fetchOCRStatus(): Promise<OCRStatusResponse> {
  return apiGet<OCRStatusResponse>("/api/ocr-status")
}

// --- EXIF Service Status ---

export function fetchExifServiceStatus(): Promise<ExifServiceStatus> {
  return apiGet<ExifServiceStatus>("/api/exif-status")
}

// --- OCR Classification ---

export function startOcrClassification(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/ocr/classify")
}

export function startOcrClassificationChanges(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/ocr/classify-changes")
}

export function stopOcrClassification(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/ocr/stop")
}

export function fetchOcrClassificationStatus(): Promise<OcrClassificationStatusResponse> {
  return apiGet<OcrClassificationStatusResponse>("/api/ocr/classify-status")
}

export function fetchOcrDocuments(
  page: number,
  pageSize: number
): Promise<OcrDocumentsResponse> {
  return apiGet<OcrDocumentsResponse>("/api/ocr/documents", {
    page: String(page),
    pageSize: String(pageSize),
  })
}

export function fetchOcrData(path: string): Promise<OcrDataResponse> {
  return apiGet<OcrDataResponse>("/api/ocr/data", { path })
}

// --- LLM Settings (global: active provider + tag scan) ---

export function fetchLlmSettings(): Promise<LlmSettingsResponse> {
  return apiGet<LlmSettingsResponse>("/api/llm/settings")
}

export function updateLlmSettings(req: UpdateLlmSettingsRequest): Promise<{ message: string }> {
  return apiPut<{ message: string }>("/api/llm/settings", req)
}

// --- LLM Provider CRUD ---

export function createLlmProvider(req: {
  alias: string
  name: string
  apiUrl?: string
  apiKey?: string
  model?: string
}): Promise<LlmProviderDTO> {
  return apiPost<LlmProviderDTO>("/api/llm/providers", req)
}

export function updateLlmProvider(alias: string, req: {
  apiUrl?: string
  apiKey?: string
  model?: string
  alias?: string
}): Promise<{ message: string }> {
  return apiPut<{ message: string }>(`/api/llm/providers/${encodeURIComponent(alias)}`, req)
}

export function deleteLlmProvider(alias: string): Promise<{ message: string }> {
  return apiDelete<{ message: string }>(`/api/llm/providers/${encodeURIComponent(alias)}`)
}

// --- LLM OCR ---

export function recognizeWithLlm(req: LlmOcrRequest): Promise<LlmRecognizeStatusResponse> {
  return apiPost<LlmRecognizeStatusResponse>("/api/llm/recognize", req)
}

export function fetchLlmRecognizeStatus(path: string): Promise<LlmRecognizeStatusResponse> {
  return apiGet<LlmRecognizeStatusResponse>("/api/llm/recognize-status", { path })
}

export function fetchLlmRecognition(path: string): Promise<LlmOcrDataResponse> {
  return apiGet<LlmOcrDataResponse>("/api/llm/recognition", { path })
}

export function fetchLlmModels(provider?: string, force?: boolean): Promise<LlmModelsResponse> {
  const params: Record<string, string> = {}
  if (provider) {
    params.provider = provider
  }
  if (force) {
    params.force = "true"
  }
  return apiGet<LlmModelsResponse>("/api/llm/models", params)
}

export function probeEmbeddingDimension(providerAlias: string, model: string): Promise<{ dimension: number }> {
  return apiPost<{ dimension: number }>("/api/llm/embedding/probe", { providerAlias, model })
}

// --- Tag Scan ---

export function fetchTagScanStatus(): Promise<TagScanStatusResponse> {
  return apiGet<TagScanStatusResponse>("/api/tag-scan/status")
}

export function pauseTagScan(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/tag-scan/pause", {})
}

export function resumeTagScan(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/tag-scan/resume", {})
}

// --- Thumbnail Cache Management ---

export function fetchThumbnailCacheStats(): Promise<ThumbnailCacheStatsResponse> {
  return apiGet<ThumbnailCacheStatsResponse>("/api/thumbnail/cache/stats")
}

export function invalidateAllThumbnails(): Promise<{ message: string }> {
  return apiDelete<{ message: string }>("/api/thumbnail/cache/invalidate-all")
}

export function warmupThumbnails(req: WarmupThumbnailsRequest): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/thumbnail/cache/warmup", req)
}

export function enableThumbnailCache(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/thumbnail/cache/enable")
}

export function disableThumbnailCache(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/thumbnail/cache/disable")
}

// --- AI Assistant ---

export function startAiAction(req: import("@/types").AiActionRequest): Promise<import("@/types").AiActionStartResponse> {
  return apiPost<import("@/types").AiActionStartResponse>("/api/ai/action", req)
}

export function fetchAiActionStatus(taskId: string): Promise<import("@/types").AiActionStatusResponse> {
  return apiGet<import("@/types").AiActionStatusResponse>(`/api/ai/status/${taskId}`)
}

// --- Image Tags ---

export function fetchImageTags(path: string): Promise<import("@/types").ImageTagsResponse> {
  return apiGet<import("@/types").ImageTagsResponse>("/api/image-tags", { path })
}

// --- Geocode / GPS ---

export function searchGeocodeLocations(query: string, signal?: AbortSignal): Promise<GeocodeSearchResponse> {
  return apiGet<GeocodeSearchResponse>("/api/geocode/search", { q: query }, signal)
}

export function updateImageGps(req: UpdateGpsRequest): Promise<UpdateGpsResponse> {
  return apiPut<UpdateGpsResponse>("/api/image-metadata/gps", req)
}

export function fetchLocationCandidatesByDate(date: string): Promise<LocationCandidatesResponse> {
  return apiGet<LocationCandidatesResponse>("/api/image-metadata/location-candidates", { date })
}

export function batchUpdateGps(req: BatchUpdateGpsRequest): Promise<BatchUpdateGpsResponse> {
  return apiPut<BatchUpdateGpsResponse>("/api/image-metadata/gps/batch", req)
}

// --- Chat / Agent ---

export function createConversation(req: CreateConversationRequest): Promise<Conversation> {
  return apiPost<Conversation>("/api/chat/conversations", req)
}

export function fetchConversations(imagePath?: string): Promise<Conversation[]> {
  const params: Record<string, string> = {}
  if (imagePath) {
    params.imagePath = imagePath
  }
  return apiGet<Conversation[]>("/api/chat/conversations", params)
}

export function deleteConversation(id: number): Promise<{ message: string }> {
  return apiDelete<{ message: string }>(`/api/chat/conversations/${id}`)
}

export function fetchConversationMessages(convId: number): Promise<ChatMessage[]> {
  return apiGet<ChatMessage[]>(`/api/chat/conversations/${convId}/messages`)
}

/**
 * Send a message to a conversation with SSE streaming.
 * Uses fetch + ReadableStream since EventSource only supports GET.
 */
export function sendMessageStream(
  convId: number,
  content: string,
  onEvent: (event: SSEEvent) => void,
  signal?: AbortSignal,
): void {
  const baseUrl = import.meta.env.VITE_API_URL || ""
  const url = `${baseUrl}/api/chat/conversations/${convId}/messages`

  fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Accept": "text/event-stream",
    },
    body: JSON.stringify({ content }),
    credentials: "include",
    signal,
  }).then(async (response) => {
    if (!response.ok) {
      const errBody = await response.text()
      onEvent({ type: "error", error: `HTTP ${response.status}: ${errBody}` })
      return
    }

    const reader = response.body?.getReader()
    if (!reader) {
      onEvent({ type: "error", error: "No response stream" })
      return
    }

    const decoder = new TextDecoder()
    let buffer = ""

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split("\n")
      buffer = lines.pop() || ""

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          try {
            const event: SSEEvent = JSON.parse(line.slice(6))
            onEvent(event)
          } catch {
            // Skip malformed SSE data
          }
        }
      }
    }

    // Process remaining buffer
    if (buffer.startsWith("data: ")) {
      try {
        const event: SSEEvent = JSON.parse(buffer.slice(6))
        onEvent(event)
      } catch {
        // Skip malformed SSE data
      }
    }
  }).catch((err) => {
    if (err.name !== "AbortError") {
      onEvent({ type: "error", error: err.message })
    }
  })
}

// --- Tag Search ---

export function searchByTags(tags: string[], matchAll = false): Promise<TagSearchResponse> {
  return apiGet<TagSearchResponse>("/api/gallery/tag-search", {
    tags: tags.join(","),
    matchAll: matchAll ? "true" : "false",
  })
}

// --- Smart Search ---

export function smartSearch(query: string, limit = 100, signal?: AbortSignal): Promise<SmartSearchResponse> {
  return apiGet<SmartSearchResponse>("/api/gallery/smart-search", {
    q: query,
    limit: String(limit),
  }, signal)
}

// --- Embedding Backfill ---

export function fetchEmbeddingStatus(): Promise<EmbeddingBackfillStatus> {
  return apiGet<EmbeddingBackfillStatus>("/api/embedding/status")
}

export function startEmbeddingBackfill(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/embedding/start")
}

export function stopEmbeddingBackfill(): Promise<{ message: string }> {
  return apiPost<{ message: string }>("/api/embedding/stop")
}
