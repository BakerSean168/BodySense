import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

// AS-002 statement erasure in the Person Records window at 390px: the real
// Go/PostgreSQL flow (review -> withdraw -> verified erasure) and the receipt
// states that need server conditions a fresh account cannot reach in a test
// (in progress, transient outstanding, policy retention) via route fixtures.
test.describe.configure({ timeout: 90_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function register(page: Page, request: APIRequestContext) {
  const email = `statement-erasure-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "male", birth_date: "2000-01-01" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return headers;
}

async function saveRecord(page: Page, text: string) {
  await page.goto("/consultation?view=state");
  const records = page.getByTestId("body-fact-records");
  await expect(records).toBeVisible();
  await page.getByTestId("person-add").click();
  await page.getByLabel("你的描述").fill(text);
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(records.getByText(text, { exact: true })).toBeVisible();
  return records;
}

async function expectInsideViewport(page: Page) {
  const panel = page.getByTestId("statement-erasure");
  const box = await panel.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  const root = await page.evaluate(() => ({
    width: document.documentElement.clientWidth,
    scroll: document.documentElement.scrollWidth,
  }));
  expect(root.scroll).toBeLessThanOrEqual(root.width);
}

const targetNames = [
  "diagnosis_records",
  "training_plans",
  "body_records",
  "assessment_reports",
  "decision_records",
  "runtime_checkpoints",
  "source_message",
  "statement_manifest",
];

function syntheticReceipt(
  statementId: string,
  status: string,
  overrides: Record<string, Record<string, unknown>> = {},
) {
  const targets = targetNames.map((target) => ({
    target,
    state: "verified",
    erased_count: 0,
    outstanding_count: 0,
    verified_at: "2026-10-19T00:00:00Z",
    ...overrides[target],
  }));
  for (const target of targets) {
    if (target.state !== "verified")
      delete (target as { verified_at?: string }).verified_at;
  }
  return {
    request_id: crypto.randomUUID(),
    statement_id: statementId,
    status,
    inventory_revision: "statement-erasure-inventory-v2",
    requested_at: "2026-10-19T00:00:00Z",
    ...(status.startsWith("erased")
      ? { erased_at: "2026-10-19T00:01:00Z" }
      : {}),
    targets,
    outstanding_targets: targets
      .filter((t) => t.state !== "verified" && t.state !== "retained_by_policy")
      .map((t) => t.target),
    retained_targets: targets
      .filter((t) => t.state === "retained_by_policy")
      .map((t) => t.target),
    reviews: [],
  };
}

test("statement erasure asks for review, withdraws on confirmation and verifies every copy", async ({
  page,
  request,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const headers = await register(page, request);
  const text = "我上周开始每天晚上十一点后才睡";
  const records = await saveRecord(page, text);
  const listed = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  const fact = (await listed.json()).facts[0] as {
    id: string;
    source_evidence_id: string;
  };

  // Penpot Records: erasure lives in the statement's detail scene.
  await records.getByRole("button", { name: `查看记录：${text}` }).click();
  await records.getByRole("button", { name: "彻底删除" }).click();
  const panel = page.getByTestId("statement-erasure");
  await expect(panel).toHaveAttribute("data-phase", "confirm");
  await panel.getByRole("button", { name: "确认彻底删除" }).click();
  // The record is still asserted: the server blocks and names it for review.
  await expect(panel).toHaveAttribute("data-phase", "review");
  await expect(panel.getByText("需要你先处理")).toBeVisible();
  await expect(
    panel.getByText("由它整理出的记录仍有效，请先撤回"),
  ).toBeVisible();
  await expect(panel.getByText("这条记录仍有效：请先撤回")).toBeVisible();
  await expectInsideViewport(page);
  await page.screenshot({
    path: testInfo.outputPath("statement-erasure-review.png"),
    fullPage: true,
  });

  await panel.getByRole("button", { name: "撤回此记录并继续删除" }).click();
  await expect(panel).toHaveAttribute("data-phase", "erased");
  await expect(panel.getByText("已彻底删除")).toBeVisible();
  await expect(records.getByText(text, { exact: true })).toHaveCount(0);
  await expectInsideViewport(page);
  await page.screenshot({
    path: testInfo.outputPath("statement-erasure-done.png"),
    fullPage: true,
  });

  const receipt = await request.get(
    `${api}/api/v1/body-state/statements/${fact.source_evidence_id}/erasure`,
    { headers },
  );
  expect(receipt.ok()).toBeTruthy();
  const body = await receipt.json();
  expect(body.status).toBe("erased");
  expect(body.outstanding_targets).toEqual([]);
  expect(body.retained_targets).toEqual([]);
  const after = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  expect(JSON.stringify(await after.json())).not.toContain(text);
});

test("statement erasure receipt shows in progress, retrying and policy-retained states", async ({
  page,
  request,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await register(page, request);
  const text = "我最近两周膝盖上楼时发紧";
  const records = await saveRecord(page, text);
  const states: Array<(statementId: string) => object> = [
    (id) =>
      syntheticReceipt(id, "running", {
        statement_manifest: { state: "pending" },
      }),
    (id) =>
      syntheticReceipt(id, "outstanding", {
        runtime_checkpoints: {
          state: "failed",
          failure_code: "runtime_thread_busy",
          outstanding_count: 1,
        },
        statement_manifest: {
          state: "blocked",
          failure_code: "targets_outstanding",
          outstanding_count: 1,
        },
      }),
    (id) =>
      syntheticReceipt(id, "erased_with_retention", {
        decision_records: {
          state: "retained_by_policy",
          retention_reason: "replay_input_frozen",
          outstanding_count: 3,
        },
      }),
  ];
  let call = 0;
  await page.route(
    "**/api/v1/body-state/statements/*/erasure",
    async (route) => {
      const statementId = route
        .request()
        .url()
        .split("/statements/")[1]
        .split("/")[0];
      const body = states[Math.min(call, states.length - 1)](statementId);
      call += 1;
      await route.fulfill({
        status: call >= states.length ? 200 : 202,
        json: body,
      });
    },
  );

  // Penpot Records: erasure lives in the statement's detail scene.
  await records.getByRole("button", { name: `查看记录：${text}` }).click();
  await records.getByRole("button", { name: "彻底删除" }).click();
  const panel = page.getByTestId("statement-erasure");
  await panel.getByRole("button", { name: "确认彻底删除" }).click();
  await expect(panel).toHaveAttribute("data-phase", "in_progress");
  await expect(panel.getByText("正在删除", { exact: true })).toBeVisible();
  await expectInsideViewport(page);

  await panel.getByRole("button", { name: "再次检查" }).click();
  await expect(panel).toHaveAttribute("data-phase", "outstanding");
  await expect(panel.getByText("对话正在进行，空闲后会自动重试")).toBeVisible();
  await expectInsideViewport(page);
  await page.screenshot({
    path: testInfo.outputPath("statement-erasure-outstanding.png"),
    fullPage: true,
  });

  await panel.getByRole("button", { name: "再次检查" }).click();
  await expect(panel).toHaveAttribute("data-phase", "retained");
  await expect(panel.getByText("已删除，部分内容按政策保留")).toBeVisible();
  await expect(
    panel.getByText("按政策保留 · 冻结的诊断与计划决策输入"),
  ).toBeVisible();
  await expect(panel.getByText("已彻底删除")).toHaveCount(0);
  await expect(panel.getByRole("button", { name: "再次检查" })).toHaveCount(0);
  await expectInsideViewport(page);
  await page.screenshot({
    path: testInfo.outputPath("statement-erasure-retained.png"),
    fullPage: true,
  });
});
