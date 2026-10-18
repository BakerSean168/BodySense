import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { parseStreamEvent } from "@bodysense/contracts";
import { refreshBrowserAccessToken } from "./support/auth";

// PC-RET-001 in a real browser against the real Go retention path: the single
// Conversation keeps a rolling raw-history window sized by the context token
// budget (CHAT_CONTEXT_TOKEN_BUDGET, 16000 by default). Once settled turns
// exceed it, the context cursor moves past the oldest turn and the prune job
// (woken at once by the enqueue) deletes it. The newest turns stay readable
// and the chat keeps working after the prune.
test.describe.configure({ timeout: 120_000 });
const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const composerPlaceholder = "和 BodySense 说说你的身体感受…";

// About 6000 tokens per turn under the Go estimator (18000 UTF-8 bytes / 3),
// so three settled turns are above the default budget and two are not.
const filler = "今天按时起床吃早饭散步".repeat(600);
const marker = (turn: number) => `保留窗口第${turn}轮`;

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`retention-${crypto.randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const headers = {
    Authorization: `Bearer ${await refreshBrowserAccessToken(page, api)}`,
  };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: { gender: "female", birth_date: "1994-03-12" },
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return headers;
}

async function openChat(page: Page) {
  await page.goto("/consultation?view=chat");
  // The lazy chat chunk can take longer than the default budget on a cold Vite server.
  await expect(page.getByPlaceholder(composerPlaceholder)).toBeVisible({
    timeout: 30_000,
  });
}

// Sends one turn through the composer and waits for its Run to settle.
async function send(page: Page, text: string): Promise<string> {
  const response = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/v1/consultation-runs") &&
      r.request().method() === "POST",
  );
  await page.getByPlaceholder(composerPlaceholder).fill(text);
  await page.getByRole("button", { name: "发送", exact: true }).click();
  const settled = await response;
  expect(settled.ok()).toBeTruthy();
  const body = await settled.text();
  const first = body
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => parseStreamEvent(JSON.parse(line.slice(5).trim())))
    .find((event) => event.ids.conversation_id);
  expect(body).toContain("run.completed");
  await expect(page.getByPlaceholder(composerPlaceholder)).toBeEnabled();
  return first!.ids.conversation_id!;
}

test("the rolling chat window prunes the oldest turn and the chat keeps working", async ({
  page,
}) => {
  const headers = await register(page, page.context().request);
  await openChat(page);

  const conversationId = await send(page, `${marker(1)} ${filler}`);
  await send(page, `${marker(2)} ${filler}`);
  // Two turns fit the budget: nothing is pruned yet.
  const threadURL = `${api}/api/v1/consultations/${conversationId}/thread`;
  const thread = async () => {
    const response = await page.context().request.get(threadURL, { headers });
    expect(response.ok(), await response.text()).toBeTruthy();
    return response.text();
  };
  expect(await thread()).toContain(marker(1));

  await send(page, `${marker(3)} ${filler}`);
  await expect
    .poll(async () => (await thread()).includes(marker(1)), { timeout: 30_000 })
    .toBe(false);

  await page.reload();
  await openChat(page);
  await expect(page.getByText(marker(3)).first()).toBeVisible();
  await expect(page.getByText(marker(2)).first()).toBeVisible();
  await expect(page.getByText(marker(1))).toHaveCount(0);

  // The next turn runs normally after the prune, and survives a reload.
  await send(page, "保留窗口之后的新消息");
  await page.reload();
  await openChat(page);
  await expect(page.getByText("保留窗口之后的新消息").first()).toBeVisible();
  await expect(page.getByText(marker(1))).toHaveCount(0);
  const latest = await thread();
  expect(latest).toContain(marker(2));
  expect(latest).not.toContain(marker(1));
});
