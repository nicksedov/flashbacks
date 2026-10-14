import path from "path"
import { defineConfig } from "vitest/config"

// Root-level Vitest config used by the focused-spec `vitest` runner, which
// resolves Vitest from the consuming project root (this repository). It defines
// two projects so a single runner can execute both the webapp component tests
// and the HTTP integration tests:
//
//   - webapp-unit:        jsdom component tests under webapp/src (the webapp's
//                         own `npm test` config lives at webapp/vitest.config.ts;
//                         keep this project in sync with its test options).
//   - webapp-integration: node HTTP integration tests under tests/webapp/
//                         (mirrors webapp/vitest.integration.config.ts so the
//                         same suite can run through the runner or `npm run
//                         test:integration`).
export default defineConfig({
  test: {
    projects: [
      {
        resolve: {
          // The runner invokes the root Vitest; force every `vitest` import
          // (including the jest-dom setup's `expect`) to resolve to a single
          // instance.
          dedupe: ["vitest"],
          alias: {
            "@": path.resolve(__dirname, "webapp/src"),
          },
        },
        test: {
          name: "webapp-unit",
          globals: true,
          environment: "jsdom",
          setupFiles: [path.resolve(__dirname, "webapp/src/test/setup.ts")],
          include: ["webapp/src/**/*.test.ts", "webapp/src/**/*.test.tsx"],
        },
      },
      {
        resolve: {
          // The integration tests live above the webapp package, so `pg` is
          // aliased to the copy installed under webapp/node_modules.
          alias: {
            pg: path.resolve(__dirname, "webapp/node_modules/pg"),
          },
        },
        test: {
          name: "webapp-integration",
          root: __dirname,
          include: ["tests/webapp/integration/**/*.test.ts"],
          environment: "node",
          // A single shared service/database is exercised, so do not
          // parallelize files.
          fileParallelism: false,
          testTimeout: 30_000,
          // Building the Go binary and waiting for the service to become ready
          // is slow.
          hookTimeout: 180_000,
        },
      },
    ],
  },
})
