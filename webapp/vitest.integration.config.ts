import { defineConfig } from "vitest/config"

// Integration tests boot the real `core` service against the staging database,
// so they are kept out of the default `vitest.config.ts` unit-test suite and run
// explicitly via `npm run test:integration`.
export default defineConfig({
  test: {
    include: ["integration/**/*.test.ts"],
    environment: "node",
    // A single shared service/database is exercised, so do not parallelize files.
    fileParallelism: false,
    testTimeout: 30_000,
    // Building the Go binary and waiting for the service to become ready is slow.
    hookTimeout: 180_000,
  },
})
