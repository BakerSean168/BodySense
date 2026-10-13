import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { clearStructuredSafetyCapture } from "./support/safety";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

test("Body Canvas V3 is body-first and preserves explicit spatial/motion context", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(120_000);
  const email = `body-canvas-v3-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
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
    data: { gender: "male", birth_date: "1996-08-27" },
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
        details: { ...clearStructuredSafetyCapture },
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "stable",
      },
    },
  });
  expect(fact.ok(), await fact.text()).toBeTruthy();

  await page.goto("/consultation");
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  await expect(page.getByRole("heading", { name: "我的身体" })).toBeVisible();
  await expect(page.getByLabel("身体上下文助手")).toBeHidden();
  await expect(page.getByRole("button", { name: "探索身体" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );

  await page
    .getByRole("toolbar", { name: "身体画布工具" })
    .getByRole("button", { name: "按名称选择区域" })
    .click();
  const regionSelect = page.getByRole("combobox", { name: "选择身体区域" });
  await expect(regionSelect).toBeVisible();
  await regionSelect.selectOption("shoulder.right");
  await page.getByRole("button", { name: "查看已选区域" }).click();

  await expect(page.getByRole("complementary", { name: "已选区域当前状况" })).toContainText(
    "右肩",
  );
  await expect(page.getByText("抬高手臂时右肩疼")).toBeVisible();

  await page.getByRole("button", { name: "针对这里提问" }).click();
  await expect(page.getByLabel("身体上下文助手")).toBeVisible();
  const contextChip = page
    .getByRole("button", { name: "移除身体区域上下文" })
    .locator("xpath=..");
  await expect(contextChip).toContainText("右肩");
  await expect(contextChip).not.toContainText("站立");

  await page.getByRole("button", { name: "收起助手" }).click();
  await page.getByRole("button", { name: "清除选区" }).click();
  await page.getByRole("button", { name: "涂抹选区" }).click();
  const canvasScene = page.locator('.bc-scene[data-viewer-state="ready"]');
  await expect(canvasScene).toBeVisible({ timeout: 75_000 });
  const canvas = canvasScene.locator("canvas");
  const canvasBox = await canvas.boundingBox();
  expect(canvasBox).not.toBeNull();
  if (canvasBox) {
    const x = canvasBox.x + canvasBox.width * 0.5;
    const y = canvasBox.y + canvasBox.height * 0.32;
    await page.mouse.move(x - 10, y);
    await page.mouse.down();
    await page.mouse.move(x + 10, y + 8, { steps: 5 });
    await page.mouse.up();
  }
  const brushSelection = page.getByRole("complementary", {
    name: "已选区域当前状况",
  });
  await expect(brushSelection).toBeVisible();
  await expect(brushSelection.locator(".bc-selection-chips button").first()).toBeVisible();
  await page.getByRole("button", { name: "旋转与点选" }).click();

  await page.getByRole("button", { name: "动作演示" }).click();
  await expect(page.getByText("MOTION STUDIO")).toBeVisible();
  await page.getByRole("button", { name: /跑步/ }).click();
  await expect(page.getByRole("slider", { name: "动作进度" })).toBeVisible();

  await page.getByRole("button", { name: "询问当前动作" }).click();
  await expect(page.getByLabel("身体上下文助手")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "移除身体区域上下文" }).locator("xpath=.."),
  ).toContainText("跑步");

  await page.getByRole("button", { name: "收起助手" }).click();
  await page.keyboard.press("b");
  await expect(page.getByRole("button", { name: "涂抹选区" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "旋转与点选" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );

  await page.getByRole("button", { name: "训练方案" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("heading", { name: "训练方案" })).toBeVisible();
  await page.getByRole("button", { name: "关闭当前面板" }).click();
  await expect(page.getByRole("dialog")).toBeHidden();

  await page.getByRole("button", { name: "我的设备" }).click();
  const equipmentDialog = page.getByRole("dialog");
  await expect(equipmentDialog.getByRole("heading", { name: "我的设备" })).toBeVisible();
  await equipmentDialog.getByLabel("设备名称").fill("弹力带");
  await equipmentDialog.getByLabel("规格或备注").fill("中等阻力");
  await equipmentDialog.getByRole("button", { name: "添加设备" }).click();
  await expect(equipmentDialog.getByText("弹力带", { exact: true })).toBeVisible();
  await expect(equipmentDialog.getByText("当前可用")).toBeVisible();

  const equipmentStateResponse = await request.get(
    `${apiBase}/api/v1/body-state`,
    { headers },
  );
  const equipmentStateBody = await equipmentStateResponse.text();
  expect(equipmentStateResponse.ok(), equipmentStateBody).toBeTruthy();
  const equipmentState = JSON.parse(equipmentStateBody) as {
    facts: Array<{ kind: string; value: string; review_state: string }>;
  };
  expect(equipmentState.facts).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        kind: "equipment.available",
        value: "弹力带",
        review_state: "confirmed",
      }),
    ]),
  );
  await equipmentDialog.getByRole("button", { name: "关闭当前面板" }).click();

  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1),
  ).toBe(true);

  await page.screenshot({
    path: testInfo.outputPath("body-canvas-v3-desktop.png"),
    fullPage: true,
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  await page
    .getByRole("toolbar", { name: "身体画布工具" })
    .getByRole("button", { name: "按名称选择区域" })
    .click();
  await expect(page.getByRole("complementary", { name: "区域名称选择" })).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("body-canvas-v3-mobile.png"),
    fullPage: true,
  });
});
