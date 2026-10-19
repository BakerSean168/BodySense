import {
  expect,
  test,
  type APIRequestContext,
  type Browser,
  type Page,
} from "@playwright/test";
import { parseStreamEvent, type StreamEvent } from "@bodysense/contracts";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import {
  conversationIdFromUrl,
  isTerminal,
  readRunEvents,
} from "./support/runEvents";

test.describe.configure({ timeout: 100_000 });
const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function register(
  page: Page,
  request: APIRequestContext,
  email = `ask-${crypto.randomUUID()}@example.test`,
) {
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const token = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${token}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "male", birth_date: "2000-01-01" }),
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

async function startAsk(
  page: Page,
  headers: Record<string, string>,
  marker = "E2E_BODYFACT_ASK_SELF",
) {
  await page.goto("/consultation?view=chat");
  const composer = page.getByPlaceholder("和 BodySense 说说你的身体感受…");
  // The lazy chat chunk can take longer than the default assertion budget on a cold Vite server.
  await expect(composer).toBeVisible({ timeout: 30_000 });
  const responsePromise = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/consultation-runs") &&
      response.request().method() === "POST",
  );
  await composer.fill(marker);
  await page.getByRole("button", { name: "发送", exact: true }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  await expect(page.getByTestId("pending-interaction-composer")).toBeVisible();
  await expect(composer).toHaveCount(0);
  await expect(page).toHaveURL(/\/consultation\/[0-9a-f-]{36}/);
  const conversationId = conversationIdFromUrl(page.url());
  const threadURL = `${api}/api/v1/consultations/${conversationId}/thread`;
  const request = page.context().request;
  const runId: string = (await getJSON(request, threadURL, headers))
    .active_turn_run_id;
  expect(runId).toBeTruthy();
  // The paused segment ends with its own stream.done (see support/runEvents).
  const stream = await readRunEvents(request, headers, conversationId, runId, {
    until: (events) => events.some(isTerminal),
  });
  const pending = stream.find(
    (event) => event.type === "state.interaction.required",
  );
  expect(pending?.type).toBe("state.interaction.required");
  if (!pending || pending.type !== "state.interaction.required")
    throw new Error("Missing pending interaction");
  expect(pending.ids.conversation_id).toBe(conversationId);
  expect(pending.ids.run_id).toBe(runId);
  const interactionId = pending.payload.interaction_id;
  return {
    conversationId,
    runId,
    interactionId,
    stream,
    answerURL: `${api}/api/v1/consultations/${conversationId}/interrupts/${interactionId}/answers`,
    threadURL,
  };
}

async function getJSON(
  request: APIRequestContext,
  url: string,
  headers: Record<string, string>,
) {
  const response = await request.get(url, { headers });
  expect(response.ok(), await response.text()).toBeTruthy();
  return response.json();
}

async function anotherOwner(browser: Browser, baseURL: string | undefined) {
  const context = await browser.newContext({ baseURL });
  const page = await context.newPage();
  const headers = await register(page, context.request);
  return { context, headers };
}

