import { createElement } from "react"
import { renderToStaticMarkup } from "react-dom/server"
import { describe, expect, it } from "vitest"
import { ThemeProvider } from "./ThemeProvider"
import { DEFAULT_THEME, selectActiveTheme } from "./theme"
import { useTheme } from "./useTheme"
import type { Theme } from "./context"

describe("selectActiveTheme", () => {
  it("uses the account theme when authenticated", () => {
    expect(selectActiveTheme(true, "dark-blue")).toBe("dark-blue")
    expect(selectActiveTheme(true, "light-green")).toBe("light-green")
  })

  it("falls back to the default theme when unauthenticated", () => {
    expect(selectActiveTheme(false, "dark-blue")).toBe(DEFAULT_THEME)
    expect(selectActiveTheme(false, "dark-contrast")).toBe(DEFAULT_THEME)
  })

  it("keeps the default theme when no account theme differs", () => {
    expect(selectActiveTheme(false, DEFAULT_THEME)).toBe(DEFAULT_THEME)
  })
})

// The webapp has no DOM test environment or component-testing library, so this
// render test mirrors the exact expression SettingsProvider feeds to ThemeProvider
// (`selectActiveTheme(isAuthenticated, theme)`) and renders it server-side to assert
// the chosen theme actually reaches the theme context consumers.
function Probe() {
  const { theme } = useTheme()
  return createElement("span", null, theme)
}

function renderActiveTheme(isAuthenticated: boolean, accountTheme: Theme): string {
  const theme = selectActiveTheme(isAuthenticated, accountTheme)
  return renderToStaticMarkup(
    createElement(ThemeProvider, { theme, children: createElement(Probe) })
  )
}

describe("active theme selection drives rendering", () => {
  it("renders the account theme for an authenticated tree", () => {
    expect(renderActiveTheme(true, "dark-blue")).toContain("dark-blue")
  })

  it("renders the default theme for an unauthenticated tree", () => {
    expect(renderActiveTheme(false, "dark-blue")).toContain(DEFAULT_THEME)
  })
})
