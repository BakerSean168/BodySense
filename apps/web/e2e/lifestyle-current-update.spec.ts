import { expect, test, type APIResponse, type Page } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function expectJson<T>(response: APIResponse, label: string): Promise<T> {
  const body = await response.text();
  expect(
    response.ok(),
    `${label} failed with ${response.status()}: ${body}`,
  ).toBeTruthy();
  return JSON.parse(body) as T;
}

async function registerAndProfile(page: Page): Promise<string> {
  const email = `lifestyle-update-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const profile = await page
    .context()
    .request.put(`${apiBase}/api/v1/profile`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      data: {
        gender: "male",
        birth_date: "1996-08-27",
      },
    });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  return accessToken;
}

test("lifestyle update preserves the previous fact and promotes the new current state", async ({
  page,
  request,
}) => {
  const accessToken = await registerAndProfile(page);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const previousSummary = "日常活动以久坐为主";
  const currentSummary = "现在工作中会频繁走动";

  const initialLifestyle = await expectJson<{
    current_revision: number;
    activity: { fact_id: string; summary: string };
  }>(
    await request.put(`${apiBase}/api/v1/lifestyle`, {
      headers,
      data: {
        expected_revision: 0,
        activity: {
          summary: previousSummary,
          details: { source: "playwright-e2e" },
        },
      },
    }),
    "seed lifestyle activity",
  );

  expect(initialLifestyle.current_revision).toBe(1);
  expect(initialLifestyle.activity.summary).toBe(previousSummary);
  const previousFactId = initialLifestyle.activity.fact_id;
  expect(previousFactId).toBeTruthy();

  await page.goto("/consultation?view=state");
  await expect(page.getByText(previousSummary, { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "更新现状", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "后来改善", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "后来加重", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "后来恢复", exact: true }),
  ).toHaveCount(0);

  await page.getByRole("button", { name: "更新现状", exact: true }).click();
  const updatePanel = page
    .getByText("原记录当时是正确的", { exact: false })
    .locator("xpath=..");
  const editor = updatePanel.locator("textarea");
  await expect(editor).toBeVisible();
  await expect(editor).toHaveValue(previousSummary);
  await editor.fill(currentSummary);
  await page.getByRole("button", { name: "保存更新", exact: true }).click();

  await expect(page.getByText(currentSummary, { exact: true })).toBeVisible();
  await expect(page.getByText(previousSummary, { exact: true })).toHaveCount(0);

  const lifestyleAfterUpdate = await expectJson<{
    current_revision: number;
    activity: { fact_id: string; summary: string };
  }>(
    await request.get(`${apiBase}/api/v1/lifestyle`, { headers }),
    "read updated lifestyle",
  );
  expect(lifestyleAfterUpdate.current_revision).toBe(2);
  expect(lifestyleAfterUpdate.activity.summary).toBe(currentSummary);
  expect(lifestyleAfterUpdate.activity.fact_id).not.toBe(previousFactId);

  const bodyState = await expectJson<{
    current_revision: number;
    facts: Array<{
      id: string;
      kind: string;
      value: string;
      lifecycle_state: string;
      supersedes_fact_id?: string;
    }>;
    recent_revisions: Array<{
      revision: number;
      change_type: string;
      source: string;
      changes: {
        facts?: Array<{
          kind: string;
          action: string;
          previous?: {
            id: string;
            value: string;
            lifecycle_state: string;
          };
          replacement?: {
            id: string;
            value: string;
            lifecycle_state: string;
            supersedes_fact_id?: string;
          };
        }>;
      };
    }>;
  }>(
    await request.get(`${apiBase}/api/v1/body-state`, { headers }),
    "read BodyState after lifestyle transition",
  );

  expect(bodyState.current_revision).toBe(2);
  expect(bodyState.facts).toEqual([
    expect.objectContaining({
      id: lifestyleAfterUpdate.activity.fact_id,
      kind: "lifestyle.activity",
      value: currentSummary,
      lifecycle_state: "active",
      supersedes_fact_id: previousFactId,
    }),
  ]);

  const transitionRevision = bodyState.recent_revisions.find(
    (revision) => revision.revision === 2,
  );
  expect(transitionRevision).toBeTruthy();
  expect(transitionRevision).toMatchObject({
    change_type: "current_context.updated",
    source: "lifestyle_editor",
  });
  expect(transitionRevision?.changes.facts).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        kind: "lifestyle.activity",
        action: "transitioned",
        previous: expect.objectContaining({
          id: previousFactId,
          value: previousSummary,
          lifecycle_state: "active",
        }),
        replacement: expect.objectContaining({
          id: lifestyleAfterUpdate.activity.fact_id,
          value: currentSummary,
          lifecycle_state: "active",
          supersedes_fact_id: previousFactId,
        }),
      }),
    ]),
  );

  await page.reload();
  await expect(page.getByText(currentSummary, { exact: true })).toBeVisible();
  await expect(page.getByText(previousSummary, { exact: true })).toHaveCount(0);
});
