import { defineConfig, devices } from "@playwright/test";

import base from "./playwright.config";

// QA-001 cross-domain matrix: the full Chromium suite at the Penpot desktop
// frame (1440x900) and at the 390px mobile width. Specs that pin their own
// viewport keep it; everything else runs at the project viewport.
export default defineConfig({
  ...base,
  projects: [
    {
      name: "chromium-1440",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
    {
      name: "chromium-390",
      use: { ...devices["Desktop Chrome"], viewport: { width: 390, height: 844 } },
    },
  ],
});
