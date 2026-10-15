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

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`intake-${crypto.randomUUID()}@example.test`);
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

async function diagnose(
  request: APIRequestContext,
  headers: Record<string, string>,
  conversationId: string,
) {
  const response = await request.post(
    `${api}/api/v1/consultations/${conversationId}/diagnosis`,
    { headers, timeout: 60_000 },
  );
  const body = await response.text();
  expect(response.ok(), body).toBeTruthy();
  return { body, diagnosis: JSON.parse(body) as { candidates: unknown[] } };
}

test("v3 symptom report asks the bound safety screen; answering it makes Diagnosis reachable", async ({
  page,
  request,
}, testInfo) => {
  const headers = await register(page, request);

  // The user reports a present symptom in the browser.
  await page.goto("/consultation?view=chat");
  const composer = page.getByPlaceholder("和 BodySense 说说你的身体感受…");
  await expect(composer).toBeVisible();
  const runResponse = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/consultation-runs") &&
      response.request().method() === "POST",
  );
  await composer.fill("我昨天肩膀不舒服。");
  await page.getByRole("button", { name: "发送", exact: true }).click();
  const response = await runResponse;
  expect(response.ok()).toBeTruthy();
  const stream = events(await response.text());
  expect(stream.some((event) => event.type === "stream.error")).toBe(false);

  const stored = stream.find(
    (event) =>
      event.type === "tool.result" &&
      event.payload.tool === "record_body_state",
  );
  if (!stored || stored.type !== "tool.result")
    throw new Error("Missing record_body_state receipt");
  const receipt = stored.payload.result as {
    status: string;
    receipt: { fact_ids: string[] };
  };
  expect(receipt.status).toBe("persisted");
  const pending = stream.find(
    (event) => event.type === "state.interaction.required",
  );
  if (!pending || pending.type !== "state.interaction.required")
    throw new Error("Missing safety screen interaction");
  const question = pending.payload.question;
  expect(question.purpose).toBe("symptom_intake");
  // Go bound the screen to the exact fact it committed this turn.
  expect(question.state_binding?.body_fact_id).toBe(
    receipt.receipt.fact_ids[0],
  );
  expect(question.fields?.map((field) => field.key)).toEqual([
    "safety_signals",
  ]);

  // Control: the symptom is committed but not yet screened, so Diagnosis
  // abstains because structured safety coverage is incomplete.
  const before = await diagnose(request, headers, pending.ids.conversation_id!);
  expect(before.diagnosis.candidates).toEqual([]);
  expect(before.body).toContain("结构化安全信息尚未完整采集");

  const card = page.getByTestId("pending-interaction-composer");
  await expect(card).toBeVisible();
  await expect(composer).toHaveCount(0);
  await expect(card.getByLabel("以上均无")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("symptom-safety-screen-pending.png"),
    fullPage: true,
  });

  const answerResponse = page.waitForResponse((answer) =>
    answer
      .url()
      .includes(`/interrupts/${pending.payload.interaction_id}/answers`),
  );
  await card.getByLabel("以上均无").check();
  await card.getByRole("button", { name: "提交", exact: true }).click();
  const answered = await answerResponse;
  const answeredBody = await answered.text();
  expect(answered.ok(), answeredBody).toBeTruthy();
  const accepted = events(answeredBody);
  expect(accepted.some((event) => event.type === "stream.error")).toBe(false);
  expect(accepted.some((event) => event.type === "stream.done")).toBe(true);
  await expect(card).toHaveCount(0);
  await expect(composer).toBeEnabled();

  // The answer screened the existing symptom; it did not add a second one.
  const ledger = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  expect(ledger.ok()).toBeTruthy();
  const records = (await ledger.json()) as {
    facts: Array<{ id: string; kind: string; statement: string }>;
  };
  const symptoms = records.facts.filter((fact) => fact.kind === "symptom");
  expect(symptoms).toHaveLength(1);
  expect(symptoms[0]).toMatchObject({
    id: receipt.receipt.fact_ids[0],
    statement: "我昨天肩膀不舒服。",
  });

  const after = await diagnose(request, headers, pending.ids.conversation_id!);
  expect(after.diagnosis.candidates).toHaveLength(1);
  expect(after.body).not.toContain("结构化安全信息尚未完整采集");
});
