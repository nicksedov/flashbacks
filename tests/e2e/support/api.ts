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

async function readJson<T>(response: APIResponse, action: string): Promise<T> {
  if (!response.ok()) {
    throw new Error(`${action} failed: HTTP ${response.status()} — ${await response.text()}`)
  }
  return (await response.json()) as T
}

/**
 * Thin client for the catalog endpoints the specs seed and verify with.
 *
 * Seeding through the API (instead of clicking everything into existence)
 * keeps the UI walkthrough focused on the screens under test while still
 * exercising the same authenticated contract the webapp uses.
 */
export class PeopleApi {
  constructor(private readonly request: APIRequestContext) {}

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
}
