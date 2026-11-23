import { expect, test } from "@playwright/test";

import {
  anotherOwner,
  api,
  getJSON,
  register,
  startAsk,
} from "./support/askUserBodyFacts";

import { requiresDeterministicModel } from "./support/consultation";

test.describe.configure({ timeout: 100_000 });

// A real model decides its own reply, so this cannot be seeded through the API.
requiresDeterministicModel();

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
      // V2 Chat renders the Penpot title "对话 / 今天", not the generic h2.
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
  // Penpot Home has no app bar: the theme lives in Person → 账户菜单 → 设置.
  const dock = page.getByRole("navigation", { name: "身体工作区导航" });
  await dock.getByRole("button", { name: "个人档案", exact: true }).click();
  await page.locator(".bc-person-identity").click();
  await page.getByRole("menuitem", { name: "设置" }).click();
  await page.getByRole("radio", { name: "深色" }).click();
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await dock.getByRole("button", { name: "对话", exact: true }).click();
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
