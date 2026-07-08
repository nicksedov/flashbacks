export interface FileDTO {
  id: number
  path: string
  fileName: string
  dirPath: string
  modTime: string
}

export interface DuplicateGroupDTO {
  index: number
  hash: string
  size: number
  sizeHuman: string
  files: FileDTO[]
  thumbnail: string
  thumbnailCachePath?: string
}

export interface DuplicatesResponse {
  groups: DuplicateGroupDTO[]
  totalFiles: number
  pageFiles: number
  totalGroups: number
  scannedDirs: string[]
  currentPage: number
  pageSize: number
  totalPages: number
  hasPrevPage: boolean
  hasNextPage: boolean
  pageSizes: number[]
}

export interface ScanResponse {
  message: string
}

export interface FastScanResponse {
  message: string
  unchanged: number
  modified: number
  created: number
  deleted: number
  total: number
}

export interface ScanStatusResponse {
  scanning: boolean
  progress: string
  filesProcessed: number
}

export interface ThumbnailResponse {
  thumbnail: string
}

export interface DeleteFilesRequest {
  filePaths: string[]
  trashDir: string
}

export interface DeleteFilesResponse {
  success: number
  failed: number
  failedFiles?: string[]
}

export interface FolderPattern {
  id: string
  folders: string[]
  duplicateCount: number
  totalFiles: number
}

export interface FolderPatternsResponse {
  patterns: FolderPattern[]
  singleFolderDuplicateCount: number
}

export interface BatchDeleteRule {
  patternId: string
  keepFolder: string
}

export interface BatchDeleteRequest {
  rules: BatchDeleteRule[]
  trashDir: string
}

export interface BatchDeleteResponse {
  rulesApplied: number
  filesDeleted: number
  failed: number
  failedFiles?: string[]
}

export interface ApiError {
  error: string
}

// --- Gallery Folder Types ---

export interface GalleryFolderDTO {
  id: number
  path: string
  fileCount: number
  createdAt: string
}

export interface GalleryFoldersResponse {
  folders: GalleryFolderDTO[]
  totalFolders: number
}

export interface AddFolderRequest {
  path: string
}

export interface AddFolderResponse {
  message: string
  folder: GalleryFolderDTO
  scanStarted: boolean
}

export interface RemoveFolderResponse {
  message: string
  filesRemoved: number
}

// --- Gallery Image Types ---

export interface GalleryImageDTO {
  id: number
  path: string
  fileName: string
  dirPath: string
  size: number
  sizeHuman: string
  modTime: string
  thumbnail?: string
  thumbnailCachePath?: string
  missingDate?: boolean
  missingGps?: boolean
}

