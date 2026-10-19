import { expect, test, type Page } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

// Penpot "Person / Media": uploads from the Person window land in a month
// grouped thumbnail grid; a failed upload is the pink row with retry.
const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
// Two distinct 1x1 PNGs, so the uploads never share a content hash.
const PNGS = [
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
];
const PNG = (index: number) => Buffer.from(PNGS[index], "base64");

async function openMedia(page: Page) {
  await page.goto("/consultation?view=person");
  const person = page.getByTestId("person-window");
  await expect(person).toBeVisible();
  await page.getByLabel("档案分类").selectOption("media");
  return person;
}

test("person media uploads land in the penpot month grid; failures retry", async ({
  page,
  request,
}) => {
  test.setTimeout(120_000);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`media-grid-${crypto.randomUUID()}@example.test`);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(onboarding|consultation|dashboard)/);
  const token = await refreshBrowserAccessToken(page, api);
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers: { Authorization: `Bearer ${token}` },
    data: profileReplacement({ gender: "male", birth_date: "1991-04-09" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  let person = await openMedia(page);
  // Empty state upload (the #399 path) still works.
  await person.locator('input[type="file"]').setInputFiles({
    name: "体态正面.png",
    mimeType: "image/png",
    buffer: PNG(0),
  });
  const tiles = person.getByTestId("person-media-tile");
  await expect(tiles).toHaveCount(1, { timeout: 20_000 });
  await expect(person.getByText("1 个", { exact: true })).toBeVisible();
  const month = `${new Date().getMonth() + 1}月`;
  const group = person.getByRole("region", { name: month });
  await expect(group.getByText("体态正面.png")).toBeVisible();
  await expect(group.getByText(/· 今天$/)).toBeVisible();

  // A failed upload shows Penpot's pink row; retry sends the same file.
  let fail = true;
  await page.route("**/api/v1/uploads", async (route) => {
    if (route.request().method() === "POST" && fail) {
      fail = false;
      await route.fulfill({
        status: 503,
        json: { error: { code: "unavailable", message: "down" } },
      });
      return;
    }
    await route.fallback();
  });
  await person.locator('input[type="file"]').setInputFiles({
    name: "体态侧面.png",
    mimeType: "image/png",
    buffer: PNG(1),
  });
  const failed = person.getByRole("alert");
  await expect(failed).toContainText("体态侧面.png");
  await expect(failed).toContainText("上传失败");
  await failed.getByRole("button", { name: "重试" }).click();
  await expect(tiles).toHaveCount(2, { timeout: 20_000 });
  await expect(person.getByRole("alert")).toHaveCount(0);

  // 390: two columns and the labelled 上传 pill.
  await page.setViewportSize({ width: 390, height: 844 });
  person = await openMedia(page);
  await expect(tiles).toHaveCount(2);
  const upload = person.getByRole("button", { name: "上传照片" });
  await expect(upload).toContainText("上传");
  const [a, b] = [
    await tiles.nth(0).boundingBox(),
    await tiles.nth(1).boundingBox(),
  ];
  expect(Math.abs(a!.y - b!.y)).toBeLessThan(2);
  expect(b!.x).toBeGreaterThan(a!.x + a!.width);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);
});
