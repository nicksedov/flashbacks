import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { PersonNameConflictDialog, RenamePersonDialog } from "./PeopleDialogs"
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

function person(id: number, name: string, photoCount = 3): PersonDTO {
  return { id, name, photoCount, coverThumbnail: "" }
}

function renderRenameDialog(
  overrides: Partial<{
    people: PersonDTO[]
    language: "en" | "ru"
    initialName: string
    excludePersonId: number
  }> = {},
) {
  const onSubmit = vi.fn()
  const onCancel = vi.fn()

  render(
    <I18nProvider language={overrides.language ?? "en"}>
      <RenamePersonDialog
        initialName={overrides.initialName ?? ""}
        people={overrides.people ?? []}
        excludePersonId={overrides.excludePersonId}
        onCancel={onCancel}
        onSubmit={onSubmit}
      />
    </I18nProvider>,
  )

  return { onSubmit, onCancel }
}

function suggestionChips(): HTMLElement[] {
  return Array.from(
    document.querySelectorAll('[data-slot="rename-suggestions"] [data-slot="suggestion-chip"]'),
  )
}

describe("RenamePersonDialog", () => {
  // Names run opposite to the photo counts, so a list ordered by name instead of
  // by count cannot pass the opening assertions.
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

  it("opens with the seven most-photographed names", () => {
    renderRenameDialog({ people: catalog })

    const chips = suggestionChips()
    expect(chips).toHaveLength(7)
    expect(chips[0].textContent).toBe("Зоя")
    expect(chips[6].textContent).toBe("Борис")
  })

  it("shows the top names when the field already holds a name", () => {
    renderRenameDialog({ people: catalog, initialName: "Анна" })

    const chips = suggestionChips()
    expect(chips).toHaveLength(7)
    expect(chips[0].textContent).toBe("Зоя")
  })

  it("narrows the chips once the field is edited", () => {
    renderRenameDialog({ people: catalog })

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "жанна" } })

    const chips = suggestionChips()
    expect(chips).toHaveLength(1)
    expect(chips[0].textContent).toBe("Жанна")
  })

  it("hides the chips when the edited fragment matches nothing", () => {
    renderRenameDialog({ people: catalog })

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "zzz" } })

    expect(document.querySelector('[data-slot="rename-suggestions"]')).toBeNull()
  })

  it("fills the name from a suggestion without submitting", () => {
    const { onSubmit } = renderRenameDialog({
      people: [person(1, "Иван", 12), person(2, "Анна", 4)],
    })

    const chips = suggestionChips()
    expect(chips).toHaveLength(2)
    fireEvent.click(chips[0])

    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Иван")
    expect(screen.getByRole("dialog")).toBeTruthy()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("shows only the name on each suggestion chip", () => {
    renderRenameDialog({ people: [person(1, "Иван", 12)] })

    const chip = suggestionChips()[0]
    expect(chip.textContent).toBe("Иван")
    const dialog = screen.getByRole("dialog")
    expect(dialog.textContent).not.toContain("12 photos")
    expect(dialog.textContent).not.toContain("Suggestions")
  })

  it("renders no suggestion list when nothing can be offered", () => {
    renderRenameDialog({ people: [person(1, "", 5)] })

    expect(document.querySelector('[data-slot="rename-suggestions"]')).toBeNull()
  })
})
