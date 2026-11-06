import { expect, type Locator, type Page } from "@playwright/test"

/**
 * Locators and flows shared by the specs.
 *
 * Every user-facing label is matched in both shipped locales (en/ru), so a
 * deployment in either language works without a per-spec switch. Sidebar
 * lookups are scoped to the `<aside>` element to avoid matching the page
 * content, which repeats some of the same words.
 */

/**
 * Login screen labels (the auth screen reuses the `adminPanel.*` keys).
 *
 * A rejected sign-in is reported either with the credentials message or, when
 * the API layer short-circuits a 401 into "Authorization required", with that
 * message — both are rejected logins, so the label matches either one.
 */
export const AUTH_LABELS = {
  login: /^(Login|Логин)$/,
  loginPlaceholder: /Enter login|Введите логин/,
  password: /^(Enter password|Введите пароль)$/,
  signIn: /^(Sign In|Войти)$/,
  signOut: /^(Logout|Выход)$/,
  invalidCredentials: /Invalid login or password|Неверный логин или пароль|Authorization required|Требуется авторизация/,
} as const

/** The inline error box below the login form. */
export function loginError(page: Page): Locator {
  return page.locator("form .text-destructive").first()
}

/** Gallery screen labels that carry no stable test hook. */
export const GALLERY_LABELS = {
  search: /Search by folder or file name|Поиск по имени папки или файла/,
  clearSearch: /^(Clear search|Очистить поиск)$/,
  rootFolders: /^(Gallery Folders|Папки галереи)$/,
} as const

/** Labels of the destructive maintenance screens, matched in both locales. */
export const MAINTENANCE_LABELS = {
  adminGeneralTab: /^(General|Основные)$/,
  trashSection: /^(Trash|Корзина)$/,
  save: /^(Save|Сохранить)$/,
  rescanAll: /^(Rescan All|Сканировать все)$/,
  addFolder: /^(Add Folder|Добавить папку)$/,
  deleteConfirm: /^(Delete|Удалить)$/,
  restore: /^(Restore|Восстановить)$/,
  emptyTrash: /^(Empty Trash|Очистить корзину)$/,
  batchDedup: /^(Batch Dedup|Пакетная дедупликация)$/,
  applyRules: /^(Apply Rules|Применить правила)$/,
  skipThis: /^(Skip this|Пропустить этот)$/,
  forward: /^(Forward →|Вперёд →|Finish|Завершить)$/,
  close: /^(Close|Закрыть)$/,
  trashEmpty: /Trash is empty|Корзина пуста/,
} as const

/** Lightbox modes, mirroring the tab ids of the shared lightbox chrome. */
export type LightboxMode = "ai" | "exif" | "ocr" | "people" | "tags"

/** One photo tile in any gallery grid, matched by its file name. */
export function galleryTile(page: Page, fileName: string): Locator {
  return page.locator('[data-slot="gallery-tile"]').filter({ hasText: fileName }).first()
}

/** The open lightbox dialog. */
export function lightbox(page: Page): Locator {
  return page.getByRole("dialog").first()
}

/** One mode tab of the shared lightbox tab bar. */
export function lightboxMode(page: Page, mode: LightboxMode): Locator {
  return page.locator(`[data-slot="lightbox-mode"][data-mode="${mode}"]`).first()
}

// --- Maintenance screens ---

/** Admin settings, General tab: gallery folders, sync schedule and the trash. */
export async function openAdminGeneral(page: Page): Promise<void> {
  await openNav(page, "admin-settings")
  await page.getByRole("tab", { name: MAINTENANCE_LABELS.adminGeneralTab }).click()
  // The rescan action only exists on this tab, so it proves the panel rendered.
  await expect(page.getByRole("button", { name: MAINTENANCE_LABELS.rescanAll }).first()).toBeVisible()
}

/** Save the trash directory through the admin settings form. */
export async function saveTrashDir(page: Page, dir: string): Promise<void> {
  await openAdminGeneral(page)
  const input = page.locator("#trash-dir-input")
  await expect(input).toBeVisible()
  await input.fill(dir)
  // The form's Save button sits next to the input; other cards also have one.
  await input.locator("xpath=..").getByRole("button", { name: MAINTENANCE_LABELS.save }).click()
  await expect(input).toHaveValue(dir)
}

/** Ask the stack to scan the registered folders again. */
export async function rescanGallery(page: Page): Promise<void> {
  await openAdminGeneral(page)
  await page.getByRole("button", { name: MAINTENANCE_LABELS.rescanAll }).first().click()
}

/** Register a gallery folder through the admin settings dialog. */
export async function addGalleryFolder(page: Page, dir: string): Promise<void> {
  await openAdminGeneral(page)
  await page.getByRole("button", { name: MAINTENANCE_LABELS.addFolder }).first().click()
  const dialog = page.getByRole("dialog").first()
  await dialog.locator("input").first().fill(dir)
  await dialog.getByRole("button", { name: MAINTENANCE_LABELS.addFolder }).click()
  await expect(page.getByText(dir, { exact: true })).toBeVisible()
}