test("second device and second tab recover the same pending Ask User and resume its original Run", async ({
  page,
  browser,
  baseURL,
}) => {
  const email = `ask-devices-${crypto.randomUUID()}@example.test`;
  const headers = await register(page, page.context().request, email);
  const ask = await startAsk(page, headers);
  // A fresh context logs in independently: no shared cookies or browser storage.
  const device = await browser.newContext({ baseURL });
  try {
    const remote = await device.newPage();
    await remote.goto("/login");
    await remote.getByLabel("邮箱地址").fill(email);
    await remote.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
    await remote.getByRole("button", { name: "登录", exact: true }).click();
    await remote.waitForURL(/\/consultation/);
    const remoteHeaders = {
      Authorization: `Bearer ${await refreshBrowserAccessToken(remote, api)}`,
    };
    const tab = await page.context().newPage();
    for (const viewer of [remote, tab]) {
      await viewer.goto("/consultation?view=chat");
      await expect(viewer).toHaveURL(
        new RegExp(`/consultation/${ask.conversationId}`),
      );
      await expect(
        viewer.getByTestId("pending-interaction-composer"),
      ).toBeVisible();
      await expect(
        viewer.getByPlaceholder("和 BodySense 说说你的身体感受…"),
      ).toHaveCount(0);
    }
    const thread = await getJSON(device.request, ask.threadURL, remoteHeaders);
    expect(thread.active_turn_run_id).toBe(ask.runId);
    expect(thread.pending_interactions).toEqual([
      expect.objectContaining({ id: ask.interactionId, run_id: ask.runId }),
    ]);
    const parallel = await device.request.post(
      `${api}/api/v1/consultation-runs`,
      {
        headers: remoteHeaders,
        data: {
          conversationId: ask.conversationId,
          clientMessageId: crypto.randomUUID(),
          requestId: crypto.randomUUID(),
          message: {
            role: "user",
            parts: [{ type: "text", text: "No second Run" }],
          },
        },
      },
    );
    expect(parallel.status()).toBe(409);
    const pending = remote.getByTestId("pending-interaction-composer");
    await pending.getByPlaceholder("输入你的回答...").fill("我没有肩痛。");
    const responsePromise = remote.waitForResponse((response) =>
      response.url().includes(`/interrupts/${ask.interactionId}/answers`),
    );
    await pending.getByRole("button", { name: "提交", exact: true }).click();
    const response = await responsePromise;
    expect(response.ok()).toBeTruthy();
    const resumed = await readRunEvents(
      device.request,
      remoteHeaders,
      ask.conversationId,
      ask.runId,
      { afterSeq: ask.stream.at(-1)!.seq, until: (e) => e.some(isTerminal) },
    );
    expect(
      resumed.filter((event) => event.type === "run.resumed"),
    ).toHaveLength(1);
    expect(resumed.every((event) => event.ids.run_id === ask.runId)).toBe(true);
    const command = response.request().postDataJSON();
    // A stale device cannot apply a competing answer after the other device commits.
    const conflicting = await page.context().request.post(ask.answerURL, {
      headers,
      data: {
        ...command,
        requestId: crypto.randomUUID(),
        answer: { text: "我有肩痛。" },
      },
    });
    expect(conflicting.status()).toBe(409);
    for (const viewer of [page, tab, remote]) {
      await viewer.reload();
      await expect(
        viewer.getByTestId("pending-interaction-composer"),
      ).toHaveCount(0);
      await expect(
        viewer.getByPlaceholder("和 BodySense 说说你的身体感受…"),
      ).toBeEnabled();
    }
    const runs = await getJSON(
      device.request,
      `${api}/api/v1/conversations/${ask.conversationId}/runs`,
      remoteHeaders,
    );
    expect(runs.runs).toEqual([
      expect.objectContaining({ id: ask.runId, status: "completed" }),
    ]);
    const ledger = await getJSON(
      device.request,
      `${api}/api/v1/body-state/records`,
      remoteHeaders,
    );
    expect(ledger.current_revision).toBe(1);
    expect(ledger.facts).toHaveLength(1);
    expect(ledger.facts[0].statement).toBe("我没有肩痛。");
    await tab.close();
  } finally {
    await device.close();
  }
});

