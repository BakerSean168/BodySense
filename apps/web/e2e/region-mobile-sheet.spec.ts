import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

// Penpot "Mobile / Body / Region": at 390 the region detail is the near-full
// card the mobile windows use (16px inset, ending above the Dock), not a
// bottom half-sheet over the body.
test("390 region detail is the near-full-height card from Penpot", async ({
  page,
  request,
}) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`region-sheet-${crypto.randomUUID()}@example.test`);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(onboarding|consultation|dashboard)/);
  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "female", birth_date: "1993-05-02" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  const fact = await request.post(`${apiBase}/api/v1/body-state/records`, {
    headers,
    data: {
      idempotency_key: `region-sheet-${crypto.randomUUID()}`,
      expected_revision: 0,
      facts: [
        {
          kind: "symptom",
          statement: "低头久了右肩发紧",
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
  await page.getByRole("button", { name: "快捷键" }).click();
  await page.getByRole("button", { name: "按名称选择身体区域" }).click();
  await page
    .getByRole("combobox", { name: "选择身体区域" })
    .selectOption("shoulder.right");
  await page.getByRole("button", { name: "查看已选区域" }).click();

  const card = page.getByRole("complementary", { name: "已选区域当前状况" });
  await expect(card).toContainText("右肩");
  await expect(card.getByText("低头久了右肩发紧")).toBeVisible();

  const box = await card.boundingBox();
  const dock = await page.locator(".bc-dock").boundingBox();
  expect(box).not.toBeNull();
  expect(dock).not.toBeNull();
  // 16px side insets and a top near the stage edge (below the safety banner
  // when one shows), so the card covers most of the viewport height.
  expect(Math.round(box!.x)).toBe(16);
  expect(Math.round(box!.width)).toBe(390 - 32);
  expect(box!.y).toBeLessThan(150);
  expect(box!.height).toBeGreaterThan(844 * 0.7);
  // It ends above the Dock instead of running under it.
  expect(box!.y + box!.height).toBeLessThanOrEqual(dock!.y);

  // The footer actions sit on the card's bottom edge.
  const ask = await card
    .getByRole("button", { name: "围绕右肩提问" })
    .boundingBox();
  expect(ask!.y + ask!.height).toBeGreaterThan(box!.y + box!.height - 80);
  // The rail gives way while the card is open.
  await expect(
    page.getByRole("toolbar", { name: "身体画布工具" }),
  ).toBeHidden();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);

  await card.getByRole("button", { name: "收起区域详情" }).click();
  await expect(card).toBeHidden();
  await expect(
    page.getByRole("toolbar", { name: "身体画布工具" }),
  ).toBeVisible();
});
