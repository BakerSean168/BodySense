import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { conversationIdFor } from "./support/consultation";
import { seedEpisodicNotes } from "./support/episodicNotes";
import { choosePersonCategory } from "./support/person";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

// S6-B #516: owner inspection and permanent deletion of non-authoritative
// conversational memory at 390px. The feature flag is enabled only in the test
// environment; this spec never touches staging or production.
test.describe.configure({ timeout: 120_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`person-memory-${crypto.randomUUID()}@example.test`);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);
  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "male", birth_date: "2000-01-01" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return headers;
}

test("Person memory lists owner notes and permanently deletes one at 390px", async ({
  page,
  request,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const headers = await register(page, request);

  const meResponse = await request.get(`${api}/api/v1/me`, { headers });
  expect(meResponse.ok(), await meResponse.text()).toBeTruthy();
  const { id: userId } = (await meResponse.json()) as { id: string };
  const conversationId = await conversationIdFor(request, api, headers);

  const firstContent = "用户偏好早上训练，晚上不做高强度运动。";
  const secondContent = "用户提到最近睡眠变短，希望周末补觉。";
  await seedEpisodicNotes(userId, conversationId, [
    { contentText: firstContent, fromSeq: 3, toSeq: 8 },
    { contentText: secondContent, fromSeq: 9, toSeq: 14 },
  ]);

  await page.goto("/consultation?view=person");
  await expect(page.getByTestId("person-window")).toBeVisible();
  await choosePersonCategory(page, "memory");

  const notes = page.getByTestId("episodic-note");
  await expect(notes).toHaveCount(2);
  await expect(page.getByText(firstContent)).toBeVisible();
  await expect(page.getByText(secondContent)).toBeVisible();
  await expect(page.getByText("对话第 3–8 条")).toBeVisible();
  await expect(page.getByText("对话第 9–14 条")).toBeVisible();
  await expect(page.getByText("仅保存，不参与对话")).toHaveCount(2);

  // Nothing in the 390px list may overflow the viewport or its sheet.
  const geometry = await page.evaluate(() => {
    const body = document.querySelector(".bc-person-body");
    const list = document.querySelector(".bc-person-notes");
    return {
      page: document.documentElement.scrollWidth - window.innerWidth,
      body: body ? body.scrollWidth - body.clientWidth : -1,
      list: list ? list.scrollWidth - list.clientWidth : -1,
      cards: list ? list.querySelectorAll("li").length : 0,
    };
  });
  expect(geometry).toEqual({ page: 0, body: 0, list: 0, cards: 2 });
  await page.screenshot({
    path: testInfo.outputPath("person-memory-list-390.png"),
  });

  await page.getByRole("button", { name: /^删除记忆：.*早上训练/ }).click();
  const dialog = page.getByRole("dialog", { name: "删除这条记忆？" });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(
    "删除后这条记忆会被永久移除，且不会被重新生成。",
  );
  await expect(dialog).toContainText("这不会删除身体记录等临床事实");
  await expect(dialog).toContainText("删除不可撤销。");
  await page.getByRole("button", { name: "永久删除" }).click();

  await expect(notes).toHaveCount(1);
  await expect(page.getByText(firstContent)).toHaveCount(0);
  await expect(page.getByText(secondContent)).toBeVisible();
  await expect(dialog).not.toBeVisible();

  // Re-read the authoritative owner list: the removed note never reappears.
  const after = await request.get(`${api}/api/v1/profile/episodic-notes`, {
    headers,
  });
  expect(after.ok(), await after.text()).toBeTruthy();
  const pageBody = (await after.json()) as {
    notes: { contentText: string }[];
  };
  expect(pageBody.notes).toHaveLength(1);
  expect(pageBody.notes[0].contentText).toBe(secondContent);
  await page.screenshot({
    path: testInfo.outputPath("person-memory-after-delete-390.png"),
  });
});