/** Drop a gallery folder registration through the admin settings list. */
export async function removeGalleryFolder(page: Page, dir: string): Promise<void> {
  await openAdminGeneral(page)
  const card = page
    .getByText(dir, { exact: true })
    .locator("xpath=ancestor::div[contains(@class,'rounded')][1]")
  await card.locator('[data-slot="folder-remove"]').click()
  const dialog = page.getByRole("dialog").first()
  await dialog.locator("button.bg-destructive").click()
  await expect(page.getByText(dir, { exact: true })).toHaveCount(0)
}

/** Delete one photo from the all-images grid, through the confirm dialog. */
export async function deletePhotoThroughGallery(page: Page, fileName: string): Promise<void> {
  await openNav(page, "gallery-all-images")
  const tile = galleryTile(page, fileName)
  await expect(tile).toBeVisible()
  await tile.hover()
  await tile.locator('[data-slot="tile-delete"]').click()
  const dialog = page.getByRole("dialog").first()
  await expect(dialog).toBeVisible()
  await dialog.locator("button.bg-destructive").click()
  await expect(tile).toHaveCount(0)
}

/** Restore one trashed file from its tile in the trash tab. */
export async function restoreTrashedPhoto(page: Page, fileName: string): Promise<void> {
  await openNav(page, "gallery-trash")
  const tile = galleryTile(page, fileName)
  await expect(tile).toBeVisible()
  await tile.hover()
  await tile.getByRole("button", { name: MAINTENANCE_LABELS.restore }).click()
  await expect(tile).toHaveCount(0)
}

/** Empty the whole trash through the trash tab and its confirmation. */
export async function emptyTrash(page: Page): Promise<void> {
  await openNav(page, "gallery-trash")
  await page.getByRole("button", { name: MAINTENANCE_LABELS.emptyTrash }).first().click()
  const dialog = page.getByRole("dialog").first()
  await expect(dialog).toBeVisible()
  await dialog.locator("button.bg-destructive").click()
  // The list refreshes on its own; the caller asserts the resulting emptiness.
  await expect(dialog).toHaveCount(0)
}

/**
 * Walk the batch dedup wizard: keep `keepDir` for the pattern that offers it,
 * skip every other pattern, then apply the collected rules.
 */
export async function applyBatchDedup(page: Page, keepDir: string): Promise<void> {
  await openNav(page, "deduplication")
  await page.getByRole("button", { name: MAINTENANCE_LABELS.batchDedup }).first().click()
  const dialog = page.getByRole("dialog").first()
  await expect(dialog).toBeVisible()

  const apply = dialog.getByRole("button", { name: MAINTENANCE_LABELS.applyRules })
  for (let step = 0; step < 25; step += 1) {
    // Select the folder to keep whenever the current pattern offers it — the
    // last pattern shows Apply right away, so this has to happen first.
    const keep = dialog.getByRole("radio", { name: keepDir })
    if (await keep.isVisible().catch(() => false)) {
      await keep.click()
    }
    if (await apply.isVisible().catch(() => false)) break

    const skip = dialog.getByRole("button", { name: MAINTENANCE_LABELS.skipThis })
    if (await skip.isVisible().catch(() => false)) {
      await skip.click()
      continue
    }
    await dialog.getByRole("button", { name: MAINTENANCE_LABELS.forward }).click()
  }

  await expect(apply).toBeVisible()
  await apply.click()

  // The wizard asks for confirmation before it deletes anything.
  const confirmApply = page.getByRole("dialog").last().getByRole("button", { name: MAINTENANCE_LABELS.applyRules })
  await expect(confirmApply).toBeVisible()
  await confirmApply.click()

  // The result replaces the wizard footer, leaving only the close action; the
  // wizard has to be dismissed because its modal hides the workspace.
  await expect(apply).toHaveCount(0)
  await dialog.getByRole("button", { name: MAINTENANCE_LABELS.close }).last().click()
  await expect(page.getByRole("dialog")).toHaveCount(0)
}

/** Sidebar destinations, keyed by the tab value the app routes on. */
export type NavKey =
  | "gallery-all-images"
  | "gallery-folders"
  | "gallery-calendar"
  | "gallery-geolocation"
  | "smart-search"
  | "deduplication"
  | "ocr"
  | "exif"
  | "people"
  | "gallery-trash"
  | "admin-users"
  | "admin-settings"
  | "about"

interface NavEntry {
  label: RegExp
  /** Sidebar group that has to be expanded for the entry to exist. */
  group?: RegExp
}

const GALLERY_GROUP = /^(Gallery|Галерея)$/
const TOOLS_GROUP = /^(Tools|Инструменты)$/
const ADMIN_GROUP = /^(Administration|Администрирование)$/

