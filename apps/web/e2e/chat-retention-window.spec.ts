import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import {
  conversationIdFor,
  requiresDeterministicModel,
  settleConversation,
} from "./support/consultation";
import { isTerminal, readRunEvents } from "./support/runEvents";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

// PC-RET-001 in a real browser against the real Go retention path: the single
// Conversation keeps a rolling raw-history window sized by the context token
// budget (CHAT_CONTEXT_TOKEN_BUDGET, 16000 by default). Once settled turns
// exceed it, the context cursor moves past the oldest turn and the prune job
// (woken at once by the enqueue) deletes it. The newest turns stay readable
// and the chat keeps working after the prune.
test.describe.configure({ timeout: 300_000 });
// Chat turns can only be created by chat Runs. On a real model each ~18 KB
// filler turn streams past the Go runtime's 120 s AI read deadline and fails
// (run.failed "context deadline exceeded" on staging), so the budget cannot
// be filled there. Model follow-up questions are handled in send() below.
requiresDeterministicModel(
  "filler turns exceed the 120 s AI runtime read deadline on a real model",
);
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
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const headers = {
    Authorization: `Bearer ${await refreshBrowserAccessToken(page, api)}`,
  };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "female", birth_date: "1994-03-12" }),
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

// Sends one turn through the composer and waits for its Run to settle, read
// from the durable Run event log (see support/runEvents). A real model may end
// the turn on a question instead of completing it; that question is not under
// test, so its Run is cancelled explicitly and the turn is settled either way.
async function send(
  page: Page,
  headers: Record<string, string>,
  conversationId: string,
  text: string,
): Promise<void> {
  const request = page.context().request;
  const response = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/v1/consultation-runs") &&
      r.request().method() === "POST",
  );
  await page.getByPlaceholder(composerPlaceholder).fill(text);
  await page.getByRole("button", { name: "发送", exact: true }).click();
  expect((await response).ok()).toBeTruthy();
  const thread = await request.get(
    `${api}/api/v1/consultations/${conversationId}/thread`,
    { headers },
  );
  expect(thread.ok(), await thread.text()).toBeTruthy();
  const runId = ((await thread.json()) as { active_turn_run_id?: string })
    .active_turn_run_id;
  if (runId) {
    const events = await readRunEvents(
      request,
      headers,
      conversationId,
      runId,
      // A real model turn on a long message can take well over 30 s.
      { until: (events) => events.some(isTerminal), timeout: 90_000 },
    );
    expect(events.some((event) => event.type === "stream.error")).toBe(false);
    const ended = events.some(
      (event) =>
        event.type === "run.completed" || event.type === "run.interrupted",
    );
    expect(ended).toBe(true);
  }
  if ((await settleConversation(request, api, headers, conversationId)) > 0) {
    await page.reload();
    await openChat(page);
  }
  await expect(page.getByPlaceholder(composerPlaceholder)).toBeEnabled({
    timeout: 30_000,
  });
}

test("the rolling chat window prunes the oldest turn and the chat keeps working", async ({
  page,
}) => {
  const headers = await register(page, page.context().request);
  await openChat(page);

  const conversationId = await conversationIdFor(
    page.context().request,
    api,
    headers,
  );
  await send(page, headers, conversationId, `${marker(1)} ${filler}`);
  await send(page, headers, conversationId, `${marker(2)} ${filler}`);
  // Two turns fit the budget: nothing is pruned yet.
  const threadURL = `${api}/api/v1/consultations/${conversationId}/thread`;
  const thread = async () => {
    const response = await page.context().request.get(threadURL, { headers });
    expect(response.ok(), await response.text()).toBeTruthy();
    return response.text();
  };
  expect(await thread()).toContain(marker(1));

  await send(page, headers, conversationId, `${marker(3)} ${filler}`);
  await expect
    .poll(async () => (await thread()).includes(marker(1)), { timeout: 30_000 })
    .toBe(false);

  await page.reload();
  await openChat(page);
  await expect(page.getByText(marker(3)).first()).toBeVisible();
  await expect(page.getByText(marker(2)).first()).toBeVisible();
  await expect(page.getByText(marker(1))).toHaveCount(0);

  // The next turn runs normally after the prune, and survives a reload.
  await send(page, headers, conversationId, "保留窗口之后的新消息");
  await page.reload();
  await openChat(page);
  await expect(page.getByText("保留窗口之后的新消息").first()).toBeVisible();
  await expect(page.getByText(marker(1))).toHaveCount(0);
  const latest = await thread();
  expect(latest).toContain(marker(2));
  expect(latest).not.toContain(marker(1));
});
