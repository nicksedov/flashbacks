import { describe, it, expect } from "vitest"
import { filterModelsForInstrument } from "./llm-model-filter"
import type { LlmModelDTO } from "@/types"

const models: LlmModelDTO[] = [
  { id: "chat-1", name: "Chat", capabilities: ["chat", "tool_calling"] },
  { id: "vision-1", name: "Vision", capabilities: ["chat", "tool_calling", "vision"] },
  { id: "embed-1", name: "Embedding", capabilities: ["embedding"] },
  { id: "edit-1", name: "Edit", capabilities: ["chat", "image_edit"] },
  { id: "unknown-1", name: "Unknown" },
]

describe("filterModelsForInstrument", () => {
  it("chat returns only chat-capable models", () => {
    const ids = filterModelsForInstrument(models, "chat").map((m) => m.id)
    expect(ids).toEqual(["chat-1", "vision-1", "edit-1"])
  })

  it("vl returns only vision-capable models", () => {
    const ids = filterModelsForInstrument(models, "vl").map((m) => m.id)
    expect(ids).toEqual(["vision-1"])
  })

  it("ocr returns only vision-capable models", () => {
    const ids = filterModelsForInstrument(models, "ocr").map((m) => m.id)
    expect(ids).toEqual(["vision-1"])
  })

  it("image_edit returns only image-edit-capable models", () => {
    const ids = filterModelsForInstrument(models, "image_edit").map((m) => m.id)
    expect(ids).toEqual(["edit-1"])
  })

  it("embedding returns only embedding-capable models", () => {
    const ids = filterModelsForInstrument(models, "embedding").map((m) => m.id)
    expect(ids).toEqual(["embed-1"])
  })
})
