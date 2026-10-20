import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

// Desktop-authored: both scenarios open the in-place anatomy layer panel (L)
// from the desktop Tools rail; the main scenario also checks it at 390px.
test.use({ viewport: { width: 1280, height: 720 } });
const publicAssetOrigin = process.env.E2E_PUBLIC_ASSET_ORIGIN?.replace(
  /\/$/,
  "",
);

test("Home body anatomy layers toggle in place on the reference body", async ({
  page,
  request,
}, testInfo) => {
  // The canonical 35-region vocabulary and all curated Vanatome mappings are
  // exhaustively validated by the fast ontology/mapping contract tests. This
  // browser scenario keeps the real-atlas/WebGL layer checks under
  // software-rendered CI. Cache policy itself is covered by delivery tests.
  test.setTimeout(240_000);
  const email = `body3d-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  const atlasRequests: string[] = [];

  page.on("request", (req) => {
    const url = req.url();
    if (isPinnedAtlasRequest(url)) {
      atlasRequests.push(url);
    }
  });

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: profileReplacement({
      gender: "male",
      birth_date: "1996-08-27",
    }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  // Symptoms enter BodyState only through the V2 records ledger.
  const fact = await request.post(`${apiBase}/api/v1/body-state/records`, {
    headers,
    data: {
      idempotency_key: `e2e-symptom-${crypto.randomUUID()}`,
      expected_revision: 0,
      facts: [
        {
          kind: "symptom",
          statement: "抬高手臂时右肩疼",
          body_region_ids: ["shoulder.right"],
          temporal: { precision: "unknown" },
          details: {},
        },
      ],
    },
  });
  expect(fact.ok(), await fact.text()).toBeTruthy();

  const consoleErrors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });

  const pageErrors: Error[] = [];
  page.on("pageerror", (error) => pageErrors.push(error));

  const coldStart = Date.now();
  await page.goto("/consultation");
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  // Home shows the reference body with its muscles under the translucent skin.
  const scene = page.locator('.bc-scene[data-viewer-state="ready"]');
  await expect(scene).toBeVisible({ timeout: 75_000 });
  await expect(scene).toHaveAttribute("data-muscles", "ready", {
    timeout: 75_000,
  });
  const coldReadyMs = Date.now() - coldStart;
  // The skeletal layer is never fetched until 骨骼 is turned on.
  await expect(scene).toHaveAttribute("data-skeleton", "idle");
  expect(atlasRequests.some((url) => url.includes("skeletal"))).toBe(false);
  // 精细解剖 is no longer a modal: there is no anatomy dialog anywhere.
  await expect(page.locator(".bc-anatomy-dialog")).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath("body-layers-default.png"),
    fullPage: true,
  });

  const tools = page.getByRole("toolbar", { name: "身体画布工具" });
  const layerButton = tools.getByRole("button", { name: "图层 · 精细解剖" });
  const layerPanel = page.getByRole("group", { name: "精细解剖图层" });
  await expect(layerPanel).toHaveCount(0);
  await layerButton.click();
  await expect(layerButton).toHaveAttribute("aria-pressed", "true");
  await expect(layerPanel).toBeVisible();
  const skin = layerPanel.getByRole("button", { name: "皮肤" });
  const muscles = layerPanel.getByRole("button", { name: "肌肉" });
  const skeleton = layerPanel.getByRole("button", { name: "骨骼" });
  // Default view: skin and muscles on, skeleton off.
  await expect(skin).toHaveAttribute("aria-pressed", "true");
  await expect(muscles).toHaveAttribute("aria-pressed", "true");
  await expect(skeleton).toHaveAttribute("aria-pressed", "false");

  await skeleton.click();
  await expect(skeleton).toHaveAttribute("aria-pressed", "true");
  await expect(scene).toHaveAttribute("data-skeleton", "ready", {
    timeout: 75_000,
  });
  await skin.click();
  await expect(skin).toHaveAttribute("aria-pressed", "false");
  await muscles.click();
  await expect(muscles).toHaveAttribute("aria-pressed", "false");
  // The body stays interactive in place: the scene is not replaced.
  await expect(scene).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("body-layers-skeleton-only.png"),
    fullPage: true,
  });

  // Closing the panel resets to the default view; L reopens it.
  await layerPanel.getByRole("button", { name: "关闭图层" }).click();
  await expect(layerPanel).toHaveCount(0);
  await expect(layerButton).toHaveAttribute("aria-pressed", "false");
  await page.evaluate(() =>
    (document.activeElement as HTMLElement | null)?.blur(),
  );
  await page.keyboard.press("l");
  await expect(layerPanel).toBeVisible();
  await expect(skin).toHaveAttribute("aria-pressed", "true");
  await expect(muscles).toHaveAttribute("aria-pressed", "true");
  await expect(skeleton).toHaveAttribute("aria-pressed", "false");
  await page.keyboard.press("Escape");
  await expect(layerPanel).toHaveCount(0);

  // Business windows still open over the body while anatomy is not a modal.
  await page.keyboard.press("l");
  await expect(layerPanel).toBeVisible();
  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "综合诊断", exact: true })
    .click();
  await expect(page.getByTestId("diagnosis-window")).toBeVisible();
  await page
    .locator('.bc-business-window[data-window="diagnosis"]')
    .getByRole("button", { name: "关闭综合诊断" })
    .click();
  await expect(page.getByTestId("diagnosis-window")).toBeHidden();

  expect(atlasRequests.length).toBeGreaterThan(0);
  for (const url of atlasRequests) {
    const parsed = new URL(url);
    expect(parsed.search).toBe("");
    expect(url).not.toContain(email);
    expect(url).not.toContain("shoulder.right");
  }
  if (publicAssetOrigin) {
    // The atlas comes from the static CDN; the Z-Anatomy muscles are served
    // same-origin from the Web image.
    for (const url of atlasRequests) {
      if (url.includes("/anatomy/z-anatomy-muscles-")) continue;
      expect(new URL(url).origin).toBe(publicAssetOrigin);
    }
  }

  const resourceSummary = await page.evaluate(() => {
    const entries = performance
      .getEntriesByType("resource")
      .filter(
        (entry) =>
          entry.name.includes("/anatomy/vanatome/1.4.0/") ||
          entry.name.includes("/anatomy/z-anatomy-muscles-"),
      ) as PerformanceResourceTiming[];
    return {
      count: entries.length,
      transferSize: entries.reduce((sum, entry) => sum + entry.transferSize, 0),
      encodedBodySize: entries.reduce(
        (sum, entry) => sum + entry.encodedBodySize,
        0,
      ),
      maxDuration: Math.max(0, ...entries.map((entry) => entry.duration)),
    };
  });
  testInfo.annotations.push({
    type: "body3d-performance",
    description: JSON.stringify({ coldReadyMs, ...resourceSummary }),
  });

  // 390: the layer panel stays usable on the phone layout.
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  if (!(await layerPanel.isVisible())) {
    await page.keyboard.press("l");
  }
  await expect(layerPanel).toBeVisible();
  const panelBox = (await layerPanel.boundingBox())!;
  expect(panelBox.x).toBeGreaterThanOrEqual(0);
  expect(panelBox.x + panelBox.width).toBeLessThanOrEqual(390 + 1);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("body-layers-mobile.png"),
    fullPage: true,
  });
  expect(pageErrors).toEqual([]);
  expect(consoleErrors).toEqual([]);
});

test("Home body stays usable when optional anatomy layers fail", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(150_000);
  const email = `body3d-fallback-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: profileReplacement({
      gender: "male",
      birth_date: "1996-08-27",
    }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  // Muscles and skeleton are optional layers: the translucent skin alone stays.
  await page.route("**/anatomy/z-anatomy-muscles-*.glb", (route) =>
    route.abort(),
  );
  await page.route("**/z-anatomy-1.4.0-skeletal.glb", (route) => route.abort());
  await page.goto("/consultation");
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  const scene = page.locator('.bc-scene[data-viewer-state="ready"]');
  await expect(scene).toBeVisible({ timeout: 75_000 });
  await expect(page.getByText("3D 暂时无法显示")).toHaveCount(0);
  await expect(scene).not.toHaveAttribute("data-muscles", "ready");

  await page
    .getByRole("toolbar", { name: "身体画布工具" })
    .getByRole("button", { name: "图层 · 精细解剖" })
    .click();
  const layerPanel = page.getByRole("group", { name: "精细解剖图层" });
  const skeleton = layerPanel.getByRole("button", { name: "骨骼" });
  await skeleton.click();
  await expect(scene).toHaveAttribute("data-skeleton", "error", {
    timeout: 30_000,
  });
  await expect(skeleton).toHaveAttribute(
    "title",
    "骨骼层暂时无法加载，点按重试",
  );
  await expect(scene).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("body-layers-fallback.png"),
    fullPage: true,
  });
});

function isPinnedAtlasRequest(url: string): boolean {
  return (
    url.includes("/anatomy/vanatome/1.4.0/") ||
    url.includes("/anatomy/z-anatomy-muscles-") ||
    url.includes("atlas.vanatome.vixotic.in/releases/1.4.0/")
  );
}
