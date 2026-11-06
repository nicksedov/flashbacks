import { expect, type APIRequestContext, type APIResponse } from "@playwright/test"

/** One person as returned by `GET /api/people`. */
export interface PersonDTO {
  id: number
  name: string
  photoCount: number
  coverThumbnail: string
}

/** One unassigned face as returned by `GET /api/faces/unassigned`. */
export interface UnassignedFaceDTO {
  id: number
  personId: number
  path: string
  fileName: string
  x: number
  y: number
  width: number
  height: number
  confidence: number
}

/** One face as returned by `GET /api/image-faces`. */
export interface ImageFaceDTO {
  id: number
  personId: number
  personName: string
  x: number
  y: number
  width: number
  height: number
  confidence: number
}

/** A gallery photo as returned by `GET /api/people/:id/photos`. */
export interface PersonPhotoDTO {
  id: number
  path: string
  fileName: string
}

export interface UserSettings {
  theme: string
  language: string
}

/** The signed-in account, as returned by `GET /api/auth/me`. */
export interface CurrentUserDTO {
  id: number
  login: string
  displayName: string
  role: "admin" | "user"
  status: string
}

/** A photo as returned by `GET /api/gallery`. */
export interface GalleryImageDTO {
  id: number
  path: string
  fileName: string
  dirPath: string
  size: number
  sizeHuman: string
  modTime: string
}

/** One page of `GET /api/gallery`. */
export interface GalleryImagesPage {
  images: GalleryImageDTO[]
  totalImages: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

/** A registered gallery folder as returned by `GET /api/folders`. */
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

export interface AddFolderResponse {
  message: string
  folder: GalleryFolderDTO
  scanStarted: boolean
}

/** One photo in the trash as returned by `GET /api/trash`. */
export interface TrashItemDTO {
  id: number
  fileName: string
  trashPath: string
  originalPath: string
  size: number
  sizeHuman: string
  deletedAt: string
  thumbnail: string
}

export interface TrashDateGroup {
  date: string
  label: string
  itemCount: number
  items: TrashItemDTO[]
}

export interface TrashListResponse {
  groups: TrashDateGroup[]
  totalItems: number
  totalGroups: number
  hasMore: boolean
  nextCursor?: string | null
}

/** System-wide settings as returned by `GET /api/settings`. */
export interface AppSettings {
  trashDir: string
  exifBackupDir: string
  thumbnailCachePath?: string
  thumbnailCacheSize?: number
  ocrConcurrentRequests?: number
  syncDays?: string
  dailySyncHour?: number
  dailySyncMinute?: number
  syncTimezoneOffset?: number
}

export interface DuplicateFileDTO {
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
  files: DuplicateFileDTO[]
}

export interface DuplicatesResponse {
  groups: DuplicateGroupDTO[]
  totalFiles: number
  totalGroups: number
  scannedDirs: string[]
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
  hasPrevPage: boolean
}

export interface CalendarDateGroup {
  date: string
  label: string
  imageCount: number
  images: GalleryImageDTO[]
}

export interface GalleryCalendarResponse {
  groups: CalendarDateGroup[]
  totalImages: number
  totalGroups: number
  hasMore: boolean
  dateRange: { minDate: string; maxDate: string; totalWithDate: number }
}

export interface ImageMetadataResponse {
  found: boolean
  metadata?: Record<string, unknown>
}

async function readJson<T>(response: APIResponse, action: string): Promise<T> {
  if (!response.ok()) {
    throw new Error(`${action} failed: HTTP ${response.status()} — ${await response.text()}`)
  }
  return (await response.json()) as T
}

/**
 * Thin client for the endpoints the specs seed and verify with.
 *
 * Seeding through the API (instead of clicking everything into existence)
 * keeps the UI walkthrough focused on the screens under test while still
 * exercising the same authenticated contract the webapp uses. The client is
 * bound to the browser context, so it speaks for the signed-in account.
 */
export class FlashbacksApi {
  constructor(private readonly request: APIRequestContext) {}

  // --- People catalog ---

  async people(): Promise<PersonDTO[]> {
    const body = await readJson<{ people?: PersonDTO[] }>(
      await this.request.get("/api/people"),
      "list people",
    )
    return body.people ?? []
  }

