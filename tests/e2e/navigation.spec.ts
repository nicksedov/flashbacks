import type { Locator, Page } from "@playwright/test"
import { test, expect } from "./fixtures"
import { HAS_CREDENTIALS, SKIP_REASON } from "./support/env"
import { GALLERY_LABELS, openNav, sidebar, type NavKey } from "./support/ui"

/**
 * One marker per sidebar destination: something only that screen renders.
 *
 * Labels are matched in both shipped locales, and the lightbox/gallery hooks
 * come from the shared tile and tab components rather than from copy that is
 * likely to be reworded.
 */
const SCREEN: Record<NavKey, (page: Page) => Locator> = {
  "gallery-all-images": (page) => page.getByPlaceholder(GALLERY_LABELS.search),
  "gallery-folders": (page) => page.getByRole("button", { name: /Gallery root|Корень галереи/ }).first(),
  "gallery-calendar": (page) =>
    page.getByRole("button", { name: /Newest first|Oldest first|Сначала новые|Сначала старые/ }).first(),
  "gallery-geolocation": (page) => page.locator(".leaflet-container").first(),
  "smart-search": (page) => page.getByRole("heading", { name: /Smart Search|Умный поиск/ }).first(),
  deduplication: (page) => page.getByRole("button", { name: /^(Rescan|Сканировать)$/ }).first(),
  ocr: (page) => page.getByRole("heading", { name: /OCR Documents|OCR Документы/ }).first(),
  exif: (page) =>
    page
      .getByText(
        /missing EXIF data|без EXIF данных|complete EXIF data|полные EXIF данные/,
      )
      .first(),
  people: (page) => page.locator('[data-slot="people-search"]'),
  "gallery-trash": (page) => page.getByText(/Trash is empty|Корзина пуста|file\(s\)|файлов/).first(),
  "admin-users": (page) => page.getByRole("heading", { name: /Users|Пользователи/ }).first(),
  "admin-settings": (page) => page.getByRole("heading", { name: /^(Settings|Настройки)$/ }).first(),
  about: (page) => page.getByRole("heading", { name: /^(About|О программе)$/ }).first(),
}

/** Destinations every signed-in account sees. */
const USER_DESTINATIONS: NavKey[] = [
  "gallery-all-images",
  "gallery-folders",
  "gallery-calendar",
  "gallery-geolocation",
  "smart-search",
  "deduplication",
  "ocr",
  "exif",
  "people",
  "gallery-trash",
  "about",
]

/** Destinations only an administrator sees. */
const ADMIN_DESTINATIONS: NavKey[] = ["admin-users", "admin-settings"]

test.describe("Workspace navigation", () => {
  test.skip(!HAS_CREDENTIALS, SKIP_REASON)

  test("shows the smart search index hint when no photo has embeddings", async ({ page, api }) => {
    const status = await api.embeddingStatus()
    test.skip(
      status.progress.total > 0,
      "The deployment has an embedding index, so smart search is not in its degraded state.",
    )

    await openNav(page, "smart-search")
    await expect(
      page.getByText(/No embeddings found|Эмбеддинги не найдены/).first(),
    ).toBeVisible()
    await expect(
      page.getByText(/Make sure an embedding model is configured|модель эмбеддингов настроена/).first(),
    ).toBeVisible()
  })

  test("shows the deduplication empty state when there are no duplicate groups", async ({ page, api }) => {
    const duplicates = await api.duplicates()
    test.skip(duplicates.totalGroups > 0, "The deployment has duplicate groups, so the empty state is not shown.")

    await openNav(page, "deduplication")
    await expect(page.getByText(/No Duplicates Found|Дубликаты не найдены/).first()).toBeVisible()
  })

  test("shows the empty trash state when the trash has no files", async ({ page, api }) => {
    const trash = await api.trash()
    test.skip(trash.totalItems > 0, "The deployment has files in the trash, so the empty state is not shown.")

    await openNav(page, "gallery-trash")
    await expect(page.getByText(/Trash is empty|Корзина пуста/).first()).toBeVisible()
  })

  test("opens every destination the signed-in account can see", async ({ page, api }) => {
    const user = await api.currentUser()
    const destinations = user.role === "admin" ? [...USER_DESTINATIONS, ...ADMIN_DESTINATIONS] : USER_DESTINATIONS
    const main = page.locator("main")

    for (const destination of destinations) {
      await test.step(`opens ${destination}`, async () => {
        await openNav(page, destination)
        await expect(SCREEN[destination](page)).toBeVisible()
        // A screen that renders nothing is a blank page even without an error.
        await expect
          .poll(async () => (await main.innerText()).trim().length, { message: `${destination} rendered no content` })
          .toBeGreaterThan(0)
        // The workspace shell survives every navigation.
        await expect(sidebar(page)).toBeVisible()
      })
    }
  })
})
