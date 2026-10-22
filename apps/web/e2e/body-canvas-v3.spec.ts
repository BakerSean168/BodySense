import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";
import { choosePersonCategory } from "./support/person";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

test("Body Canvas V3 is body-first and preserves explicit spatial/motion context", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(120_000);
  // Desktop-authored flow (Tools rail, dock, hotkeys); the 390px layout has
  // its own test below. Pin the desktop frame so matrix projects agree.
  await page.setViewportSize({ width: 1280, height: 720 });
  const email = `body-canvas-v3-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
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
    data: profileReplacement({ gender: "male", birth_date: "1996-08-27" }),
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

  await page.goto("/consultation");
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  await expect(page.getByRole("heading", { name: "我的身体" })).toBeVisible();
  // Body-first: no business window is open on a bare consultation route.
  await expect(page.getByTestId("diagnosis-window")).toBeHidden();
  // Penpot Home: no app bar; the pose bar is hidden and the body stands.
  const ribbon = page.getByRole("group", { name: "参考动作" });
  await expect(ribbon).toHaveCount(0);
  await expect(page.getByRole("button", { name: "开始一次提问" })).toHaveCount(
    0,
  );
  await expect(
    page.getByRole("toolbar", { name: "身体画布工具" }).getByRole("button"),
  ).toHaveCount(5);

  // The Trainer shortcut is navigation, never an implicit plan execution.
  await page.keyboard.press("t");
  const trainerWindow = page.locator(
    '.bc-business-window[data-window="trainer"]',
  );
  await expect(trainerWindow.getByTestId("trainer-entry-guard")).toBeVisible();
  await expect(trainerWindow.getByText(/训练执行尚未就绪/)).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(trainerWindow).toBeHidden();
  await page.keyboard.press("m");
  const shortcutPerson = page.locator(
    '.bc-business-window[data-window="person"]',
  );
  await expect(shortcutPerson).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(shortcutPerson).toBeHidden();

  // The region-by-name list lives in the help overlay (Penpot rail has 4 tools).
  await page.getByRole("button", { name: "快捷键" }).click();
  await page.getByRole("button", { name: "按名称选择身体区域" }).click();
  const regionSelect = page.getByRole("combobox", { name: "选择身体区域" });
  await expect(regionSelect).toBeVisible();
  await regionSelect.selectOption("shoulder.right");
  await page.getByRole("button", { name: "查看已选区域" }).click();

  const selectedRegionPanel = page.getByRole("complementary", {
    name: "已选区域当前状况",
  });
  await expect(selectedRegionPanel).toContainText("右肩");
  await expect(selectedRegionPanel.getByText("抬高手臂时右肩疼")).toBeVisible();

  await expect(selectedRegionPanel.getByText("已选择")).toBeVisible();
  await selectedRegionPanel
    .getByRole("button", { name: "围绕右肩提问" })
    .click();
  const chatWindow = page.locator('.bc-business-window[data-window="chat"]');
  await expect(chatWindow).toBeVisible();
  const contextChip = page
    .getByRole("button", { name: "移除身体区域上下文" })
    .locator("xpath=..");
  await expect(contextChip).toContainText("右肩");
  await expect(contextChip).not.toContainText("站立");

  await chatWindow.getByRole("button", { name: "关闭对话" }).click();
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
  await expect(
    brushSelection.locator(".bc-selection-chips button").first(),
  ).toBeVisible();
  // The docked Body / Region scene hides the rail (Penpot); V still selects.
  await page.keyboard.press("v");
  await expect(
    page.getByRole("button", { name: "旋转与点选", includeHidden: true }),
  ).toHaveAttribute("aria-pressed", "true");

  // With the pose bar hidden, the motion shortcuts leave the body standing.
  await page.keyboard.press("2");
  await expect(page.getByRole("slider", { name: "动作进度" })).toHaveCount(0);
  await expect(ribbon).toHaveCount(0);

  // The Region scene is still docked, so the rail is hidden; keys still work.
  await page.keyboard.press("b");
  await expect(
    page.getByRole("button", { name: "涂抹选区", includeHidden: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "旋转与点选", includeHidden: true }),
  ).toHaveAttribute("aria-pressed", "true");
  // Without a motion question to hand the region off, clear it explicitly.
  await page.getByRole("button", { name: "清除选区" }).click();
  await expect(brushSelection).toBeHidden();

  // V2 TrainingPlan: plan selector belongs to the shared header, no duplicate
  // content selector/title and no local action mutates an unaccepted plan.
  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "训练计划", exact: true })
    .click();
  const trainingWindow = page.locator(
    '.bc-business-window[data-window="training"]',
  );
  await expect(trainingWindow).toBeVisible();
  // Penpot Training / Empty: a new account shows the title only, no picker.
  await expect(trainingWindow.getByTestId("training-empty")).toBeVisible();
  await expect(
    trainingWindow.getByRole("combobox", { name: "选择训练计划" }),
  ).toHaveCount(0);
  await trainingWindow.getByRole("button", { name: "关闭训练计划" }).click();
  await expect(trainingWindow).toBeHidden();

  // Equipment lives in the Person window; adding is only the header "+",
  // which opens a dialog that persists a real EquipmentAsset.
  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "个人档案", exact: true })
    .click();
  const personWindow = page.locator(
    '.bc-business-window[data-window="person"]',
  );
  await expect(personWindow).toBeVisible();
  await choosePersonCategory(page, "equipment");
  await expect(personWindow.getByText("还没有器材")).toBeVisible();
  // No fixed slot ceiling is shown anywhere.
  await expect(personWindow.getByText(/\/\s*12/)).toBeHidden();

  await personWindow.getByRole("button", { name: "添加器材" }).click();
  const addDialog = page.getByRole("dialog", { name: "添加器材" });
  await addDialog.getByLabel("名称").fill("弹力带");
  await addDialog.getByLabel(/备注/).fill("中等阻力");
  await addDialog.getByRole("button", { name: "添加", exact: true }).click();
  await expect(addDialog).toBeHidden();
  await expect(personWindow.getByText("弹力带", { exact: true })).toBeVisible();
  await expect(
    personWindow.getByRole("switch", { name: "弹力带 可用" }),
  ).toBeChecked();

  // Equipment is its own owner-scoped EquipmentAsset, never a BodyState fact.
  const equipmentResponse = await request.get(`${apiBase}/api/v1/equipment`, {
    headers,
  });
  const equipmentBody = await equipmentResponse.text();
  expect(equipmentResponse.ok(), equipmentBody).toBeTruthy();
  const equipment = JSON.parse(equipmentBody) as {
    items: Array<{
      display_name: string;
      notes: string | null;
      availability: string;
    }>;
  };
  expect(equipment.items).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        display_name: "弹力带",
        notes: "中等阻力",
        availability: "available",
      }),
    ]),
  );
  await personWindow.getByRole("button", { name: "关闭个人档案" }).click();

  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);

  await page.screenshot({
    path: testInfo.outputPath("body-canvas-v3-desktop.png"),
    fullPage: true,
  });

  await page.setViewportSize({ width: 390, height: 844 });
  // Mobile: no pose bar and no motion playback bar.
  await expect(ribbon).toHaveCount(0);
  await expect(page.locator(".bc-playback")).toHaveCount(0);
  // V2 mobile retains all four module entrances, including Training.
  await expect(page.locator(".bc-dock .bc-dock-item")).toHaveCount(4);
  await expect(
    page.locator(".bc-dock").getByRole("button", { name: "训练计划" }),
  ).toBeVisible();
  // The persistent body stage survives the window close above.
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  await page.getByRole("button", { name: "快捷键" }).click();
  await page.getByRole("button", { name: "按名称选择身体区域" }).click();
  await expect(
    page.getByRole("complementary", { name: "区域名称选择" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("body-canvas-v3-mobile.png"),
    fullPage: true,
  });
});
