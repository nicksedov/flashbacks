import path from "path"
import { defineConfig } from "vitest/config"

// Root-level Vitest config used by the focused-spec `vitest` runner, which
// resolves Vitest from the consuming project root (this repository). It runs
// the webapp's component tests; the webapp keeps its own `npm test` config at
// webapp/vitest.config.ts. Keep this in sync with that config's test options.
export default defineConfig({
  resolve: {
    // The runner invokes the root Vitest; force every `vitest` import (including
    // the jest-dom setup's `expect`) to resolve to that single instance.
    dedupe: ["vitest"],
    alias: {
      "@": path.resolve(__dirname, "webapp/src"),
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: [path.resolve(__dirname, "webapp/src/test/setup.ts")],
    include: ["webapp/src/**/*.test.ts", "webapp/src/**/*.test.tsx"],
  },
})
