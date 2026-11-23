import {
  expect,
  type APIRequestContext,
  type Browser,
  type Page,
} from "@playwright/test";

import { parseStreamEvent, type StreamEvent } from "@bodysense/contracts";

import { refreshBrowserAccessToken } from "./auth";

import { profileReplacement } from "./profile";

import { fillRegisterPassword } from "./register";

import { conversationIdFromUrl, isTerminal, readRunEvents } from "./runEvents";

export const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

export async function register(
  page: Page,
  request: APIRequestContext,
  email = `ask-${crypto.randomUUID()}@example.test`,
) {
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, "BodySenseE2E!123");
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

export function events(text: string): StreamEvent[] {
  return text
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => parseStreamEvent(JSON.parse(line.slice(5).trim())));
}

export async function startAsk(
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

export async function getJSON(
  request: APIRequestContext,
  url: string,
  headers: Record<string, string>,
) {
  const response = await request.get(url, { headers });
  expect(response.ok(), await response.text()).toBeTruthy();
  return response.json();
}

export async function anotherOwner(
  browser: Browser,
  baseURL: string | undefined,
) {
  const context = await browser.newContext({ baseURL });
  const page = await context.newPage();
  const headers = await register(page, context.request);
  return { context, headers };
}
