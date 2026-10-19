import { expect, test, type Page } from "@playwright/test";

// A dropped request in the entry module graph (a network change on the
// device, or a deploy that replaced the chunks) must not leave a blank page:
// index.html reloads once, and only once, so a real outage cannot loop.

const graphModule = /\/src\/generated\/api\/model\/[^/]+\.zod\.ts/;

function countDocumentLoads(page: Page) {
  const loads = { count: 0 };
  page.on("request", (request) => {
    if (request.isNavigationRequest() && request.frame() === page.mainFrame()) {
      loads.count += 1;
    }
  });
  return loads;
}

test("a dropped entry module request reloads once and the app mounts", async ({
  page,
}) => {
  let dropped = 0;
  await page.route(graphModule, async (route) => {
    if (dropped === 0) {
      dropped += 1;
      await route.abort("internetdisconnected");
      return;
    }
    await route.fallback();
  });
  const loads = countDocumentLoads(page);
  await page.goto("/register");
  await expect(page.getByLabel("邮箱地址")).toBeVisible({ timeout: 30_000 });
  expect(dropped).toBe(1);
  expect(loads.count).toBe(2);
});

test("a persistent boot failure reloads at most once", async ({ page }) => {
  let dropped = 0;
  await page.route(graphModule, async (route) => {
    dropped += 1;
    await route.abort("internetdisconnected");
  });
  const loads = countDocumentLoads(page);
  await page.goto("/register");
  await expect.poll(() => loads.count, { timeout: 20_000 }).toBe(2);
  await page.waitForTimeout(3_000);
  expect(loads.count).toBe(2);
  expect(dropped).toBeGreaterThanOrEqual(2);
  await expect(page.getByLabel("邮箱地址")).toHaveCount(0);
});
