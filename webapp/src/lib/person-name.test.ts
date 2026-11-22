import { describe, expect, it } from "vitest"
import { ApiError } from "@/api/client"
import {
  findPersonNameConflict,
  nameConflictFromError,
  normalizePersonName,
  PERSON_NAME_SUGGESTION_LIMIT,
  personNameSuggestions,
} from "@/lib/person-name"
import type { PersonDTO } from "@/types"

function person(id: number, name: string, photoCount = 3): PersonDTO {
  return { id, name, photoCount, coverThumbnail: "" }
}

describe("normalizePersonName", () => {
  it.each([
    ["", ""],
    ["   ", ""],
    ["  Anna  ", "anna"],
    ["Anna   Petrova", "anna petrova"],
    ["Anna\tPetrova", "anna petrova"],
    ["ИВАН", "иван"],
    [" иВаН ", "иван"],
  ])("normalizes %j to %j", (input, expected) => {
    expect(normalizePersonName(input)).toBe(expected)
  })
})

describe("findPersonNameConflict", () => {
  const people = [person(1, "Иван"), person(2, "Анна"), person(3, "")]

  it("finds the person that already holds the name", () => {
    expect(findPersonNameConflict(" иван ", people)?.id).toBe(1)
  })

  it("ignores the person being renamed", () => {
    expect(findPersonNameConflict("Иван", people, [1])).toBeNull()
  })

  it("finds nothing for a free name", () => {
    expect(findPersonNameConflict("Пётр", people)).toBeNull()
  })

  it("never reports unnamed people as a conflict", () => {
    expect(findPersonNameConflict("", people)).toBeNull()
    expect(findPersonNameConflict("   ", people)).toBeNull()
  })
})

describe("nameConflictFromError", () => {
  const conflict = person(1, "Иван", 12)

  it("reads the holder from a 409 body", () => {
    const error = new ApiError("taken", 409, { error: "people.name_taken", conflict })
    expect(nameConflictFromError(error)?.id).toBe(1)
    expect(nameConflictFromError(error)?.photoCount).toBe(12)
  })

  it("ignores other statuses and plain errors", () => {
    expect(nameConflictFromError(new ApiError("boom", 500, {}))).toBeNull()
    expect(nameConflictFromError(new Error("boom"))).toBeNull()
    expect(nameConflictFromError(null)).toBeNull()
  })
})

describe("personNameSuggestions", () => {
  // Names run in the opposite order to the photo counts, so a list ordered by
  // name instead of by count cannot pass the ranking assertions below.
  const catalog = [
    person(1, "Зоя", 30),
    person(2, "Жанна", 25),
    person(3, "Елена", 20),
    person(4, "Дмитрий", 15),
    person(5, "Галина", 10),
    person(6, "Василиса", 5),
    person(7, "Борис", 3),
    person(8, "Анна", 1),
  ]

  it("ranks named people by photo count", () => {
    const result = personNameSuggestions(catalog, "")

    expect(result).toHaveLength(PERSON_NAME_SUGGESTION_LIMIT)
    expect(result.map((entry) => entry.id)).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  it("breaks equal photo counts by name", () => {
    const result = personNameSuggestions([person(1, "Борис", 5), person(2, "Анна", 5)], "")

    expect(result.map((entry) => entry.name)).toEqual(["Анна", "Борис"])
  })

  it("keeps only names containing the fragment", () => {
    const people = [
      person(1, "Иван Петров", 2),
      person(2, "Пётр", 20),
      person(3, "Иванка", 9),
    ]

    const result = personNameSuggestions(people, "иван")

    expect(result.map((entry) => entry.id)).toEqual([3, 1])
  })

  it("ignores case and surrounding spaces", () => {
    const result = personNameSuggestions([person(1, "Иван Петров", 4)], " ИВАН ")

    expect(result.map((entry) => entry.id)).toEqual([1])
  })

  it("returns nothing when no name contains the fragment", () => {
    expect(personNameSuggestions(catalog, "zzz")).toEqual([])
  })

  it("excludes the person being renamed", () => {
    const people = [person(1, "Иван", 9), person(2, "Иван Петров", 4)]

    const result = personNameSuggestions(people, "иван", { excludeIds: [1] })

    expect(result.map((entry) => entry.id)).toEqual([2])
  })

  it("skips unnamed people", () => {
    const people = [person(1, "", 9), person(2, "   ", 8), person(3, "Анна", 1)]

    expect(personNameSuggestions(people, "").map((entry) => entry.id)).toEqual([3])
  })
})
