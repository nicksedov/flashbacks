import { ApiError } from "@/api/client"
import type { PersonDTO } from "@/types"

/**
 * Canonical form of a person's name, mirroring the core service:
 * surrounding whitespace is trimmed, every internal whitespace run collapses to
 * one space and the letters are lower-cased, so «Иван», «иван» and « Иван »
 * are one name. An empty result means "unnamed", which never conflicts.
 */
export function normalizePersonName(name: string): string {
  return name.replace(/\s+/g, " ").trim().toLowerCase()
}

/**
 * Returns the person that already holds the given name, or null when the name
 * is free. The person being renamed (and any other id in `excludeIds`) is
 * ignored, so re-casing your own name is not a conflict.
 *
 * This is a local pre-check against the already-loaded catalog; the server
 * stays authoritative, because another client may have taken the name.
 */
export function findPersonNameConflict(
  name: string,
  people: PersonDTO[],
  excludeIds: number[] = [],
): PersonDTO | null {
  const key = normalizePersonName(name)
  if (!key) return null

  const excluded = new Set(excludeIds)
  const holder = people.find(
    (person) => !excluded.has(person.id) && normalizePersonName(person.name) === key,
  )
  return holder ?? null
}

/** Body a 409 name conflict carries, as documented in the OpenAPI contract. */
interface PersonNameConflictBody {
  error?: string
  conflict?: PersonDTO
}

/**
 * Returns the person that holds the name when `err` is the 409 of a people
 * mutation, or null for any other failure. The catalog in the page can be stale,
 * so a conflict the pre-check missed still opens the same dialog.
 */
export function nameConflictFromError(err: unknown): PersonDTO | null {
  if (!(err instanceof ApiError) || err.status !== 409) return null
  const body = err.body as PersonNameConflictBody | null | undefined
  return body?.conflict ?? null
}
