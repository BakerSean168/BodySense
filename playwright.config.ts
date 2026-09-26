import { defineConfig, devices } from "@playwright/test";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const webBase = process.env.E2E_BASE_URL || "http://127.0.0.1:5173";
const webUrl = new URL(webBase);
const webHost = webUrl.hostname;
const webPort = webUrl.port || (webUrl.protocol === "https:" ? "443" : "80");

export default defineConfig({
  testDir: "./apps/web/e2e",
  timeout: 30_000,
  expect: { timeout: 8_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: webBase,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    // Start Vite directly under Playwright's ownership. Wrapping it in
    // `pnpm exec` can orphan the Vite child process during teardown, leaving
    // Playwright waiting indefinitely after the browser tests have completed.
    command: `VITE_DEV_API_TARGET=${apiBase} node ./node_modules/vite/bin/vite.js --config apps/web/vite.config.ts --host ${webHost} --port ${webPort}`,
    url: webBase,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
