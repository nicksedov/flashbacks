import { useState } from "react"
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { LoginScreen } from "./LoginScreen"
import { I18nProvider } from "@/i18n/I18nProvider"
import { AuthContext, type AuthContextType } from "@/providers/authContext"
import { SettingsContext, type SettingsContextValue } from "@/providers/settingsContext"
import type { Language } from "@/i18n"
import type { AccountCreationMode } from "@/types"

vi.mock("@/api/endpoints", () => ({
  fetchAuthStatus: vi.fn().mockResolvedValue({
    isAuthenticated: false,
    isBootstrapMode: false,
    accountCreationMode: "self_service",
  }),
  login: vi.fn(),
  register: vi.fn(),
}))

vi.mock("sonner", () => ({
  toast: { info: vi.fn(), success: vi.fn(), error: vi.fn() },
}))

const defaultAuth: AuthContextType = {
  user: null,
  isAuthenticated: false,
  isBootstrapMode: false,
  isBootstrapVerified: false,
  accountCreationMode: "self_service",
  isLoading: false,
  login: vi.fn(),
  logout: vi.fn(),
  updateUser: vi.fn(),
  setBootstrapVerified: vi.fn(),
}

function renderLogin(
  initialLanguage: Language = "en",
  accountCreationMode: AccountCreationMode = "self_service"
) {
  function Harness() {
    const [authLanguage, setAuthLanguage] = useState<Language>(initialLanguage)
    const auth: AuthContextType = { ...defaultAuth, accountCreationMode }
    const settings: SettingsContextValue = {
      theme: "light-purple",
      setTheme: vi.fn(),
      toggleTheme: vi.fn(),
      language: "en",
      setLanguage: vi.fn(),
      authLanguage,
      setAuthLanguage,
      trashDir: "",
      setTrashDir: vi.fn(),
      isLoading: false,
    }

    return (
      <I18nProvider language={authLanguage}>
        <AuthContext.Provider value={auth}>
          <SettingsContext.Provider value={settings}>
            <LoginScreen />
          </SettingsContext.Provider>
        </AuthContext.Provider>
      </I18nProvider>
    )
  }

  return render(<Harness />)
}

/** Open the Radix select and pick an option by its visible endonym. */
async function chooseLanguage(optionName: string) {
  const trigger = screen.getByRole("combobox", { name: /Language|Язык/ })
  fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: "mouse" })
  fireEvent.click(trigger)

  const option = await screen.findByRole("option", { name: optionName })
  fireEvent.click(option)
}

describe("LoginScreen language switcher", () => {
  beforeEach(() => {
    // Radix Select relies on pointer capture and ResizeObserver APIs that jsdom lacks.
    Element.prototype.hasPointerCapture = vi.fn(() => false)
    Element.prototype.releasePointerCapture = vi.fn()
    Element.prototype.scrollIntoView = vi.fn()
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      }
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("shows a language switcher on the login screen", async () => {
    renderLogin("en")

    const trigger = await screen.findByRole("combobox", { name: "Language" })
    expect(trigger).toBeTruthy()
    expect(trigger.textContent).toContain("English")
  })

  it("keeps the switcher available in registration mode and reflecting the choice", async () => {
    renderLogin("ru")

    fireEvent.click(screen.getByRole("button", { name: "Создать учётную запись" }))

    const trigger = await screen.findByRole("combobox", { name: "Язык" })
    expect(trigger.textContent).toContain("Русский")
    expect(screen.getByLabelText("Логин")).toBeTruthy()
    // The same branded logo stays at the top in registration mode.
    expect(screen.getByAltText("Flashbacks").getAttribute("src")).toBe(
      "/flashbacks_logo_welcomescreen.png"
    )
  })

  it("re-renders the screen in the selected language without a reload", async () => {
    renderLogin("en")

    await screen.findByRole("combobox", { name: "Language" })
    expect(screen.getByText("Enter your credentials to log in")).toBeTruthy()

    await chooseLanguage("Русский")

    expect(screen.getByText("Введите свои учетные данные для входа")).toBeTruthy()
  })

  it("exposes an accessible name and is keyboard operable", async () => {
    renderLogin("en")

    const trigger = await screen.findByRole("combobox", { name: "Language" })
    expect(trigger.getAttribute("aria-label")).toBe("Language")
    // Radix triggers are native buttons: focusable and activated from the keyboard.
    expect(trigger.tagName).toBe("BUTTON")
    expect(trigger.hasAttribute("disabled")).toBe(false)

    trigger.focus()
    expect(document.activeElement).toBe(trigger)
  })

  it("shows the branded welcome logo proportionally at the top of the card", async () => {
    renderLogin("en")

    const logo = (await screen.findByAltText("Flashbacks")) as HTMLImageElement
    expect(logo.getAttribute("src")).toBe("/flashbacks_logo_welcomescreen.png")
    // w-72 (288px) on a max-w-md card, width capped by the card and height auto.
    expect(logo.className).toContain("w-72")
    expect(logo.className).toContain("max-w-full")
    expect(logo.className).toContain("h-auto")
  })
})
