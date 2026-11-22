import { ApiError } from "@/api/client"
import type { PersonDTO } from "@/types"

/** How many name suggestions a rename dialog offers at once. */
export const PERSON_NAME_SUGGESTION_LIMIT = 7

/** Narrowing options for {@link personNameSuggestions}. */
export interface PersonNameSuggestionOptions {
  /** People that must never be offered, typically the person being renamed. */
  excludeIds?: number[]
  /** Cap of returned entries; defaults to {@link PERSON_NAME_SUGGESTION_LIMIT}. */
  limit?: number
  /** Interface language, used only to order people with equal photo counts. */
  locale?: string
}

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
 * Candidates the rename dialog offers under the name input.
 *
 * With an empty query the list is the named people with the most photos; once
 * the user types, it keeps only names containing the typed fragment. Both the
 * fragment and the names go through {@link normalizePersonName}, so matching
 * ignores case and surrounding or repeated whitespace exactly like the
 * uniqueness rule does. Unnamed people carry nothing to suggest and the ids in
 * `excludeIds` (the person being renamed) are skipped; equal photo counts are
 * ordered by name so the list is stable between openings.
 */
export function personNameSuggestions(
  people: PersonDTO[],
  query: string,
  options: PersonNameSuggestionOptions = {},
): PersonDTO[] {
  const { excludeIds = [], limit = PERSON_NAME_SUGGESTION_LIMIT, locale } = options
  const excluded = new Set(excludeIds)
  const needle = normalizePersonName(query)

  return people
    .filter((person) => {
      const key = normalizePersonName(person.name)
      if (!key || excluded.has(person.id)) return false
      return needle === "" || key.includes(needle)
    })
    .sort((left, right) => {
      if (left.photoCount !== right.photoCount) return right.photoCount - left.photoCount
      return normalizePersonName(left.name).localeCompare(normalizePersonName(right.name), locale)
    })
    .slice(0, Math.max(limit, 0))
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
