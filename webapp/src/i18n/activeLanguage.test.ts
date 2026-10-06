import { createElement } from "react"
import { renderToStaticMarkup } from "react-dom/server"
import { describe, expect, it } from "vitest"
import { I18nProvider } from "./I18nProvider"
import { selectActiveLanguage } from "./language"
import { useTranslation } from "./useTranslation"
import type { Language } from "./types"

// The webapp has no DOM test environment or component-testing library, so this
// render test mirrors the exact expression SettingsProvider feeds to I18nProvider
// (`selectActiveLanguage(isAuthenticated, language, authLanguage)`) and renders it
// server-side to assert the chosen language actually reaches the translated output.
function Probe() {
  const { t } = useTranslation()
  return createElement("span", null, t("settings.language"))
}

function renderActiveLanguage(
  isAuthenticated: boolean,
  accountLanguage: Language,
  authLanguage: Language
): string {
  const language = selectActiveLanguage(isAuthenticated, accountLanguage, authLanguage)
  return renderToStaticMarkup(
    createElement(I18nProvider, { language, children: createElement(Probe) })
  )
}

describe("active language selection drives rendering", () => {
  it("renders the account language for an authenticated tree", () => {
    expect(renderActiveLanguage(true, "ru", "en")).toContain("Язык")
  })

  it("renders the pre-authentication language for an unauthenticated tree", () => {
    expect(renderActiveLanguage(false, "ru", "en")).toContain("Language")
  })
})
