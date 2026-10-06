import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import {
  AUTH_LANGUAGE_STORAGE_KEY,
  readStoredAuthLanguage,
  resolveInitialAuthLanguage,
  resolveSystemLanguage,
  selectActiveLanguage,
  writeStoredAuthLanguage,
} from "./language"

function installLocalStorage(): void {
  const store = new Map<string, string>()
  const storage: Storage = {
    get length() {
      return store.size
    },
    clear: () => {
      store.clear()
    },
    getItem: (key: string) => (store.has(key) ? store.get(key) ?? null : null),
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }
  vi.stubGlobal("localStorage", storage)
}

describe("resolveSystemLanguage", () => {
  it("selects Russian for Russian locales", () => {
    expect(resolveSystemLanguage(["ru"])).toBe("ru")
    expect(resolveSystemLanguage(["ru-RU"])).toBe("ru")
  })

  it("selects English for English locales", () => {
    expect(resolveSystemLanguage(["en"])).toBe("en")
    expect(resolveSystemLanguage(["en-US"])).toBe("en")
  })

  it("falls back to English for unsupported locales", () => {
    expect(resolveSystemLanguage(["de"])).toBe("en")
    expect(resolveSystemLanguage(["fr-FR"])).toBe("en")
  })

  it("falls back to English when no locale is available", () => {
    expect(resolveSystemLanguage([])).toBe("en")
  })
})

describe("stored pre-authentication language", () => {
  beforeEach(() => {
    installLocalStorage()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("writes and reads back a supported language", () => {
    writeStoredAuthLanguage("ru")
    expect(localStorage.getItem(AUTH_LANGUAGE_STORAGE_KEY)).toBe("ru")
    expect(readStoredAuthLanguage()).toBe("ru")
  })

  it("ignores an unsupported stored value", () => {
    localStorage.setItem(AUTH_LANGUAGE_STORAGE_KEY, "de")
    expect(readStoredAuthLanguage()).toBeNull()
  })

  it("returns null when nothing is stored", () => {
    expect(readStoredAuthLanguage()).toBeNull()
  })

  it("is resilient when storage throws", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("blocked")
      },
      setItem: () => {
        throw new Error("blocked")
      },
    } as unknown as Storage)

    expect(readStoredAuthLanguage()).toBeNull()
    expect(() => writeStoredAuthLanguage("ru")).not.toThrow()
  })
})

describe("resolveInitialAuthLanguage", () => {
  beforeEach(() => {
    installLocalStorage()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("prefers a remembered supported value over the system default", () => {
    writeStoredAuthLanguage("ru")
    expect(resolveInitialAuthLanguage(["en-US"])).toBe("ru")
  })

  it("uses the system default when nothing valid is stored", () => {
    expect(resolveInitialAuthLanguage(["ru-RU"])).toBe("ru")
    expect(resolveInitialAuthLanguage(["de"])).toBe("en")
  })

  it("ignores an unsupported stored value", () => {
    localStorage.setItem(AUTH_LANGUAGE_STORAGE_KEY, "fr")
    expect(resolveInitialAuthLanguage(["ru"])).toBe("ru")
  })
})

describe("selectActiveLanguage", () => {
  it("uses the account language when authenticated", () => {
    expect(selectActiveLanguage(true, "ru", "en")).toBe("ru")
  })

  it("uses the pre-authentication language when unauthenticated", () => {
    expect(selectActiveLanguage(false, "ru", "en")).toBe("en")
  })
})
