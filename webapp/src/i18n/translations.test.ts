import { describe, it, expect } from "vitest"
import { translationsEn } from "./translations.en"
import { translationsRu } from "./translations.ru"

const enKeys = Object.keys(translationsEn)
const ruKeys = Object.keys(translationsRu)

describe("i18n translation parity", () => {
  it("should have the same number of keys in EN and RU", () => {
    expect(enKeys.length).toBe(ruKeys.length)
  })

  it("every EN key should exist in RU", () => {
    const missingInRu = enKeys.filter((key) => !(key in translationsRu))
    expect(missingInRu).toEqual([])
  })

  it("every RU key should exist in EN", () => {
    const missingInEn = ruKeys.filter((key) => !(key in translationsEn))
    expect(missingInEn).toEqual([])
  })

  it("no EN translation value should be empty", () => {
    const emptyKeys = enKeys.filter(
      (key) => String(translationsEn[key as keyof typeof translationsEn]) === ""
    )
    expect(emptyKeys).toEqual([])
  })

  it("no RU translation value should be empty", () => {
    const emptyKeys = ruKeys.filter(
      (key) => String(translationsRu[key as keyof typeof translationsRu]) === ""
    )
    expect(emptyKeys).toEqual([])
  })

  it("EN and RU should have identical key sets", () => {
    const enSet = new Set(enKeys)
    const ruSet = new Set(ruKeys)
    const allKeys = new Set([...enKeys, ...ruKeys])

    const mismatches: { key: string; inEn: boolean; inRu: boolean }[] = []
    for (const key of allKeys) {
      const inEn = enSet.has(key)
      const inRu = ruSet.has(key)
      if (inEn !== inRu) {
        mismatches.push({ key, inEn, inRu })
      }
    }

    expect(mismatches).toEqual([])
  })
})

describe("i18n placeholder consistency", () => {
  const placeholderPattern = /\{(\w+)\}/g

  it("EN and RU should have the same placeholders for each key", () => {
    const mismatches: { key: string; enPlaceholders: string[]; ruPlaceholders: string[] }[] = []

    for (const key of enKeys) {
      if (!(key in translationsRu)) continue

      const enValue = translationsEn[key as keyof typeof translationsEn]
      const ruValue = translationsRu[key as keyof typeof translationsRu]

      const enPlaceholders = [...enValue.matchAll(placeholderPattern)].map((m) => m[1]).sort()
      const ruPlaceholders = [...ruValue.matchAll(placeholderPattern)].map((m) => m[1]).sort()

      if (JSON.stringify(enPlaceholders) !== JSON.stringify(ruPlaceholders)) {
        mismatches.push({ key, enPlaceholders, ruPlaceholders })
      }
    }

    expect(mismatches).toEqual([])
  })
})
