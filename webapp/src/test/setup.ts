import { cleanup } from "@testing-library/react"
import { afterEach } from "vitest"

// Unmount React trees rendered by @testing-library/react between tests.
afterEach(() => {
  cleanup()
})
