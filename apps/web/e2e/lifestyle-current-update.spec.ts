import { expect, test, type APIResponse, type Page } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";
import { choosePersonCategory } from "./support/person";

// These cases include registration, real Go/Python/DB calls and full reloads.
// Keep each expectation's normal timeout; allow a cold 3D/browser startup on
// shared CI hosts without exhausting the entire case's default 30 seconds.
test.describe.configure({ timeout: 90_000 });

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
  const email = `lifestyle-update-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);
  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const profile = await page
    .context()
    .request.put(`${apiBase}/api/v1/profile`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      data: profileReplacement({ gender: "male", birth_date: "1996-08-27" }),
    });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return accessToken;
}

test("lifestyle temporal change preserves the earlier assertion rather than correcting its history", async ({
  page,
  request,
}) => {
  const accessToken = await registerAndProfile(page);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const previousStatement = "我之前日常活动以久坐为主";
  const currentStatement = "我现在工作中会频繁走动";

  const initial = await expectJson<{ revision: number; fact_ids: string[] }>(
    await request.post(`${apiBase}/api/v1/body-state/records`, {
      headers,
      data: {
        idempotency_key: crypto.randomUUID(),
        expected_revision: 0,
        facts: [
          {
            kind: "lifestyle",
            statement: previousStatement,
            body_region_ids: [],
            temporal: { precision: "unknown", original_expression: "之前" },
            details: { habit_type: "activity" },
          },
        ],
      },
    }),
    "seed prior-period habit assertion",
  );
  expect(initial.revision).toBe(1);
  const previousId = initial.fact_ids[0];
  expect(previousId).toBeTruthy();

  await page.goto("/consultation?view=state");
  await expect(page.getByTestId("person-window")).toBeVisible();
  await choosePersonCategory(page, "habits");
  const records = page.getByTestId("body-fact-records");
  await expect(
    records.getByText(previousStatement, { exact: true }),
  ).toBeVisible();
  // V2 temporal change is another source-bound assertion; the previous report
  // was not false. Do not recreate the old universal lifecycle/status editor.
  await expect(
    records.getByRole("button", { name: "更新现状", exact: true }),
  ).toHaveCount(0);
  await expect(
    records.getByRole("button", { name: "后来恢复", exact: true }),
  ).toHaveCount(0);
  await records.getByRole("button", { name: "补充记录" }).click();
  await records.getByLabel("你的描述").fill(currentStatement);
  await records.getByLabel(/发生时间/).fill("现在");
  await records.getByRole("button", { name: "保存记录" }).click();
  await expect(records.getByText("已保存你的陈述。")).toBeVisible();
  await expect(
    records.getByText(currentStatement, { exact: true }),
  ).toBeVisible();
  await expect(
    records.getByText(previousStatement, { exact: true }),
  ).toBeVisible();

  const updated = await expectJson<{
    current_revision: number;
    facts: Array<{
      id: string;
      kind: string;
      statement: string;
      assertion_status: string;
      temporal: { precision: string; original_expression?: string };
      supersedes_fact_id?: string;
    }>;
  }>(
    await request.get(`${apiBase}/api/v1/body-state/records`, { headers }),
    "read retained habit history",
  );
  expect(updated.current_revision).toBe(2);
  expect(updated.facts).toHaveLength(2);
  expect(updated.facts.find((fact) => fact.id === previousId)).toMatchObject({
    statement: previousStatement,
    kind: "lifestyle",
    assertion_status: "asserted",
    temporal: { precision: "unknown", original_expression: "之前" },
  });
  const current = updated.facts.find(
    (fact) => fact.statement === currentStatement,
  );
  expect(current).toMatchObject({
    kind: "lifestyle",
    assertion_status: "asserted",
    temporal: { precision: "unknown", original_expression: "现在" },
  });
  expect(current?.id).not.toBe(previousId);
  expect(
    updated.facts.every((fact) => fact.supersedes_fact_id === undefined),
  ).toBe(true);
  await page.reload();
  await expect(
    records.getByText(currentStatement, { exact: true }),
  ).toBeVisible();
  await expect(
    records.getByText(previousStatement, { exact: true }),
  ).toBeVisible();
});
