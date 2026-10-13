import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { ModelDetailsDialog } from "./ModelDetailsDialog"
import type { LlmModelDTO } from "@/types"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const model: LlmModelDTO = {
  id: "qwen3.8-max",
  name: "Qwen3.8-Max",
  contextLength: 1000000,
  capabilities: ["chat", "tool_calling", "vision"],
  modalities: ["text", "image"],
  inputPricePerMTok: 2,
  outputPricePerMTok: 6,
}

describe("ModelDetailsDialog", () => {
  it("shows context length, modalities, capabilities and pricing", () => {
    render(<ModelDetailsDialog model={model} open onOpenChange={vi.fn()} />)

    expect(screen.getByText("Qwen3.8-Max")).toBeTruthy()
    expect(screen.getByText("1.0M")).toBeTruthy()
    expect(screen.getByText("text, image")).toBeTruthy()
    expect(screen.getByText("vision")).toBeTruthy()
    expect(screen.getByText("$2.00")).toBeTruthy()
    expect(screen.getByText("$6.00")).toBeTruthy()
  })
})
