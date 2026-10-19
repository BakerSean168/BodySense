import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

test.describe.configure({ timeout: 90_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function register(page: Page, request: APIRequestContext) {
  const email = `episode-person-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "male", birth_date: "2000-01-01" }),
  });
  expect(profile.ok()).toBeTruthy();
  return headers;
}

test("Person Episode explicit create/link uses real Go receipts and survives reload without inferring recovery", async ({
  page,
  request,
}, testInfo) => {
  const headers = await register(page, request);
  await page.goto("/consultation?view=state");
  const person = page.getByTestId("person-window");
  // Penpot Person / Records: Episodes open from the "相关" chips; the editor
  // is the scene's "整理经历" action.
  await person.getByRole("button", { name: "经历 0 段" }).click();
  const disclosure = page.getByTestId("person-episodes");
  await expect(disclosure.getByText(/还没有整理出经历/)).toBeVisible();
  await disclosure.getByRole("button", { name: "整理经历" }).click();
  const editor = disclosure.getByTestId("body-episode-editor");
  // Create a typed self-reported symptom from the real Person UI, not an
  // API-seeded fixture, then require a separate user-authorized Episode action.
  await editor.getByRole("button", { name: "补充可关联的本人症状" }).click();
  await editor.getByLabel("本人完整症状陈述").fill("我右肩有些疼痛");
  await editor.getByLabel("症状类型（本人描述）").fill("右肩疼痛");
  await editor.getByLabel("侧别（不确定可以不指定）").selectOption("right");
  await editor.getByRole("button", { name: "先保存症状陈述" }).click();
  await expect(editor.getByText(/本人症状陈述已保存/)).toBeVisible();
  await person.getByRole("button", { name: "返回记录" }).click();
  const records = page.getByTestId("body-fact-records");
  await expect(records.getByText("我右肩有些疼痛")).toBeVisible();
  await person.getByRole("button", { name: /^经历 \d 段$/ }).click();
  await disclosure.getByRole("button", { name: "整理经历" }).click();
  const firstLedger = await (
    await request.get(`${api}/api/v1/body-state/records`, { headers })
  ).json();
  expect(firstLedger.current_revision).toBe(1);
  expect(firstLedger.facts).toHaveLength(1);
  expect(firstLedger.facts[0].details).toMatchObject({
    symptom_type: "右肩疼痛",
    presence: "present",
    laterality: "right",
  });
  const first = { factId: firstLedger.facts[0].id as string };
  await editor.getByLabel("本人症状陈述").selectOption(first.factId);

  const createRequest = page.waitForRequest(
    (value) =>
      value.method() === "POST" &&
      value.url().endsWith("/api/v1/body-state/episodes"),
  );
  await editor.getByRole("button", { name: "确认新建经历" }).click();
  const payload = (await createRequest).postDataJSON();
  expect(payload).toMatchObject({
    fact_id: first.factId,
    expected_revision: 1,
    idempotency_key: expect.any(String),
  });
  await expect(
    editor.getByText(/关联已提交；仅整理本人症状陈述/),
  ).toBeVisible();
  const original = await (
    await request.get(`${api}/api/v1/body-state/episodes`, { headers })
  ).json();
  expect(original.episodes).toHaveLength(1);
  expect(original.episodes[0].projection.state).toBe("unknown");
  expect(original.episodes[0].links).toHaveLength(1);
  const episodeId = original.episodes[0].id as string;

  const replay = await request.post(`${api}/api/v1/body-state/episodes`, {
    headers,
    data: payload,
  });
  expect(replay.ok()).toBeTruthy();
  const originalReceipt = await replay.json();
  expect(originalReceipt.episode_id).toBe(episodeId);
  expect(originalReceipt.revision).toBe(2);

  // An explicitly negative report is a new Fact, not an automatic resolution.
  await editor.getByRole("button", { name: "补充可关联的本人症状" }).click();
  await editor.getByLabel("本人完整症状陈述").fill("今天我右肩没有疼痛");
  await editor.getByLabel("症状类型（本人描述）").fill("右肩疼痛");
  await editor.getByLabel("当前陈述是否存在症状").selectOption("absent");
  await editor.getByLabel("侧别（不确定可以不指定）").selectOption("right");
  await editor.getByRole("button", { name: "先保存症状陈述" }).click();
  await expect(editor.getByText(/本人症状陈述已保存/)).toBeVisible();
  const secondLedger = await (
    await request.get(`${api}/api/v1/body-state/records`, { headers })
  ).json();
  expect(secondLedger.current_revision).toBe(3);
  const second = {
    factId: secondLedger.facts.find(
      (item: { statement: string }) => item.statement === "今天我右肩没有疼痛",
    )?.id as string,
  };
  expect(second.factId).toBeTruthy();
  const beforeLink = await (
    await request.get(`${api}/api/v1/body-state/episodes/${episodeId}`, {
      headers,
    })
  ).json();
  expect(beforeLink.links).toHaveLength(1);
  expect(beforeLink.projection.state).toBe("unknown");
  await page.reload();
  await person.getByRole("button", { name: "经历 1 段" }).click();
  const reopened = page.getByTestId("person-episodes");
  await expect(
    reopened.getByText("右肩疼痛经历", { exact: true }),
  ).toBeVisible();
  await expect(reopened.getByTitle("持续状态尚未确认")).toHaveText(
    "是否持续 · 未知",
  );
  await reopened.getByRole("button", { name: "整理经历" }).click();
  const secondEditor = reopened.getByTestId("body-episode-editor");
  await secondEditor.getByLabel("操作").selectOption("link");
  await secondEditor.getByLabel("本人症状陈述").selectOption(second.factId);
  await secondEditor.getByLabel("目标经历").selectOption(episodeId);
  await secondEditor.getByRole("button", { name: "确认关联陈述" }).click();
  await expect(
    secondEditor.getByText(/关联已提交；仅整理本人症状陈述/),
  ).toBeVisible();

  const now = await (
    await request.get(`${api}/api/v1/body-state/episodes/${episodeId}`, {
      headers,
    })
  ).json();
  expect(now.links).toHaveLength(2);
  expect(now.projection.state).toBe("unknown");
  expect(now.projection.reason).toBe("insufficient_evidence");
  expect(now.projection.based_on_revision).toBe(4);

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(reopened).toBeVisible();
  const widths = await page.evaluate(() => ({
    client: document.documentElement.clientWidth,
    scroll: document.documentElement.scrollWidth,
  }));
  expect(widths.scroll).toBeLessThanOrEqual(widths.client);
  await page.screenshot({
    path: testInfo.outputPath("person-episode-linked-mobile.png"),
    fullPage: true,
  });
});
