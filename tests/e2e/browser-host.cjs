#!/usr/bin/env node
/**
 * Browser host for tunneled Playwright runs — run this on your own machine.
 *
 * The test runner (code-server, CI) uses `PW_TUNNEL=1` and waits for a browser
 * to connect on port `E2E_TUNNEL_PORT` (default 56767). Forward that port from
 * this machine to the runner host, then start this script:
 *
 *   npm install --no-save playwright @rushstack/playwright-browser-tunnel @rushstack/terminal
 *   npx playwright install chromium
 *   E2E_TUNNEL_ENDPOINT=ws://localhost:56767 node tests/e2e/browser-host.cjs
 *
 * The `playwright` version here must be compatible with the runner's
 * `@playwright/test` version.
 */
const path = require("node:path")
const { PlaywrightTunnel } = require("@rushstack/playwright-browser-tunnel")
const { ConsoleTerminalProvider, Terminal } = require("@rushstack/terminal")

const endpoint = process.env.E2E_TUNNEL_ENDPOINT ?? "ws://localhost:56767"
const terminal = new Terminal(new ConsoleTerminalProvider({ verboseEnabled: true }))

const tunnel = new PlaywrightTunnel({
  terminal,
  mode: "poll-connection",
  wsEndpoint: endpoint,
  playwrightInstallPath: path.dirname(require.resolve("playwright-core/package.json")),
  onStatusChange: (status) => console.log(`[browser-host] ${status}`),
})

async function main() {
  await tunnel.startAsync({ keepRunning: true })
  console.log(`[browser-host] waiting for the test run on ${endpoint}`)
  await tunnel.waitForCloseAsync()
  await tunnel.stopAsync()
}

main().catch((error) => {
  console.error("[browser-host] failed:", error)
  process.exit(1)
})
