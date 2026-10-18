import {
  expect,
  test,
  type Page,
  type APIRequestContext,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";

// These cases include registration, real Go/Python/DB calls and full reloads.
// Keep each expectation's normal timeout; allow a cold 3D/browser startup on
// shared CI hosts without exhausting the entire case's default 30 seconds.
test.describe.configure({ timeout: 90_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function register(page: Page, request: APIRequestContext) {
  const email = `body-fact-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
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

function parseEvents(text: string): Array<{
  type: string;
  ids: Record<string, string>;
  payload: Record<string, any>;
}> {
  return text
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => JSON.parse(line.slice(5).trim()));
}

async function send(
  request: APIRequestContext,
  headers: Record<string, string>,
  text: string,
  conversationId: string | null = null,
) {
  const command = {
    conversationId,
    clientMessageId: crypto.randomUUID(),
    requestId: crypto.randomUUID(),
    message: { role: "user", parts: [{ type: "text", text }] },
  };
  const response = await request.post(`${api}/api/v1/consultation-runs`, {
    headers,
    data: command,
    timeout: 60_000,
  });
  expect(response.ok()).toBeTruthy();
  const events = parseEvents(await response.text());
  expect(events.filter((event) => event.type === "stream.error")).toEqual([]);
  const run = events.find((event) => event.type === "run.started");
  expect(run).toBeDefined();
  return { events, command, conversationId: run!.ids.conversation_id };
}

test("Person Records directly saves real facts and retains them after refresh", async ({
  page,
  request,
}, testInfo) => {
  const headers = await register(page, request);
  await page.goto("/consultation?view=state");
  const records = page.getByTestId("body-fact-records");
  await expect(records).toBeVisible();
  await expect(records.getByText(/还没有身体记录/)).toBeVisible();
  await records.getByRole("button", { name: "补充记录" }).click();
  await records.getByLabel("你的描述").fill("我今天步行二十分钟");
  await records.getByLabel("记录类别").selectOption("activity");
  await records.getByRole("button", { name: "保存记录" }).click();
  await expect(records.getByText("已保存你的陈述。")).toBeVisible();
  await expect(
    records.getByText("我今天步行二十分钟", { exact: true }),
  ).toBeVisible();
  const persisted = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  const body = await persisted.json();
  expect(body.facts).toHaveLength(1);
  expect(body.facts[0]).toMatchObject({
    kind: "activity",
    statement: "我今天步行二十分钟",
    temporal: { precision: "unknown" },
  });
  await page.reload();
  await expect(
    records.getByText("我今天步行二十分钟", { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("person-records.png"),
    fullPage: true,
  });
});

test("Python extraction -> Go receipt -> PostgreSQL -> actual Person Habits survives exact replay", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  const turn = await send(request, headers, "我每晚睡七小时。");
  const saved = turn.events.find(
    (event) =>
      event.type === "tool.result" &&
      event.payload.tool === "record_body_state",
  );
  expect(saved?.payload.result.status).toBe("persisted");
  const receipt = saved!.payload.result.receipt;
  expect(receipt.id).toMatch(/^[0-9a-f-]{36}$/);
  expect(receipt.fact_ids).toHaveLength(1);
  const replay = await request.post(`${api}/api/v1/consultation-runs`, {
    headers,
    data: turn.command,
  });
  expect(replay.ok()).toBeTruthy();
  const replayed = parseEvents(await replay.text()).find(
    (event) =>
      event.type === "tool.result" &&
      event.payload.tool === "record_body_state",
  );
  expect(replayed?.payload.result.receipt.id).toBe(receipt.id);
  const response = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  const ledger = await response.json();
  expect(ledger.facts).toHaveLength(1);
  expect(ledger.facts[0].statement).toBe("我每晚睡七小时。");
  expect(ledger.current_revision).toBe(receipt.revision);
  await page.goto(`/consultation/${turn.conversationId}?view=state`);
  await page.getByLabel("档案分类").selectOption("habits");
  await expect(
    page
      .getByTestId("body-fact-records")
      .getByText("我每晚睡七小时。", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page
      .getByTestId("body-fact-records")
      .getByText("我每晚睡七小时。", { exact: true }),
  ).toBeVisible();
});

test("negative self-report stays negative and third-party talk does not create a fact", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  const turn = await send(request, headers, "我没有肩痛。");
  const first = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  const before = await first.json();
  expect(before.facts).toHaveLength(1);
  expect(before.facts[0].details.presence).toBe("absent");
  await send(request, headers, "我的朋友每晚睡七小时。", turn.conversationId);
  const second = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  const after = await second.json();
  expect(after.facts).toEqual(before.facts);
  expect(after.current_revision).toBe(before.current_revision);
});

test("Person Records narrow viewport keeps live form and records inside the sheet", async ({
  page,
  request,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const headers = await register(page, request);
  await page.goto("/consultation?view=state");
  const records = page.getByTestId("body-fact-records");
  await expect(records).toBeVisible();
  await records.getByRole("button", { name: "补充记录" }).click();
  const text =
    "我每天散步二十分钟，并希望完整保留这段较长的本人原始描述而不截断";
  await records.getByLabel("你的描述").fill(text);
  await records.getByRole("button", { name: "保存记录" }).click();
  await expect(records.getByText("已保存你的陈述。")).toBeVisible();
  const statement = records.getByText(text, { exact: true });
  await expect(statement).toBeVisible();
  const box = await statement.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  const rootSize = await page.evaluate(() => ({
    width: document.documentElement.clientWidth,
    scroll: document.documentElement.scrollWidth,
  }));
  expect(rootSize.scroll).toBeLessThanOrEqual(rootSize.width);
  const persisted = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  expect((await persisted.json()).facts[0].statement).toBe(text);
  await page.screenshot({
    path: testInfo.outputPath("person-records-mobile.png"),
    fullPage: true,
  });
});

test("Person Records corrections and withdrawal use exact targets and durable replay", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  const initial = await request.post(`${api}/api/v1/body-state/records`, {
    headers,
    data: {
      idempotency_key: crypto.randomUUID(),
      expected_revision: 0,
      facts: [
        {
          kind: "activity",
          statement: "我昨天跑步三十分钟",
          body_region_ids: [],
          temporal: { precision: "unknown" },
          details: {},
        },
      ],
    },
  });
  expect(initial.ok()).toBeTruthy();
  const originalId = (await initial.json()).fact_ids[0];
  await page.goto("/consultation?view=state");
  const records = page.getByTestId("body-fact-records");
  await expect(
    records.getByText("我昨天跑步三十分钟", { exact: true }),
  ).toBeVisible();
  // Penpot Records: amendments live in the statement's detail scene.
  await records
    .getByRole("button", { name: "查看记录：我昨天跑步三十分钟" })
    .click();
  await records.getByRole("button", { name: "更正", exact: true }).click();
  await records
    .getByLabel("更正后的完整陈述")
    .fill("我昨天只是散步十分钟，原来说错了");
  await records.getByRole("button", { name: "保存更正" }).click();
  await expect(
    records.getByText("更正已保存，原陈述保留在历史中。"),
  ).toBeVisible();
  await expect(
    records.getByText("我昨天只是散步十分钟，原来说错了", { exact: true }),
  ).toBeVisible();
  await expect(
    records.getByText("我昨天跑步三十分钟", { exact: true }),
  ).toHaveCount(0);
  const corrected = await (
    await request.get(`${api}/api/v1/body-state/records`, { headers })
  ).json();
  expect(corrected.facts).toHaveLength(1);
  expect(corrected.facts[0].supersedes_fact_id).toBe(originalId);
  expect(corrected.current_revision).toBe(2);
  const currentId = corrected.facts[0].id;
  await records
    .getByRole("button", { name: "查看记录：我昨天只是散步十分钟，原来说错了" })
    .click();
  await expect(records.getByText("更正后的陈述")).toBeVisible();
  await records.getByRole("button", { name: "撤回陈述" }).click();
  await expect(records.getByText(/这不是隐私数据删除/)).toBeVisible();
  const outgoing = page.waitForRequest(
    (req) =>
      req.url().endsWith(`/records/${currentId}/withdraw`) &&
      req.method() === "POST",
  );
  const incoming = page.waitForResponse(
    (res) =>
      res.url().endsWith(`/records/${currentId}/withdraw`) &&
      res.request().method() === "POST",
  );
  await records.getByRole("button", { name: "确认撤回" }).click();
  const originalRequest = await outgoing;
  const originalReceipt = await (await incoming).json();
  await expect(records.getByText(/陈述已撤回，历史保留/)).toBeVisible();
  await expect(records.getByText(/还没有身体记录/)).toBeVisible();
  const replay = await request.post(
    `${api}/api/v1/body-state/records/${currentId}/withdraw`,
    { headers, data: originalRequest.postDataJSON() },
  );
  expect(replay.ok()).toBeTruthy();
  expect((await replay.json()).id).toBe(originalReceipt.id);
  const final = await (
    await request.get(`${api}/api/v1/body-state/records`, { headers })
  ).json();
  expect(final.facts).toEqual([]);
  expect(final.current_revision).toBe(3);
  await page.reload();
  await expect(records.getByText(/还没有身体记录/)).toBeVisible();
});

test("BodyFact tool identity stays distinct across runs and users in durable thread projection", async ({
  page,
  request,
  browser,
}) => {
  const headers = await register(page, request);
  const first = await send(request, headers, "我每晚睡七小时。");
  await send(request, headers, "我没有肩痛。", first.conversationId);
  const response = await request.get(
    `${api}/api/v1/consultations/${first.conversationId}/thread`,
    { headers },
  );
  expect(response.ok(), await response.text()).toBeTruthy();
  const projection = await response.json();
  const calls = projection.tool_calls.filter(
    (item: { tool_name: string }) => item.tool_name === "record_body_state",
  );
  expect(calls).toHaveLength(2);
  const ids = calls.map((item: { tool_call_id: string }) => item.tool_call_id);
  expect(new Set(ids).size).toBe(2);
  for (const call of calls) {
    expect(call.tool_call_id).toBe(`bodyfact-intake-v1:${call.run_id}`);
    expect(call.result.status).toBe("persisted");
  }
  const secondContext = await browser.newContext();
  try {
    const otherPage = await secondContext.newPage();
    const otherHeaders = await register(otherPage, secondContext.request);
    const other = await send(
      secondContext.request,
      otherHeaders,
      "我每晚睡七小时。",
    );
    const otherResponse = await secondContext.request.get(
      `${api}/api/v1/consultations/${other.conversationId}/thread`,
      { headers: otherHeaders },
    );
    expect(otherResponse.ok(), await otherResponse.text()).toBeTruthy();
    const otherCalls = (await otherResponse.json()).tool_calls.filter(
      (item: { tool_name: string }) => item.tool_name === "record_body_state",
    );
    expect(otherCalls).toHaveLength(1);
    expect(ids).not.toContain(otherCalls[0].tool_call_id);
    const forbidden = await secondContext.request.get(
      `${api}/api/v1/consultations/${first.conversationId}/thread`,
      { headers: otherHeaders },
    );
    expect(forbidden.status()).toBe(404);
  } finally {
    await secondContext.close();
  }
});