  async unassignedFaces(page = 1, pageSize = 100): Promise<UnassignedFaceDTO[]> {
    const body = await readJson<{ faces?: UnassignedFaceDTO[] }>(
      await this.request.get("/api/faces/unassigned", {
        params: { page: String(page), pageSize: String(pageSize) },
      }),
      "list unassigned faces",
    )
    return body.faces ?? []
  }

  async createPerson(name: string, faceIds: number[]): Promise<PersonDTO> {
    return readJson<PersonDTO>(
      await this.request.post("/api/people", {
        data: { name, faceIds },
      }),
      "create person",
    )
  }

  async renamePerson(id: number, name: string): Promise<PersonDTO> {
    return readJson<PersonDTO>(
      await this.request.patch(`/api/people/${id}`, { data: { name } }),
      `rename person ${id}`,
    )
  }

  async mergePeople(targetPersonId: number, sourcePersonIds: number[], name?: string): Promise<PersonDTO> {
    return readJson<PersonDTO>(
      await this.request.post("/api/people/merge", {
        data: { targetPersonId, sourcePersonIds, name },
      }),
      "merge people",
    )
  }

  /** Disband a person; an id that is already gone is treated as success. */
  async disbandPerson(id: number): Promise<void> {
    const response = await this.request.delete(`/api/people/${id}`)
    if (response.status() === 404) return
    if (!response.ok()) {
      throw new Error(`disband person ${id} failed: HTTP ${response.status()} — ${await response.text()}`)
    }
  }

  async assignFace(faceId: number, personId: number): Promise<ImageFaceDTO> {
    return readJson<ImageFaceDTO>(
      await this.request.patch(`/api/faces/${faceId}`, { data: { personId } }),
      `assign face ${faceId}`,
    )
  }

  async personPhotos(id: number, page = 1, pageSize = 100): Promise<PersonPhotoDTO[]> {
    const body = await readJson<{ images?: PersonPhotoDTO[] }>(
      await this.request.get(`/api/people/${id}/photos`, {
        params: { page: String(page), pageSize: String(pageSize) },
      }),
      `list photos of person ${id}`,
    )
    return body.images ?? []
  }

  async imageFaces(imagePath: string): Promise<ImageFaceDTO[]> {
    const body = await readJson<{ faces?: ImageFaceDTO[] }>(
      await this.request.get("/api/image-faces", { params: { path: imagePath } }),
      "list image faces",
    )
    return body.faces ?? []
  }

  async userSettings(): Promise<UserSettings> {
    return readJson<UserSettings>(await this.request.get("/api/user-settings"), "read user settings")
  }

  async updateUserSettings(settings: UserSettings): Promise<void> {
    const response = await this.request.put("/api/user-settings", { data: settings })
    expect(response.ok(), `update user settings: HTTP ${response.status()}`).toBeTruthy()
  }

  // --- Account ---

  async currentUser(): Promise<CurrentUserDTO> {
    const body = await readJson<{ user: CurrentUserDTO }>(
      await this.request.get("/api/auth/me"),
      "read current user",
    )
    return body.user
  }

  // --- Gallery ---

  /** One page of the gallery, defaulting to the all-images view the UI shows. */
  async galleryImages(options: {
    page?: number
    pageSize?: number
    view?: string
    sortOrder?: string
    search?: string
    dirPath?: string
  } = {}): Promise<GalleryImagesPage> {
    const { page = 1, pageSize = 60, view = "allImages", sortOrder = "newest", search, dirPath } = options
    const params: Record<string, string> = {
      page: String(page),
      pageSize: String(pageSize),
      view,
      sortOrder,
    }
    if (search) params.search = search
    if (dirPath) params.dirPath = dirPath
    return readJson<GalleryImagesPage>(
      await this.request.get("/api/gallery", { params }),
      "list gallery images",
    )
  }

  /** Every photo path the gallery knows, following the pages. */
  async galleryImagePaths(pageSize = 200): Promise<string[]> {
    const paths: string[] = []
    for (let page = 1; ; page += 1) {
      const body = await this.galleryImages({ page, pageSize })
      paths.push(...body.images.map((image) => image.path))
      if (!body.hasNextPage) return paths
    }
  }

