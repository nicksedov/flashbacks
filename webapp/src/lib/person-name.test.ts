import { describe, expect, it } from "vitest"
import { ApiError } from "@/api/client"
import {
  findPersonNameConflict,
  nameConflictFromError,
  normalizePersonName,
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
