import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { parseStreamEvent, type StreamEvent } from "@bodysense/contracts";
import { refreshBrowserAccessToken } from "./support/auth";

test.describe.configure({ timeout: 100_000 });
const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const SAID = "我觉得是久坐导致的。";
const QUOTE = "我觉得是久坐导致的";

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`suggest-${crypto.randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const token = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${token}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: { gender: "male", birth_date: "2000-01-01" },
  });
  expect(profile.ok()).toBeTruthy();
  return headers;
}

function events(text: string): StreamEvent[] {
  return text
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => parseStreamEvent(JSON.parse(line.slice(5).trim())));
}

async function hypotheses(
  request: APIRequestContext,
  headers: Record<string, string>,
) {
  const response = await request.get(
    `${api}/api/v1/body-state/hypothesis-records`,
    { headers },
  );
  expect(response.ok(), await response.text()).toBeTruthy();
  return (await response.json()) as {
    current_revision: number;
    hypotheses: Array<{ id: string; statement: string }>;
  };
}

/** Say the words in chat and wait for Go's bound suggestion card. */
async function startSuggestion(page: Page) {
  await page.goto("/consultation?view=chat");
  const composer = page.getByPlaceholder("和 BodySense 说说你的身体感受…");
  // The lazy chat chunk can take longer than the default assertion budget on a cold Vite server.
  await expect(composer).toBeVisible({ timeout: 30_000 });
  const responsePromise = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/consultation-runs") &&
      response.request().method() === "POST",
  );
  await composer.fill(SAID);
  await page.getByRole("button", { name: "发送", exact: true }).click();
  const response = await responsePromise;
  expect(response.ok()).toBeTruthy();
  const stream = events(await response.text());
  const pending = stream.find(
    (event) => event.type === "state.interaction.required",
  );
  if (!pending || pending.type !== "state.interaction.required")
    throw new Error(
      `Missing BodyState suggestion: ${JSON.stringify(stream.map((e) => [e.type, e.payload]))}`,
    );
  expect(pending.payload.question.purpose).toBe("body_state_suggestion");
  const card = page
    .getByTestId("pending-interaction-composer")
    .getByTestId("body-state-suggestion-card");
  await expect(card).toBeVisible();
  await expect(card.getByTestId("body-state-suggestion-quote")).toContainText(
    QUOTE,
  );
  return { card, interactionId: pending.payload.interaction_id };
}

test("accepting a suggestion shows saved only after Go's receipt, for the hypothesis it wrote", async ({
  page,
  request,
}, testInfo) => {
  const headers = await register(page, request);
  const { card, interactionId } = await startSuggestion(page);
  expect((await hypotheses(request, headers)).hypotheses).toEqual([]);
  await page.screenshot({
    path: testInfo.outputPath("body-state-suggestion-ask.png"),
    fullPage: true,
  });

  // Hold Go's answer stream: until the receipt reaches the browser nothing may say saved.
  let receiptStream = "";
  await page.route(`**/interrupts/${interactionId}/answers`, async (route) => {
    const response = await route.fetch();
    receiptStream = await response.text();
    const ledger = await hypotheses(request, headers);
    expect(ledger.hypotheses).toEqual([
      expect.objectContaining({ statement: QUOTE }),
    ]);
    await expect(page.getByTestId("body-state-suggestion-saved")).toHaveCount(
      0,
    );
    await route.fulfill({ response, body: receiptStream });
  });
  await card.getByRole("button", { name: "记录为我的猜测" }).click();

  const saved = page.getByTestId("body-state-suggestion-saved");
  await expect(saved).toBeVisible();
  await expect(saved).toContainText("已保存为你的猜测");
  await expect(saved).toContainText(QUOTE);
  const answered = events(receiptStream).find(
    (event) => event.type === "state.interaction.answered",
  );
  if (!answered || answered.type !== "state.interaction.answered")
    throw new Error("Missing answered milestone");
  const receipt = answered.payload.body_state_receipt;
  if (!receipt || receipt.kind !== "hypothesis")
    throw new Error("Missing hypothesis receipt");
  const ledger = await hypotheses(request, headers);
  expect(ledger.hypotheses).toHaveLength(1);
  expect(ledger.hypotheses[0].id).toBe(receipt.hypothesis_id);
  expect(ledger.current_revision).toBe(receipt.revision);

  // Chat history keeps the same receipt after a reload.
  await page.unroute(`**/interrupts/${interactionId}/answers`);
  await page.reload();
  await expect(page.getByTestId("body-state-suggestion-saved")).toBeVisible({
    timeout: 30_000,
  });
  await page.screenshot({
    path: testInfo.outputPath("body-state-suggestion-saved.png"),
    fullPage: true,
  });
});

test("declining a suggestion writes nothing", async ({ page, request }) => {
  const headers = await register(page, request);
  const { card, interactionId } = await startSuggestion(page);
  const responsePromise = page.waitForResponse((response) =>
    response.url().includes(`/interrupts/${interactionId}/answers`),
  );
  await card.getByRole("button", { name: "不记录", exact: true }).click();
  const answered = events(await (await responsePromise).text()).find(
    (event) => event.type === "state.interaction.answered",
  );
  if (!answered || answered.type !== "state.interaction.answered")
    throw new Error("Missing answered milestone");
  expect(answered.payload.body_state_receipt).toBeNull();
  await expect(
    page.getByTestId("body-state-suggestion-declined"),
  ).toBeVisible();
  await expect(page.getByTestId("body-state-suggestion-saved")).toHaveCount(0);
  const ledger = await hypotheses(request, headers);
  expect(ledger.hypotheses).toEqual([]);
  expect(ledger.current_revision).toBe(0);
});

test("editing declines the suggestion and returns the words to the composer", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  const { card } = await startSuggestion(page);
  await card.getByRole("button", { name: "改一改再说" }).click();
  await expect(
    page.getByTestId("body-state-suggestion-declined"),
  ).toBeVisible();
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toHaveValue(QUOTE);
  expect((await hypotheses(request, headers)).hypotheses).toEqual([]);
});