  async galleryFolders(): Promise<GalleryFolderDTO[]> {
    const body = await readJson<GalleryFoldersResponse>(
      await this.request.get("/api/folders"),
      "list gallery folders",
    )
    return body.folders ?? []
  }

  async addFolder(path: string): Promise<GalleryFolderDTO> {
    const body = await readJson<AddFolderResponse>(
      await this.request.post("/api/folders", { data: { path } }),
      `add gallery folder ${path}`,
    )
    return body.folder
  }

  /** Remove a folder registration; an id that is already gone is success. */
  async removeFolder(id: number): Promise<void> {
    const response = await this.request.delete(`/api/folders/${id}`)
    if (response.status() === 404) return
    if (!response.ok()) {
      throw new Error(`remove folder ${id} failed: HTTP ${response.status()} — ${await response.text()}`)
    }
  }

  async folderByPath(path: string): Promise<GalleryFolderDTO | undefined> {
    const folders = await this.galleryFolders()
    return folders.find((folder) => folder.path === path)
  }

  async calendar(pageSize = 100): Promise<GalleryCalendarResponse> {
    return readJson<GalleryCalendarResponse>(
      await this.request.get("/api/gallery/calendar", { params: { page: "1", pageSize: String(pageSize) } }),
      "read gallery calendar",
    )
  }

  async duplicates(page = 1, pageSize = 100): Promise<DuplicatesResponse> {
    return readJson<DuplicatesResponse>(
      await this.request.get("/api/duplicates", {
        params: { page: String(page), pageSize: String(pageSize) },
      }),
      "list duplicates",
    )
  }

  /** Embedding backfill progress, which tells whether smart search can work. */
  async embeddingStatus(): Promise<{
    running: boolean
    progress: { total: number; processed: number; remaining: number; lastError: string }
  }> {
    return readJson(
      await this.request.get("/api/embedding/status"),
      "read embedding status",
    )
  }

  async imageMetadata(path: string): Promise<ImageMetadataResponse> {
    return readJson<ImageMetadataResponse>(
      await this.request.get("/api/image-metadata", { params: { path } }),
      `read metadata of ${path}`,
    )
  }

  // --- Trash ---

  async trash(cursor?: string | null): Promise<TrashListResponse> {
    return readJson<TrashListResponse>(
      await this.request.get("/api/trash", cursor ? { params: { cursor } } : undefined),
      "list trash",
    )
  }

  /** Flattened trash rows across every returned date group. */
  async trashItems(): Promise<TrashItemDTO[]> {
    const body = await this.trash()
    return body.groups.flatMap((group) => group.items)
  }

  async trashItemByOriginalPath(path: string): Promise<TrashItemDTO | undefined> {
    const items = await this.trashItems()
    return items.find((item) => item.originalPath === path)
  }

  /** Restore a trashed file and return where it landed. */
  async restoreTrashFile(id: number): Promise<string> {
    const body = await readJson<{ success: boolean; restoredPath: string }>(
      await this.request.post("/api/trash-restore", { data: { id } }),
      `restore trash item ${id}`,
    )
    return body.restoredPath
  }

  /** Delete a trash row for good; a row that is already gone is success. */
  async deleteTrashFile(id: number): Promise<void> {
    const response = await this.request.post("/api/trash-delete", { data: { id } })
    if (response.status() === 404) return
    if (!response.ok()) {
      throw new Error(`delete trash item ${id} failed: HTTP ${response.status()} — ${await response.text()}`)
    }
  }

  async cleanTrash(): Promise<{ deleted: number; failed: number }> {
    return readJson<{ deleted: number; failed: number }>(
      await this.request.post("/api/trash-clean"),
      "clean trash",
    )
  }

  // --- Settings ---

  async appSettings(): Promise<AppSettings> {
    return readJson<AppSettings>(await this.request.get("/api/settings"), "read app settings")
  }

  async updateAppSettings(patch: Partial<AppSettings>): Promise<AppSettings> {
    return readJson<AppSettings>(
      await this.request.put("/api/settings", { data: patch }),
      "update app settings",
    )
  }
}
