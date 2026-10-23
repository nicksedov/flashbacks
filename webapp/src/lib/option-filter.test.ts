import { describe, expect, it } from "vitest"
import { filterOptionsByQuery } from "./option-filter"

const options = [
  { value: "minicpm-v", label: "MiniCPM-V (5.5 GB)" },
  { value: "llama3", label: "llama3 (4.7 GB)" },
  { value: "qwen3-embedding:4b", label: "qwen3-embedding:4b" },
]

describe("filterOptionsByQuery", () => {
  it("returns every option for an empty query", () => {
    expect(filterOptionsByQuery(options, "")).toEqual(options)
  })

  it("returns every option for a whitespace-only query", () => {
    expect(filterOptionsByQuery(options, "   ")).toEqual(options)
  })

  it("keeps options whose label contains the query as a substring", () => {
    expect(filterOptionsByQuery(options, "llama")).toEqual([options[1]])
  })

  it("ignores letter case on both sides", () => {
    expect(filterOptionsByQuery(options, "MINI")).toEqual([options[0]])
    expect(filterOptionsByQuery(options, "Minicpm-V")).toEqual([options[0]])
  })

  it("trims the query before matching", () => {
    expect(filterOptionsByQuery(options, "  embed  ")).toEqual([options[2]])
  })

  it("preserves the input order of the matches", () => {
    expect(filterOptionsByQuery(options, "gb")).toEqual([options[0], options[1]])
  })

  it("returns no options when nothing matches", () => {
    expect(filterOptionsByQuery(options, "does-not-exist")).toEqual([])
  })
})
