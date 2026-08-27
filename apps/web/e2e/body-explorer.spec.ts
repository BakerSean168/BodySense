import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

test("3D Body Explorer links canonical BodyState, anatomy focus, and chat context", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(240_000);
  const email = `body3d-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  const atlasRequests: string[] = [];

  page.on("request", (req) => {
    if (req.url().includes("/static/anatomy/vanatome/1.4.0/")) {
      atlasRequests.push(req.url());
    }
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
      age: 30,
      height_cm: 175,
      weight_kg: 70,
      occupation: "software engineer",
      exercise_frequency: "2-3/week",
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
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "worsening",
      },
    },
  });
  expect(fact.ok(), await fact.text()).toBeTruthy();

  const coldStart = Date.now();
  await page.goto("/consultation?view=state");
  await expect(
    page.getByRole("combobox", { name: "选择身体区域" }),
  ).toBeVisible();
  await expect(page.getByText("Atlas 1.4.0", { exact: true })).toBeVisible({
    timeout: 75_000,
  });
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

  const canonicalRegionIds = await regionSelect
    .locator("option")
    .evaluateAll((options) =>
      options
        .map((option) => (option as HTMLOptionElement).value)
        .filter(Boolean),
    );
  expect(canonicalRegionIds).toHaveLength(35);

  // Exercise every canonical BodyRegion mapping against the real pinned atlas.
  // This verifies that each focus target is accepted by the live Vanatome viewer;
  // semantic anatomical boundary review remains a separate human visual QA step.
  for (const regionId of canonicalRegionIds) {
    await regionSelect.selectOption(regionId);
    await expect(regionSelect).toHaveValue(regionId);
    await expect(page.getByRole("button", { name: "深入查看" })).toBeVisible();
    await expect(page.getByText("3D 身体视图暂时不可用")).toHaveCount(0);
    await page.waitForTimeout(35);
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
  const canvas = page.locator("canvas").first();
  const canvasBox = await canvas.boundingBox();
  if (canvasBox) {
    const centerY = canvasBox.y + canvasBox.height / 2;
    await page.mouse.move(canvasBox.x + canvasBox.width * 0.75, centerY);
    await page.mouse.down();
    await page.mouse.move(canvasBox.x + canvasBox.width * 0.2, centerY, {
      steps: 20,
    });
    await page.mouse.up();
    await page.waitForTimeout(500);
    await page.screenshot({
      path: testInfo.outputPath("body-explorer-full-body-back.png"),
      fullPage: true,
    });
  }

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
  for (const url of atlasRequests) {
    const parsed = new URL(url);
    expect(parsed.origin).toBe(new URL(page.url()).origin);
    expect(parsed.search).toBe("");
    expect(url).not.toContain(email);
    expect(url).not.toContain("shoulder.right");
  }

  const resourceSummary = await page.evaluate(() => {
    const entries = performance
      .getEntriesByType("resource")
      .filter((entry) =>
        entry.name.includes("/static/anatomy/vanatome/1.4.0/"),
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

  await page.route(
    "**/static/anatomy/vanatome/1.4.0/**/catalog.json",
    (route) => route.abort(),
  );
  await page.reload();
  await expect(page.getByText("3D 身体视图暂时不可用")).toBeVisible({
    timeout: 30_000,
  });
  await page.screenshot({
    path: testInfo.outputPath("body-explorer-atlas-fallback.png"),
    fullPage: true,
  });
});

async function readUsedJsHeap(page: import("@playwright/test").Page) {
  return page.evaluate(() => {
    const performanceWithMemory = performance as Performance & {
      memory?: { usedJSHeapSize?: number };
    };
    return performanceWithMemory.memory?.usedJSHeapSize ?? null;
  });
}
