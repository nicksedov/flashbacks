import { useEffect, useState } from "react"
import { fetchImageFaces } from "@/api/endpoints"
import type { ImageFaceDTO } from "@/types"

/** One person present on the image, numbered for the overlay and the list. */
export interface ImagePerson {
  /** The number drawn on this person's boxes and in the people list, from 1. */
  number: number
  personId: number
  /** Empty while the person is unnamed; the UI renders its localized fallback. */
  name: string
}

export interface UseImageFacesResult {
  /** Every detected face, including the ones no person claims. */
  faces: ImageFaceDTO[]
  /** The people of the photo, numbered by first appearance. */
  people: ImagePerson[]
  loading: boolean
  error: boolean
}

/** The settled answer for one image path. */
interface LoadedFaces {
  path: string
  faces: ImageFaceDTO[]
  error: boolean
}

/**
 * Number the people of one photo: each distinct person appears once, numbered by
 * where their first face falls in the face list, so a numbered box and its list
 * entry always agree. A face with `personId` 0 belongs to nobody and is skipped —
 * the overlay draws it unnumbered.
 */
export function numberPeople(faces: ImageFaceDTO[]): ImagePerson[] {
  const people: ImagePerson[] = []
  const byPersonId = new Map<number, ImagePerson>()

  for (const face of faces) {
    if (face.personId === 0) continue

    const existing = byPersonId.get(face.personId)
    if (existing) {
      // A person's faces share one name; keep the first non-empty one.
      if (!existing.name && face.personName) existing.name = face.personName
      continue
    }

    const person: ImagePerson = {
      number: people.length + 1,
      personId: face.personId,
      name: face.personName,
    }
    byPersonId.set(face.personId, person)
    people.push(person)
  }

  return people
}

/**
 * Load the faces detected on one image. The request is issued only while the
 * People mode is active (`enabled`), so opening the lightbox does not pay for a
 * face lookup the user never looks at.
 *
 * The answer is stored together with the path it belongs to, and the loading flag
 * is derived from the absence of an answer for the current path. That keeps the
 * hook free of state resets when the lightbox moves to another photo — a stale
 * answer for a previous image simply stops matching.
 */
export function useImageFaces(imagePath: string | null, enabled: boolean): UseImageFacesResult {
  const [loaded, setLoaded] = useState<LoadedFaces | null>(null)
  const activePath = enabled && imagePath ? imagePath : null

  useEffect(() => {
    if (!activePath) return

    let cancelled = false
    fetchImageFaces(activePath)
      .then((response) => {
        if (cancelled) return
        setLoaded({ path: activePath, faces: response.faces ?? [], error: false })
      })
      .catch(() => {
        if (cancelled) return
        setLoaded({ path: activePath, faces: [], error: true })
      })

    return () => {
      cancelled = true
    }
  }, [activePath])

  const current = activePath && loaded?.path === activePath ? loaded : null

  return {
    faces: current?.faces ?? [],
    people: numberPeople(current?.faces ?? []),
    loading: activePath !== null && current === null,
    error: current?.error ?? false,
  }
}