export const NAV: Record<NavKey, NavEntry> = {
  "gallery-all-images": { label: /^(All Images|Все изображения)$/, group: GALLERY_GROUP },
  "gallery-folders": { label: /^(Folders|Папки)$/, group: GALLERY_GROUP },
  "gallery-calendar": { label: /^(Calendar|Календарь)$/, group: GALLERY_GROUP },
  "gallery-geolocation": { label: /^(Map|Карта)$/, group: GALLERY_GROUP },
  "smart-search": { label: /^(Smart Search|Умный поиск)$/, group: TOOLS_GROUP },
  deduplication: { label: /^(Deduplication|Дедупликация)$/, group: TOOLS_GROUP },
  ocr: { label: /^(Text Recognition|Распознавание текста)$/, group: TOOLS_GROUP },
  exif: { label: /^(Metadata|Метаданные)$/, group: TOOLS_GROUP },
  people: { label: /^(People|Люди)$/ },
  "gallery-trash": { label: /^(Trash|Корзина)$/ },
  "admin-users": { label: /^(Users|Пользователи)$/, group: ADMIN_GROUP },
  "admin-settings": { label: /^(Settings|Настройки)$/, group: ADMIN_GROUP },
  about: { label: /^(About|О программе)$/ },
}

/** The sidebar element (the desktop panel and the mobile drawer share it). */
export function sidebar(page: Page): Locator {
  return page.locator("aside").first()
}

/** The sidebar button for one destination, whether or not it is expanded. */
export function navButton(page: Page, key: NavKey): Locator {
  return sidebar(page).getByRole("button", { name: NAV[key].label }).first()
}

/** True when the deployment shows this destination for the signed-in account. */
export async function hasNavEntry(page: Page, key: NavKey): Promise<boolean> {
  const entry = NAV[key]
  if (entry.group) {
    const group = sidebar(page).getByRole("button", { name: entry.group }).first()
    if (!(await group.isVisible().catch(() => false))) return false
    // Expand the group when the entry is not rendered yet. A group that is
    // already expanded is left alone, so the nav state is not toggled blindly.
    if (!(await navButton(page, key).isVisible().catch(() => false))) {
      await group.click()
    }
  }
  return navButton(page, key).isVisible().catch(() => false)
}

/**
 * Open a sidebar destination. Groups start expanded, so the common path is one
 * click; a collapsed group is expanded first.
 */
export async function openNav(page: Page, key: NavKey): Promise<void> {
  const entry = NAV[key]
  const button = navButton(page, key)
  if (!(await button.isVisible().catch(() => false)) && entry.group) {
    const group = sidebar(page).getByRole("button", { name: entry.group }).first()
    await group.click()
  }
  await expect(button).toBeVisible()
  await button.click()
}

/** Wait until the workspace shell is up: the sidebar is what proves a session. */
export async function expectWorkspace(page: Page): Promise<void> {
  await expect(navButton(page, "people")).toBeVisible()
  await expect(navButton(page, "gallery-trash")).toBeVisible()
}

/** Fill and submit the login form. */
export async function signIn(page: Page, login: string, password: string): Promise<void> {
  await page.getByRole("textbox", { name: AUTH_LABELS.login }).fill(login)
  await page.getByRole("textbox", { name: AUTH_LABELS.password }).fill(password)
  await page.getByRole("button", { name: AUTH_LABELS.signIn }).click()
}

/** Wait until the login form is on screen and no session is active. */
export async function expectLoginScreen(page: Page): Promise<void> {
  await expect(page.getByRole("textbox", { name: AUTH_LABELS.login })).toBeVisible()
  await expect(page.getByRole("button", { name: AUTH_LABELS.signIn })).toBeVisible()
}

/**
 * Sign the current session out and land back on the login form. The header
 * keeps the logout button at every width (icon only below `md`), so the
 * accessible name is enough to reach it.
 */
export async function signOut(page: Page): Promise<void> {
  await page.getByRole("button", { name: AUTH_LABELS.signOut }).first().click()
  await expectLoginScreen(page)
}

// --- People catalog ---

/** Open the workspace People tab and wait for the catalog grid. */
export async function openPeopleTab(page: Page): Promise<void> {
  await openNav(page, "people")
  await expect(page.locator('[data-slot="people-search"]')).toBeVisible()
}

/** The clickable tile button of one person (carries the display name). */
export function personTile(page: Page, name: string): Locator {
  return page.locator('[data-slot="person-tile"]').filter({ hasText: name })
}

/** The tile wrapper holding both the open button and the tile actions. */
export function personCard(page: Page, name: string): Locator {
  return personTile(page, name).locator("xpath=..")
}

/** Select the person with the tile checkbox. */
export async function selectPerson(page: Page, name: string): Promise<void> {
  await personCard(page, name).locator('[data-slot="person-select"]').click()
  await expect(personCard(page, name).locator('[data-slot="person-select"]')).toHaveAttribute(
    "aria-pressed",
    "true",
  )
}

/** Search the grid by name. */
export async function searchPeople(page: Page, text: string): Promise<void> {
  await page.locator('[data-slot="people-search"]').fill(text)
}

/**
 * Make sure the lightbox people side panel is open. It is collapsible below
 * `md`; on wide viewports the content is already rendered.
 */
export async function openLightboxPeopleList(page: Page): Promise<void> {
  const firstEntry = page.getByTestId("people-list-item").first()
  if (!(await firstEntry.isVisible().catch(() => false))) {
    await page.getByTestId("mobile-detail-toggle").click()
  }
  await expect(firstEntry).toBeVisible()
}
