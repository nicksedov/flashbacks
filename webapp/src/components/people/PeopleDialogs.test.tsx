import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { PersonNameConflictDialog } from "./PeopleDialogs"
import type { PersonDTO } from "@/types"

const holder: PersonDTO = { id: 3, name: "Иван", photoCount: 12, coverThumbnail: "" }

function renderDialog(
  overrides: Partial<{
    mode: "merge-person" | "merge-selection" | "attach-faces"
    language: "en" | "ru"
  }> = {},
) {
  const onMerge = vi.fn()
  const onChangeName = vi.fn()
  const onCancel = vi.fn()

  render(
    <I18nProvider language={overrides.language ?? "en"}>
      <PersonNameConflictDialog
        pendingName="иван"
        conflict={holder}
        mode={overrides.mode ?? "merge-person"}
        onMerge={onMerge}
        onChangeName={onChangeName}
        onCancel={onCancel}
      />
    </I18nProvider>,
  )

  return { onMerge, onChangeName, onCancel }
}

function conflictDialog(): HTMLElement {
  return screen.getByRole("dialog")
}

describe("PersonNameConflictDialog", () => {
  it("names the person that holds the name and shows its photo count", () => {
    renderDialog()

    const dialog = conflictDialog()
    expect(dialog.textContent).toContain("Иван")
    expect(dialog.textContent).toContain("12 photos")
    expect(dialog.textContent).toContain("Name already taken")
  })

  it("offers exactly the merge and the new-name alternatives", () => {
    renderDialog()

    expect(document.querySelector('[data-slot="name-conflict-merge"]')).toBeTruthy()
    expect(document.querySelector('[data-slot="name-conflict-rename"]')).toBeTruthy()
    expect(document.querySelector('[data-slot="name-conflict-cancel"]')).toBeTruthy()
    // The name input lives in the dialog this one replaced.
    expect(document.querySelector('[data-slot="rename-input"]')).toBeNull()
  })

  it("calls the merge action", () => {
    const { onMerge, onChangeName, onCancel } = renderDialog()

    fireEvent.click(document.querySelector('[data-slot="name-conflict-merge"]') as HTMLElement)

    expect(onMerge).toHaveBeenCalledTimes(1)
    expect(onChangeName).not.toHaveBeenCalled()
    expect(onCancel).not.toHaveBeenCalled()
  })

  it("calls the change-name action", () => {
    const { onMerge, onChangeName } = renderDialog()

    fireEvent.click(document.querySelector('[data-slot="name-conflict-rename"]') as HTMLElement)

    expect(onChangeName).toHaveBeenCalledTimes(1)
    expect(onMerge).not.toHaveBeenCalled()
  })

  it("calls the cancel action", () => {
    const { onMerge, onCancel } = renderDialog()

    fireEvent.click(document.querySelector('[data-slot="name-conflict-cancel"]') as HTMLElement)

    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onMerge).not.toHaveBeenCalled()
  })

  it("words the merge action for an attached face selection", () => {
    renderDialog({ mode: "attach-faces" })

    expect(conflictDialog().textContent).toContain("Add the faces to")
  })

  it("words the merge action for a selection", () => {
    renderDialog({ mode: "merge-selection" })

    expect(conflictDialog().textContent).toContain("Merge everything into")
  })

  it("is localized in Russian", () => {
    renderDialog({ language: "ru" })

    const dialog = conflictDialog()
    expect(dialog.textContent).toContain("Имя уже занято")
    expect(dialog.textContent).toContain("Ввести другое имя")
    expect(dialog.textContent).toContain("12 фото")
    expect(dialog.textContent).not.toContain("people.nameConflict")
  })
})