export interface GalleryImagesResponse {
  images: GalleryImageDTO[]
  totalImages: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

// --- App Settings Types ---

export interface AppSettingsDTO {
  trashDir: string
  exifBackupDir: string
  thumbnailCachePath?: string
  thumbnailCacheSize?: number
  ocrConcurrentRequests?: number
  syncDays?: string
  dailySyncHour?: number
  dailySyncMinute?: number
  syncTimezoneOffset?: number
  lastSyncAt?: string | null
  lastSyncNew?: number
  lastSyncUpdated?: number
  lastSyncDeleted?: number
  lastSyncThumbnails?: number
}

export interface UserSettingsDTO {
  theme: 
    | "light-purple" 
    | "dark-purple"
    | "light-green"
    | "dark-green"
    | "light-blue"
    | "dark-blue"
    | "light-orange"
    | "dark-orange"
    | "dark-contrast"
  language: "en" | "ru"
}

export interface UpdateSettingsRequest {
  trashDir?: string
  exifBackupDir?: string
  thumbnailCachePath?: string
  ocrConcurrentRequests?: number
  syncDays?: string
  dailySyncHour?: number
  dailySyncMinute?: number
  syncTimezoneOffset?: number
}

export interface SyncStatusResponse {
  running: boolean
  syncInProgress: boolean
  nextRunAt?: string | null
  lastSyncAt?: string | null
  lastSyncNew: number
  lastSyncUpdated: number
  lastSyncDeleted: number
  lastSyncThumbnails: number
  processedFiles: number
  totalFiles: number
}

export interface UpdateUserSettingsRequest {
  theme?: "light-purple" | "dark-purple" | "light-green" | "dark-green" | "light-blue" | "dark-blue" | "light-orange" | "dark-orange" | "dark-contrast"
  language?: "en" | "ru"
}

// --- Trash Types ---

export interface TrashInfoResponse {
  fileCount: number
  totalSize: number
  totalSizeHuman: string
}

export interface CleanTrashResponse {
  deleted: number
  failed: number
}

export interface TrashFileDTO {
  fileName: string
  size: number
  sizeHuman: string
  modTime: string
}

export interface RestoreTrashFileRequest {
  fileName: string
  targetPath?: string
}

export interface DeleteTrashFileRequest {
  fileName: string
}

// --- Image Metadata Types ---

export interface ImageMetadataDTO {
  width: number
  height: number
  dimensions: string
  cameraModel: string
  lensModel: string
  iso: number
  aperture: string
  shutterSpeed: string
  focalLength: string
  dateTaken: string
  orientation: number
  colorSpace: string
  software: string
  gpsLatitude: number | null
  gpsLongitude: number | null
  nameLocal: string
  nameEng: string
  hasGps: boolean
  hasExif: boolean
}

export interface ImageMetadataResponse {
  found: boolean
  metadata?: ImageMetadataDTO
}

// --- Gallery Calendar Types ---

export interface CalendarDateGroup {
  date: string       // "YYYY-MM-DD"
  label: string      // Human-readable label
  imageCount: number
  images: GalleryImageDTO[]
}

export interface CalendarDateRange {
  minDate: string    // "YYYY-MM-DD" or empty
  maxDate: string    // "YYYY-MM-DD" or empty
  totalWithDate: number
}

export interface CalendarMonthInfo {
  year: number
  month: number      // 1-12
  days: number[]     // Days that have images (1-31)
}

export interface TimelineDateMarker {
  date: string       // "YYYY-MM-DD"
  imageCount: number // Number of images on this date
  page: number       // Page number (1-based) where this date first appears (deprecated)
  cursor: string     // Cursor pointing to the start of this date
}

export interface CalendarAllDatesResponse {
  minDate: string    // "YYYY-MM-DD" or empty
  maxDate: string    // "YYYY-MM-DD" or empty
  dates: TimelineDateMarker[]
}

export interface CalendarSeekResponse {
  cursor: string     // Cursor pointing to the requested date
  actualDate: string // The actual date found (may differ if requested date has no images)
  imageCount: number // Number of images on this date
}

export interface GalleryCalendarResponse {
  groups: CalendarDateGroup[]
  totalImages: number
  totalGroups: number
  hasMore: boolean
  dateRange: CalendarDateRange
  months: CalendarMonthInfo[]
  nextCursor?: string  // Cursor-based pagination support
}

// --- Gallery Geolocation Types ---

export interface GeoClusterRequest {
  minLat: number
  maxLat: number
  minLng: number
  maxLng: number
  zoom: number
  width: number
  height: number
}

export interface GeoCluster {
  id: string
  latitude: number
  longitude: number
  count: number
}

export interface GeoClustersResponse {
  clusters: GeoCluster[]
  totalImages: number
}

export interface GeoImagesResponse {
  images: GalleryImageDTO[]
  totalImages: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

// --- Auth & User Types ---

export type UserRole = "admin" | "user"

export interface UserDTO {
  id: number
  login: string
  displayName: string
  role: UserRole
  hasAvatar: boolean
  isActive: boolean
  mustChangePassword: boolean
  createdAt: string
  lastLoginAt: string | null
}

export interface AuthStatusResponse {
  isAuthenticated: boolean
  isBootstrapMode: boolean
  user?: UserDTO
}

export interface LoginRequest {
  login: string
  password: string
}

export interface LoginResponse {
  user?: UserDTO
  isBootstrap?: boolean
  message?: string
}

export interface ChangePasswordRequest {
  oldPassword: string
  newPassword: string
}

export interface BootstrapSetupRequest {
  newPassword: string
  displayName: string
}

export interface UpdateProfileRequest {
  displayName: string
}

export interface ChangePasswordResponse {
  message: string
  mustLogin?: boolean
}

export interface CreateUserRequest {
  login: string
  displayName: string
  role: UserRole
  password: string
}

export interface UpdateUserRequest {
  displayName?: string
  role?: UserRole
  isActive?: boolean
}

export interface ResetPasswordRequest {
  newPassword: string
}

export interface UsersListResponse {
  users: UserDTO[]
  total: number
}

export interface AuditLogDTO {
  id: number
  actorUserId: number | null
  action: string
  targetType: string
  targetId: number | null
  meta: string
  createdAt: string
}

export interface AuditLogsResponse {
  logs: AuditLogDTO[]
  total: number
  page: number
}

// --- OCR Status Types ---

export interface OCRStatus {
  enabled: boolean
  health: string
  lastCheck?: string
  error?: string
  serviceUrl?: string
}

export interface OCRStatusResponse {
  status: OCRStatus
}

// --- EXIF Service Status Types ---

export interface ExifServiceStatus {
  enabled: boolean
  health: "healthy" | "unhealthy" | "disabled" | string
  lastCheck: string
  error: string
  serviceURL: string
}

// --- OCR Classification Types ---

export interface OcrBoundingBoxDTO {
  x: number
  y: number
  width: number
  height: number
  word: string
  confidence: number
}

export interface OcrDocumentDTO {
  id: number
  imageFileId: number
  path: string
  fileName: string
  dirPath: string
  size: number
  sizeHuman: string
  modTime: string
  thumbnail?: string
  thumbnailCachePath?: string
  meanConfidence: number
  weightedConfidence: number
  tokenCount: number
  angle: number
  scaleFactor: number
}

export interface OcrDocumentsResponse {
  documents: OcrDocumentDTO[]
  total: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

export interface OcrDataResponse {
  imagePath: string
  angle: number
  scaleFactor: number
  isTextDocument: boolean
  boundingBoxWidth: number
  boundingBoxHeight: number
  boxes: OcrBoundingBoxDTO[]
}

export interface OcrClassificationStatusResponse {
  processing: boolean
  incremental: boolean
  progress: string
  filesProcessed: number
  totalFiles: number
}

// --- LLM OCR Types ---

export type LlmProviderType = "ollama" | "ollama_cloud" | "openai" | "deepseek"

export interface LlmProviderDTO {
  id: number
  alias: string
  name: LlmProviderType
  apiUrl: string
  apiKey: string
  model: string
  cachedModels: LlmModelDTO[] | null
}

export interface LlmSettingsResponse {
  id: number
  activeProvider: string // Chat/text LLM provider alias
  vlProvider: string // VL (vision-language) provider alias for image analysis
  tagScanEnabled?: boolean
  tagScanStartHour?: number
  tagScanStartMinute?: number
  tagScanEndHour?: number
  tagScanEndMinute?: number
  tagScanTimezoneOffset?: number // User's timezone offset in minutes (JS getTimezoneOffset: UTC+3 = -180)
  embeddingProviderAlias?: string
  embeddingModel?: string
  embeddingDimension?: number
  embeddingBatchSize?: number
  providers: LlmProviderDTO[]
}

export interface UpdateLlmSettingsRequest {
  activeProvider?: string // Chat/text LLM provider alias
  vlProvider?: string // VL provider alias for image analysis
  providerAlias?: string // Which provider to update/delete (by alias)
  providerName?: LlmProviderType // For new providers
  providerApiUrl?: string
  providerApiKey?: string
  providerModel?: string
  providerAliasUpdate?: string // New alias value for the provider
  providerDelete?: boolean // Delete the provider identified by providerAlias
  tagScanEnabled?: boolean
  tagScanStartHour?: number
  tagScanStartMinute?: number
  tagScanEndHour?: number
  tagScanEndMinute?: number
  tagScanTimezoneOffset?: number // User's timezone offset in minutes (JS getTimezoneOffset: UTC+3 = -180)
  embeddingProviderAlias?: string
  embeddingModel?: string
  embeddingDimension?: number
  embeddingBatchSize?: number
}

export interface TagScanStatusResponse {
  running: boolean
  paused: boolean
  enabled: boolean
  schedule: string
  scanned: number
  remaining: number
  total: number
  currentImage?: string
  lastError?: string
}

export interface LlmOcrRequest {
  imagePath: string
  force?: boolean
}

export interface LlmOcrResponse {
  success: boolean
  markdownContent?: string
  language: string
  provider: string
  model: string
  processingTimeMs: number
  error?: string
}

export interface LlmRecognizeStatusResponse {
  status: "processing" | "completed" | "failed" | "not_found"
  markdownContent?: string
  language?: string
  provider?: string
  model?: string
  processingTimeMs?: number
  error?: string
}

export interface LlmOcrDataResponse {
  found: boolean
  markdownContent?: string
  language?: string
  provider?: string
  model?: string
  processingTimeMs?: number
  success?: boolean
  error?: string
  createdAt?: string
}

export interface LlmModelDTO {
  id: string
  name: string
  size?: number
  contextLength?: number
}

export interface LlmModelsResponse {
  success: boolean
  models: LlmModelDTO[]
  error?: string
  provider: string
}

// --- Thumbnail Cache Types ---

export interface ThumbnailCacheStatsResponse {
  totalSize: number
  totalFiles: number
  cacheDir: string
  enabled: boolean
  initialized: boolean
}

export interface ThumbnailCacheStatusResponse {
  enabled: boolean
  cacheDir: string
  filesCount: number
  totalSize: number
  totalSizeHuman: string
}

export interface WarmupThumbnailsRequest {
  filePaths: string[]
}

// --- AI Assistant Types ---

export type AiActionType = "describe" | "tags" | "recognizeText" | "askQuestion"

export interface AiActionRequest {
  imagePath: string
  action: AiActionType
  question?: string  // Only for "askQuestion" action
  language?: string  // UI language code (e.g. "en", "ru")
  force?: boolean    // Force regeneration, skip cached results
}

export interface AiActionStartResponse {
  taskId: string
  action: AiActionType
  status: string  // "processing"
}

export interface AiActionStatusResponse {
  taskId: string
  status: string  // "processing", "completed", "failed"
  action: AiActionType
  result?: string
  tags?: string[]  // Only for "tags" action
  error?: string
  provider?: string
  model?: string
  processingTimeMs?: number
}

export interface AiActionResponse {
  success: boolean
  action: AiActionType
  result?: string
  tags?: string[]  // Only for "tags" action
  error?: string
  provider?: string
  model?: string
  processingTimeMs?: number
}

// --- Geocode / GPS Types ---

export interface GeocodeSearchResult {
  lat: number
  lon: number
  displayName: string
  type: string
}

export interface GeocodeSearchResponse {
  results: GeocodeSearchResult[]
}

export interface UpdateGpsRequest {
  path: string
  lat: number
  lng: number
}

export interface UpdateGpsResponse {
  success: boolean
  lat: number
  lng: number
  nameLocal: string
  nameEng: string
}

export interface LocationCandidate {
  lat: number
  lng: number
  nameLocal: string
  nameEng: string
  photoCount: number
  thumbnail?: string
}

export interface LocationCandidatesResponse {
  candidates: LocationCandidate[]
}

export interface BatchUpdateGpsRequest {
  paths: string[]
  lat: number
  lng: number
}

export interface BatchUpdateGpsResponse {
  success: number
  failed: number
  skipped: number
  failedFiles?: string[]
  nameLocal: string
  nameEng: string
  lat: number
  lng: number
}

// --- Chat / Agent Types ---

export interface Conversation {
  id: number
  imagePath?: string
  title: string
  summary?: string
  tokenCount: number
  maxTokens: number
  language: string
  createdAt: string
  updatedAt: string
}

export interface ChatMessage {
  id: number
  role: "user" | "assistant" | "system" | "tool"
  content: string
  toolCalls?: ChatToolCallInfo[]
  createdAt: string
}

export interface ChatToolCallInfo {
  name: string
  arguments: string
  result: string
}

export interface CreateConversationRequest {
  imagePath?: string
  language?: string
}

export interface SendMessageRequest {
  content: string
}

// SSE event types from the agent
export interface SSEToolCallEvent {
  type: "tool_call"
  name: string
  status: "running" | "completed"
}

export interface SSEToolResultEvent {
  type: "tool_result"
  name: string
  status: string
  result: string
}

export interface SSEMessageEvent {
  type: "message"
  content: string
}

export interface SSEErrorEvent {
  type: "error"
  error: string
}

export interface SSEDoneEvent {
  type: "done"
}

export interface SSETokenUsageEvent {
	type: "token_usage"
	tokenCount: number
	maxTokens: number
	// DeepSeek-specific extended usage fields
	promptTokens?: number
	completionTokens?: number
	promptCacheHitTokens?: number
	promptCacheMissTokens?: number
	reasoningTokens?: number
}

export type SSEEvent = SSEToolCallEvent | SSEToolResultEvent | SSEMessageEvent | SSEErrorEvent | SSEDoneEvent | SSETokenUsageEvent

// --- Tag Search Types ---

export interface TagSearchResult {
  id: number
  path: string
  fileName: string
  modTime?: string
}

export interface TagSearchResponse {
  images: TagSearchResult[]
  total: number
}

// --- Smart Search Types ---

// --- Image Tags Types ---

export interface ImageTagsResponse {
  tags: string[]
}

// --- Smart Search Types ---

export interface SmartSearchResult {
  id: number
  path: string
  fileName: string
  modTime?: string
  similarity: number
  tags: string[]
  matchType?: "exact" | "embedding" | "both"
}

export interface SmartSearchResponse {
  images: SmartSearchResult[]
  total: number
  query: string
}

// --- Embedding Backfill Types ---

export interface EmbeddingBackfillProgress {
  total: number
  processed: number
  remaining: number
  lastError: string
}

export interface EmbeddingBackfillStatus {
	running: boolean
	progress: EmbeddingBackfillProgress
}

// --- Move Files Types ---

export interface MoveFilesRequest {
	filePaths: string[]
	targetDir: string
}

export interface MoveFilesResponse {
	success: number
	failed: number
	failedFiles?: string[]
}

// --- Create Subfolder Types ---

export interface CreateFolderRequest {
	parentPath: string
	folderName: string
}

export interface CreateFolderResponse {
	message: string
	path: string
}

// --- Subdirectory Listing Types ---

export interface SubdirEntry {
	name: string
	path: string
}

export interface SubdirsResponse {
	subdirs: SubdirEntry[]
	path: string
}

export interface MoveFilesResponse {
	success: number
	failed: number
	failedFiles?: string[]
}