test("Ask User reload preserves the original Run; accepted answer becomes one source-bound fact", async ({
  page,
  request,
}, testInfo) => {
  const headers = await register(page, request);
  const ask = await startAsk(page, headers);
  const before = await getJSON(request, ask.threadURL, headers);
  expect(before.pending_interactions).toHaveLength(1);
  expect(before.active_turn_run_id).toBe(ask.runId);
  expect(
    (await getJSON(request, `${api}/api/v1/body-state/records`, headers)).facts,
  ).toEqual([]);

  const parallel = await request.post(`${api}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId: ask.conversationId,
      clientMessageId: crypto.randomUUID(),
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: "Do not start a second run." }],
      },
    },
  });
  expect(parallel.status()).toBe(409);
  await page.reload();
  const pending = page.getByTestId("pending-interaction-composer");
  await expect(
    pending.getByText("Describe your own current symptoms.", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath("ask-user-composer-pending.png"),
    fullPage: true,
  });

  const responsePromise = page.waitForResponse((response) =>
    response.url().includes(`/interrupts/${ask.interactionId}/answers`),
  );
  await pending.getByPlaceholder("输入你的回答...").fill("我没有肩痛。");
  await pending.getByRole("button", { name: "提交", exact: true }).click();
  const response = await responsePromise;
  expect(response.ok()).toBeTruthy();
  const accepted = await readRunEvents(
    request,
    headers,
    ask.conversationId,
    ask.runId,
    { afterSeq: ask.stream.at(-1)!.seq, until: (e) => e.some(isTerminal) },
  );
  expect(accepted.some((event) => event.type === "stream.error")).toBe(false);
  expect(accepted.every((event) => event.ids.run_id === ask.runId)).toBe(true);
  expect(accepted[0].seq).toBeGreaterThan(ask.stream.at(-1)!.seq);
  expect(accepted.filter((event) => event.type === "run.resumed")).toHaveLength(
    1,
  );
  const stored = accepted.find(
    (event) =>
      event.type === "tool.result" &&
      event.payload.tool === "record_body_state",
  );
  expect(stored?.type).toBe("tool.result");
  const originalCommand = response.request().postDataJSON();
  await expect(page.getByTestId("pending-interaction-composer")).toHaveCount(0);
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeEnabled();

  const ledger = await getJSON(
    request,
    `${api}/api/v1/body-state/records`,
    headers,
  );
  expect(ledger.current_revision).toBe(1);
  expect(ledger.facts).toHaveLength(1);
  expect(ledger.facts[0]).toMatchObject({
    statement: "我没有肩痛。",
    details: { presence: "absent" },
  });
  const thread = await getJSON(request, ask.threadURL, headers);
  expect(thread.pending_interactions).toEqual([]);
  expect(thread.interaction_history).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        id: ask.interactionId,
        run_id: ask.runId,
        status: "answered",
      }),
    ]),
  );
  const runs = await getJSON(
    request,
    `${api}/api/v1/conversations/${ask.conversationId}/runs`,
    headers,
  );
  expect(runs.runs).toHaveLength(1);
  expect(runs.runs[0]).toMatchObject({ id: ask.runId, status: "completed" });
  expect(thread.messages).toHaveLength(4);
  const replay = await request.post(ask.answerURL, {
    headers,
    data: originalCommand,
  });
  expect(replay.ok(), await replay.text()).toBeTruthy();
  const replayEvents = events(await replay.text());
  expect(replayEvents[0].type).toBe("state.interaction.answered");
  expect(replayEvents.map((event) => event.seq)).toEqual(
    accepted.map((event) => event.seq),
  );
  const again = await getJSON(
    request,
    `${api}/api/v1/body-state/records`,
    headers,
  );
  expect(again.current_revision).toBe(1);
  expect(again.facts[0].id).toBe(ledger.facts[0].id);
  expect(
    (await getJSON(request, ask.threadURL, headers)).messages,
  ).toHaveLength(4);

  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "个人档案", exact: true })
    .click();
  await expect(
    page
      .getByTestId("body-fact-records")
      .getByText("我没有肩痛。", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page
      .getByTestId("body-fact-records")
      .getByText("我没有肩痛。", { exact: true }),
  ).toBeVisible();
});

test("Ask User rejects foreign owners, forged choices and cancelled answers without creating facts", async ({
  page,
  request,
  browser,
  baseURL,
}) => {
  const headers = await register(page, request);
  const ask = await startAsk(page, headers, "E2E_BODYFACT_ASK_CHOICE");
  const other = await anotherOwner(browser, baseURL);
  try {
    const denied = await request.post(ask.answerURL, {
      headers: other.headers,
      data: {
        requestId: crypto.randomUUID(),
        answer: { text: "left", selected: ["left"] },
      },
    });
    expect(denied.status()).toBe(404);
  } finally {
    await other.context.close();
  }
  for (const answer of [
    { text: "right", selected: ["left"] },
    { text: "both", selected: ["both"] },
    { text: "我有肩痛。", selected: ["我有肩痛。"], is_custom: true },
  ]) {
    const denied = await request.post(ask.answerURL, {
      headers,
      data: { requestId: crypto.randomUUID(), answer },
    });
    expect(denied.status()).toBe(400);
  }
  let thread = await getJSON(request, ask.threadURL, headers);
  expect(thread.pending_interactions).toHaveLength(1);
  expect(thread.messages).toHaveLength(2);
  expect(
    (await getJSON(request, `${api}/api/v1/body-state/records`, headers)).facts,
  ).toEqual([]);

  await page.getByRole("button", { name: "停止", exact: true }).click();
  await expect(page.getByTestId("pending-interaction-composer")).toHaveCount(0);
  const stale = await request.post(ask.answerURL, {
    headers,
    data: {
      requestId: crypto.randomUUID(),
      answer: { text: "left", selected: ["left"] },
    },
  });
  expect(stale.status()).toBe(409);
  thread = await getJSON(request, ask.threadURL, headers);
  expect(thread.messages).toHaveLength(2);
  expect(
    (await getJSON(request, `${api}/api/v1/body-state/records`, headers))
      .current_revision,
  ).toBe(0);
});

test("Ask User failed submission preserves its draft and retry identity in the replacement composer", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  const ask = await startAsk(page, headers);
  const requests: unknown[] = [];
  let attempts = 0;
  await page.route(
    `**/interrupts/${ask.interactionId}/answers`,
    async (route) => {
      requests.push(route.request().postDataJSON());
      attempts += 1;
      if (attempts === 1) {
        await route.fulfill({
          status: 503,
          contentType: "application/json",
          body: JSON.stringify({
            error: {
              code: "UNAVAILABLE",
              message: "Synthetic temporary failure",
            },
          }),
        });
      } else {
        await route.continue();
      }
    },
  );
  const pending = page.getByTestId("pending-interaction-composer");
  await pending.getByPlaceholder("输入你的回答...").fill("我没有肩痛。");
  await pending.getByRole("button", { name: "提交", exact: true }).click();
  await expect(
    pending.getByText(
      /Synthetic temporary failure|提交失败|503|Unavailable|unavailable/i,
    ),
  ).toBeVisible();
  await expect(pending.getByPlaceholder("输入你的回答...")).toHaveValue(
    "我没有肩痛。",
  );
  expect(
    (await getJSON(request, ask.threadURL, headers)).pending_interactions,
  ).toHaveLength(1);
  await pending.getByRole("button", { name: "提交", exact: true }).click();
  await expect(pending).toHaveCount(0);
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeEnabled();
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
  expect(
    (await getJSON(request, `${api}/api/v1/body-state/records`, headers)).facts,
  ).toHaveLength(1);
});

test("Ask User mobile replacement composer fits the sheet in both themes and submits without a new Run", async ({
  page,
  request,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const headers = await register(page, request);
  const ask = await startAsk(page, headers);
  const pending = page.getByTestId("pending-interaction-composer");
  const input = pending.getByPlaceholder("输入你的回答...");
  const submit = pending.getByRole("button", { name: "提交", exact: true });
  await input.fill("我没有肩痛。");
  const assertBounds = async () => {
    await expect(input).toBeVisible();
    await expect(submit).toBeVisible();
    const result = await pending.evaluate((node) => {
      const sheet = node.closest(".bc-business-sheet");
      const root = sheet?.getBoundingClientRect();
      const bounds = node.getBoundingClientRect();
      const button = node
        .querySelector("button.ask-user-submit")
        ?.getBoundingClientRect();
      // V2 Chat now renders a contextual title + local clock, not the generic h2.
      const title = sheet?.querySelector(".bc-chat-context-title");
      if (!(title instanceof HTMLElement)) {
        throw new Error("Missing V2 Chat title in the mobile sheet");
      }
      return {
        left: bounds.left,
        right: bounds.right,
        bottom: bounds.bottom,
        sheetBottom: root?.bottom ?? 0,
        buttonBottom: button?.bottom ?? 0,
        scrollWidth: document.documentElement.scrollWidth,
        viewport: innerWidth,
        color: getComputedStyle(node.querySelector(".ask-user-question")!)
          .color,
        headingColor: getComputedStyle(title).color,
      };
    });
    expect(result.left).toBeGreaterThanOrEqual(0);
    expect(result.headingColor).toBe(result.color);
    expect(result.right).toBeLessThanOrEqual(391);
    expect(result.scrollWidth).toBeLessThanOrEqual(result.viewport);
    expect(result.bottom).toBeLessThanOrEqual(result.sheetBottom + 1);
    expect(result.buttonBottom).toBeLessThanOrEqual(result.bottom + 1);
    return result.color;
  };
  const lightColor = await assertBounds();
  await page.screenshot({
    path: testInfo.outputPath("ask-user-mobile-light.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "切换深色", exact: true }).click();
  const darkColor = await assertBounds();
  expect(darkColor).not.toBe(lightColor);
  await expect(input).toHaveValue("我没有肩痛。");
  await page.screenshot({
    path: testInfo.outputPath("ask-user-mobile-dark.png"),
    fullPage: true,
  });
  await submit.click();
  await expect(pending).toHaveCount(0);
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeEnabled();
  const runs = await getJSON(
    request,
    `${api}/api/v1/conversations/${ask.conversationId}/runs`,
    headers,
  );
  expect(runs.runs).toHaveLength(1);
  expect(runs.runs[0].id).toBe(ask.runId);
  const facts = await getJSON(
    request,
    `${api}/api/v1/body-state/records`,
    headers,
  );
  expect(facts.facts).toHaveLength(1);
});
