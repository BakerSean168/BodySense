import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { clearStructuredSafetyCapture } from "./support/safety";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const publicAssetOrigin = process.env.E2E_PUBLIC_ASSET_ORIGIN?.replace(
  /\/$/,
  "",
);

test("3D Body Explorer links canonical BodyState, anatomy focus, and chat context", async ({
  page,
  request,
}, testInfo) => {
  // The canonical 35-region vocabulary and all curated Vanatome mappings are
  // exhaustively validated by the fast ontology/mapping contract tests. This
  // browser scenario intentionally samples representative regions while keeping
  // the expensive real-atlas/WebGL checks, visual states, warm reloads, and
  // tab/view recovery under software-rendered CI.
  test.setTimeout(360_000);
  const email = `body3d-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  const atlasRequests: string[] = [];
  const atlasRequestFailures: Array<{ url: string; errorText: string }> = [];
  const bodyExplorerChunkRequests: string[] = [];

  page.on("request", (req) => {
    const url = req.url();
    if (isPinnedAtlasRequest(url)) {
      atlasRequests.push(url);
    }
    if (/\/assets\/BodyExplorer3D-[^/]+\.js(?:$|\?)/.test(url)) {
      bodyExplorerChunkRequests.push(url);
    }
  });
  page.on("requestfailed", (req) => {
    if (!isPinnedAtlasRequest(req.url())) return;
    atlasRequestFailures.push({
      url: req.url(),
      errorText: req.failure()?.errorText ?? "unknown",
    });
  });

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: {
      gender: "male",
      birth_date: "1996-08-27",
    },
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  const fact = await request.post(`${apiBase}/api/v1/body-state/facts`, {
    headers,
    data: {
      expected_revision: 0,
      fact: {
        concern_key: "region:shoulder.right",
        kind: "discomfort",
        body_region: "右肩",
        body_region_id: "shoulder.right",
        value: "抬高手臂时右肩疼",
        details: {
          ...clearStructuredSafetyCapture,
        },
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "worsening",
      },
    },
  });
  expect(fact.ok(), await fact.text()).toBeTruthy();

  const consoleErrors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });

  const coldStart = Date.now();
  await page.goto("/consultation?view=state");
  await expect(
    page.getByRole("combobox", { name: "选择身体区域" }),
  ).toBeVisible();
  await expect
    .poll(
      async () => {
        if (await page.getByText("3D 身体视图暂时不可用").isVisible()) {
          throw new Error(
            `3D atlas failed before readiness: ${JSON.stringify(atlasRequestFailures)}`,
          );
        }
        return page.getByText("Atlas 1.4.0", { exact: true }).isVisible();
      },
      { timeout: 75_000 },
    )
    .toBe(true);
  await expect(page.getByTestId("body-explorer-3d")).toHaveAttribute(
    "data-viewer-state",
    "ready",
    { timeout: 75_000 },
  );
  const coldReadyMs = Date.now() - coldStart;
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-full-body-front.png"),
    fullPage: true,
  });

  const regionSelect = page.getByRole("combobox", { name: "选择身体区域" });
  const pageErrors: Error[] = [];
  page.on("pageerror", (error) => pageErrors.push(error));

  // The regional shell is intentionally non-selectable. Load the muscular
  // atlas through the normal semantic-region path, then clear the semantic
  // selection while retaining that atlas. A subsequent canvas click must
  // therefore create a fresh anatomy selection through Vanatome raycasting.
  await regionSelect.selectOption("shoulder.right");
  await expect(page.getByTestId("body-explorer-3d")).toHaveAttribute(
    "data-loaded-systems",
    /muscular/,
    { timeout: 75_000 },
  );
  await regionSelect.selectOption("");
  await expect(regionSelect).toHaveValue("");

  const canvas = page.locator("canvas").first();
  const canvasBox = await canvas.boundingBox();
  expect(canvasBox).not.toBeNull();
  const focusButton = page.getByRole("button", { name: "聚焦" });
  await expect(focusButton).toBeDisabled();
  let pointerHit = false;
  if (canvasBox) {
    const hitPoints = [
      [0.5, 0.34],
      [0.46, 0.3],
      [0.54, 0.3],
      [0.5, 0.42],
      [0.5, 0.5],
      [0.42, 0.42],
      [0.58, 0.42],
      [0.46, 0.58],
      [0.54, 0.58],
    ] as const;
    for (const [xRatio, yRatio] of hitPoints) {
      await page.mouse.move(
        canvasBox.x + canvasBox.width * xRatio,
        canvasBox.y + canvasBox.height * yRatio,
      );
      await page.waitForTimeout(120);
      await page.mouse.click(
        canvasBox.x + canvasBox.width * xRatio,
        canvasBox.y + canvasBox.height * yRatio,
      );
      if (await focusButton.isEnabled()) {
        pointerHit = true;
        break;
      }
    }
  }
  expect(pointerHit).toBe(true);
  await expect(page.getByRole("button", { name: "隔离" })).toBeEnabled();
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-pointer-hit.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "返回全身" }).click();
  await expect(focusButton).toBeDisabled();

  const canonicalRegionIds = await regionSelect
    .locator("option")
    .evaluateAll((options) =>
      options
        .map((option) => (option as HTMLOptionElement).value)
        .filter(Boolean),
    );
  expect(canonicalRegionIds).toHaveLength(35);

  // The browser layer verifies representative axial, bilateral, and lower-limb
  // focus targets. Exhaustive 35/35 mapping coverage lives in
  // bodyRegionOntology.test.ts + anatomyMapping.test.ts, where it is deterministic
  // and does not require a full software-rendered WebGL focus cycle per region.
  const representativeRegionIds = ["head", "shoulder.left", "knee.left"];
  for (const regionId of representativeRegionIds) {
    expect(canonicalRegionIds).toContain(regionId);
    await regionSelect.selectOption(regionId);
    await expect(regionSelect).toHaveValue(regionId);
    await expect(page.getByRole("button", { name: "深入查看" })).toBeVisible();
    await expect(page.getByText("3D 身体视图暂时不可用")).toHaveCount(0);
  }
  expect(pageErrors).toEqual([]);

  await regionSelect.selectOption("shoulder.right");
  await expect(page.getByText("抬高手臂时右肩疼")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-right-shoulder-selected.png"),
    fullPage: true,
  });

  await page.getByRole("button", { name: "深入查看" }).click();
  await page.getByRole("button", { name: "肌肉" }).click();
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-anatomy-muscular.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "返回区域" }).click();

  await regionSelect.selectOption("lower_back");
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-lower-back-selected.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "深入查看" }).click();
  await page.getByRole("button", { name: "骨骼" }).click();
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-anatomy-skeletal.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "返回区域" }).click();

  await regionSelect.selectOption("");
  await regionSelect.selectOption("shoulder.right");
  const heapBeforeTabs = await readUsedJsHeap(page);
  for (let index = 0; index < 5; index += 1) {
    await page.getByRole("tab", { name: "分析" }).click();
    await expect(page.getByRole("tab", { name: "分析" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await page.getByRole("tab", { name: "状态" }).click();
    await expect(page.getByText("Atlas 1.4.0", { exact: true })).toBeVisible({
      timeout: 30_000,
    });
  }
  const heapAfterTabs = await readUsedJsHeap(page);

  await page.getByRole("button", { name: "收起对话区" }).click();
  await expect(page.getByRole("button", { name: "展开对话区" })).toBeVisible();

  const askButtons = page.getByRole("button", { name: "询问 BodySense" });
  await askButtons.first().click();
  await expect(page.getByRole("button", { name: "收起对话区" })).toBeVisible();
  await expect(page.getByText(/右肩/).last()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "移除身体区域上下文" }),
  ).toBeVisible();

  await page.screenshot({
    path: testInfo.outputPath("body-explorer-right-shoulder.png"),
    fullPage: true,
  });

  expect(atlasRequests.length).toBeGreaterThan(0);
  const staticRequests = [...atlasRequests, ...bodyExplorerChunkRequests];
  for (const url of staticRequests) {
    const parsed = new URL(url);
    expect(parsed.search).toBe("");
    expect(url).not.toContain(email);
    expect(url).not.toContain("shoulder.right");
  }
  if (publicAssetOrigin) {
    expect(bodyExplorerChunkRequests.length).toBeGreaterThan(0);
    for (const url of staticRequests) {
      expect(new URL(url).origin).toBe(publicAssetOrigin);
    }
  }

  const resourceSummary = await page.evaluate(() => {
    const entries = performance
      .getEntriesByType("resource")
      .filter((entry) =>
        entry.name.includes("/anatomy/vanatome/1.4.0/"),
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
  const warmStart = Date.now();
  await page.reload();
  await expect(page.getByText("Atlas 1.4.0", { exact: true })).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByTestId("body-explorer-3d")).toHaveAttribute(
    "data-viewer-state",
    "ready",
    { timeout: 30_000 },
  );
  const warmReadyMs = Date.now() - warmStart;

  testInfo.annotations.push({
    type: "body3d-performance",
    description: JSON.stringify({
      coldReadyMs,
      warmReadyMs,
      heapBeforeTabs,
      heapAfterTabs,
      ...resourceSummary,
    }),
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "工作区" }).click();
  await expect(
    page.getByRole("combobox", { name: "选择身体区域" }),
  ).toBeVisible();
  await expect(page.getByText("Atlas 1.4.0", { exact: true })).toBeVisible();
  expect(consoleErrors).toEqual([]);
});

test("3D Body Explorer falls back when atlas metadata is unavailable", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(90_000);
  const email = `body3d-fallback-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: {
      gender: "male",
      birth_date: "1996-08-27",
    },
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  await page.route("**/releases/1.4.0/catalog.json", (route) => route.abort());
  await page.goto("/consultation?view=state");

  await expect(page.getByText("3D 身体视图暂时不可用")).toBeVisible({
    timeout: 30_000,
  });
  await expect(
    page.getByText("已切换到可访问的 2D 身体概览。", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "重试" })).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-atlas-fallback.png"),
    fullPage: true,
  });
});

function isPinnedAtlasRequest(url: string): boolean {
  return (
    url.includes("/anatomy/vanatome/1.4.0/") ||
    url.includes("atlas.vanatome.vixotic.in/releases/1.4.0/")
  );
}

async function readUsedJsHeap(page: import("@playwright/test").Page) {
  return page.evaluate(() => {
    const performanceWithMemory = performance as Performance & {
      memory?: { usedJSHeapSize?: number };
    };
    return performanceWithMemory.memory?.usedJSHeapSize ?? null;
  });
}
