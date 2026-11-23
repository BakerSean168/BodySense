import { expect, test } from "@playwright/test";

import {
  api,
  events,
  getJSON,
  register,
  startAsk,
} from "./support/askUserBodyFacts";

import { refreshBrowserAccessToken } from "./support/auth";

import { requiresDeterministicModel } from "./support/consultation";

import { isTerminal, readRunEvents } from "./support/runEvents";

test.describe.configure({ timeout: 100_000 });

// A real model decides its own reply, so this cannot be seeded through the API.
requiresDeterministicModel();

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
