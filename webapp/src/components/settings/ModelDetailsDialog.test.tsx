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
  inputModalities: ["text", "image", "video"],
  outputModalities: ["text"],
  inputPricePerMTok: 2,
  outputPricePerMTok: 6,
}

describe("ModelDetailsDialog", () => {
  it("shows context length, input modalities, output modalities, capabilities and pricing", () => {
    render(<ModelDetailsDialog model={model} open onOpenChange={vi.fn()} />)

    expect(screen.getByText("Qwen3.8-Max")).toBeTruthy()
    expect(screen.getByText("1.0M")).toBeTruthy()
    expect(screen.getByText("text, image, video")).toBeTruthy()
    expect(screen.getByText("text")).toBeTruthy()
    // Capabilities render their localized label key (the test i18n returns keys).
    expect(screen.getByText("llm_capabilities.vision")).toBeTruthy()
    expect(screen.getByText("$2.00")).toBeTruthy()
    expect(screen.getByText("$6.00")).toBeTruthy()
  })

  it("labels a new capability", () => {
    render(
      <ModelDetailsDialog
        model={{ ...model, capabilities: ["chat", "video_generation"] }}
        open
        onOpenChange={vi.fn()}
      />,
    )

    // A standardized capability added after the original vocabulary is shown
    // with its localized label, never the raw value.
    expect(screen.getByText("llm_capabilities.videoGeneration")).toBeTruthy()
    expect(screen.queryByText("video_generation")).toBeNull()
  })
})
