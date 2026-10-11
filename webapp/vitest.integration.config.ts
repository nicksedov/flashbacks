import path from "path"
import { defineConfig } from "vitest/config"

// HTTP integration tests live in the repository-level `tests/webapp/` directory
// so that every cross-service integration suite (webapp HTTP, OCR HTTP) is kept
// together under `tests/`.
//
// They boot the real `core` service against the dedicated test database, so they
// kept out of the default `vitest.config.ts` unit-test suite and run explicitly
// via `npm run test:integration`.
//
// This config file stays inside `webapp/` because that is where the `vitest` and
// `pg` packages are installed. The test files live above the package, so `root`
// is anchored at the repository root and `pg` is aliased to the copy in
// `webapp/node_modules`; the bare `vitest` import is resolved by Vitest itself.

const REPO_ROOT = path.resolve(__dirname, "..")

export default defineConfig({
  resolve: {
    alias: {
      pg: path.resolve(__dirname, "node_modules", "pg"),
    },
  },
  test: {
    root: REPO_ROOT,
    include: ["tests/webapp/integration/**/*.test.ts"],
    environment: "node",
    // A single shared service/database is exercised, so do not parallelize files.
    fileParallelism: false,
    testTimeout: 30_000,
    // Building the Go binary and waiting for the service to become ready is slow.
    hookTimeout: 180_000,
  },
})
