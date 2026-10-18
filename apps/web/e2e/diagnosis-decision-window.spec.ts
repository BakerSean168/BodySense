import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeStructuredSafetyIntake } from "./support/safety";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

interface SeededRun {
  conversationId: string;
  headers: Record<string, string>;
  runId: string;
  proposalTitle: string;
}

interface NativeRun {
  runId: string;
  state: string;
  proposals: Array<{
    proposalId: string;
    changeKind: string;
    content: { proposedEntry?: { medicalName?: string } };
  }>;
}

/**
 * Real Go API + deterministic Python Diagnosis stub: register, record one
 * user-message symptom through the Go-bound safety intake, then start one
 * formal analysis run from that statement. The run's proposals are Agent
 * output only; nothing is adopted until the browser settles the run.
 */
async function seedDiagnosisRun(
  page: Page,
  request: APIRequestContext,
): Promise<SeededRun> {
  const email = `e2e-dx-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = {
    Authorization: `Bearer ${accessToken}`,
    "Content-Type": "application/json",
  };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: { gender: "female", birth_date: "1994-03-12" },
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  const opening = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId: null,
      clientMessageId: `e2e-dx-open-${crypto.randomUUID()}`,
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: "我想做一次综合诊断。" }],
      },
    },
    timeout: 60_000,
  });
  expect(opening.ok(), await opening.text()).toBeTruthy();
  const conversationResponse = await request.get(
    `${apiBase}/api/v1/conversation`,
    { headers },
  );
  expect(conversationResponse.ok()).toBeTruthy();
  const conversationId = (
    (await conversationResponse.json()) as { conversation: { id: string } }
  ).conversation.id;

  const symptom = (await completeStructuredSafetyIntake(
    request,
    apiBase,
    headers,
    conversationId,
  )) as { id: string; source_evidence_id?: string };
  const records = await request.get(`${apiBase}/api/v1/body-state/records`, {
    headers,
  });
  expect(records.ok()).toBeTruthy();
  const fact = (
    (await records.json()) as {
      facts: Array<{ id: string; source_evidence_id: string }>;
    }
  ).facts.find((row) => row.id === symptom.id);
  expect(fact?.source_evidence_id).toBeTruthy();

  const diagnosis = await request.get(`${apiBase}/api/v1/diagnosis`, {
    headers,
  });
  expect(diagnosis.ok(), await diagnosis.text()).toBeTruthy();
  const { currentRevision } = (await diagnosis.json()) as {
    currentRevision: number;
  };

  const start = await request.post(
    `${apiBase}/api/v1/diagnosis/analysis-runs`,
    {
      headers,
      data: {
        requestEvidenceId: fact!.source_evidence_id,
        trigger: "new_analysis",
        expectedRevision: currentRevision,
        idempotencyKey: `e2e-dx-start-${crypto.randomUUID()}`,
      },
      timeout: 60_000,
    },
  );
  const startBody = await start.text();
  expect(start.status(), startBody).toBe(201);
  const run = JSON.parse(startBody) as NativeRun;
  expect(run.state).toBe("open");
  expect(run.proposals.length).toBeGreaterThan(0);
  const title = run.proposals[0]!.content.proposedEntry?.medicalName;
  expect(title).toBeTruthy();
  return {
    conversationId,
    headers,
    runId: run.runId,
    proposalTitle: title!,
  };
}

async function openDiagnosis(page: Page, conversationId: string) {
  await page.goto(`/consultation/${conversationId}?view=diagnosis`);
  const window = page.getByTestId("diagnosis-window");
  await expect(window).toBeVisible();
  return window;
}

test("Diagnosis proposal -> decide -> committed only after the settlement receipt", async ({
  page,
  request,
}) => {
  test.setTimeout(150_000);
  const seeded = await seedDiagnosisRun(page, request);
  const window = await openDiagnosis(page, seeded.conversationId);

  const panel = window.getByTestId("diagnosis-proposals");
  await expect(panel).toHaveAttribute("data-phase", "deciding");
  const item = panel.locator(".bc-diagnosis-proposal").first();
  await expect(
    item.getByText(seeded.proposalTitle, { exact: true }),
  ).toBeVisible();
  await expect(item.getByTestId("proposal-state")).toHaveText("待你决定");
  await expect(item.getByText(/^提案 1\/\d+ · 新病情$/)).toBeVisible();
  // A proposal is never a condition before adoption.
  await expect(window.getByText("还没有已确认的病情")).toBeVisible();
  await expect(
    window.getByRole("heading", { name: seeded.proposalTitle }),
  ).toHaveCount(0);

  await item.getByRole("radio", { name: "采纳", exact: true }).check();
  await panel.getByRole("button", { name: "保存所选的 1 条" }).click();

  // Decision receipt only: pending, still not committed.
  await expect(item.getByTestId("proposal-state")).toHaveText(
    "已选择采纳，尚未写入诊断",
  );
  await expect(window.getByTestId("diagnosis-receipt")).toHaveCount(0);
  await expect(window.getByText("还没有已确认的病情")).toBeVisible();

  const settleResponse = page.waitForResponse(
    (response) =>
      response
        .url()
        .includes(`/diagnosis/analysis-runs/${seeded.runId}/settlement`) &&
      response.request().method() === "POST",
  );
  await panel.getByRole("button", { name: /^写入已采纳的 1 条/ }).click();
  const settled = await settleResponse;
  expect(settled.status()).toBe(200);
  const receipt = (await settled.json()) as {
    outcome: string;
    receiptId: string;
  };
  expect(receipt.outcome).toBe("committed");

  // Receipt scene: committed is shown with the server's receipt id.
  const banner = window.getByTestId("diagnosis-receipt");
  await expect(banner).toContainText("已写入综合诊断");
  await expect(banner).toContainText(receipt.receiptId.slice(0, 8));
  await expect(item.getByTestId("proposal-state")).toHaveText("已写入诊断");
  await expect(panel.getByTestId("diagnosis-decide")).toHaveCount(0);
  // Current two/Current: the adopted condition comes from the fresh query.
  await expect(
    window.getByRole("heading", { name: seeded.proposalTitle }),
  ).toBeVisible();

  const current = await request.get(`${apiBase}/api/v1/diagnosis`, {
    headers: seeded.headers,
  });
  const adopted = (
    (await current.json()) as {
      conditionEntries: Array<{ medicalName: string; disposition: string }>;
    }
  ).conditionEntries.filter((entry) => entry.disposition === "adopted");
  expect(adopted.map((entry) => entry.medicalName)).toEqual([
    seeded.proposalTitle,
  ]);
});

test("urgent Safety keeps Diagnosis decisions disabled", async ({
  page,
  request,
}) => {
  test.setTimeout(150_000);
  const seeded = await seedDiagnosisRun(page, request);
  // A user-answered positive screen creates Go-owned Safety authority.
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    seeded.headers,
    seeded.conversationId,
    ["麻木"],
    "我今天肩膀不舒服。",
  );
  const workspace = await request.get(`${apiBase}/api/v1/health-workspace`, {
    headers: seeded.headers,
  });
  expect(workspace.ok(), await workspace.text()).toBeTruthy();
  const body = (await workspace.json()) as {
    capabilities: { requires_safety_review: boolean };
  };
  expect(body.capabilities.requires_safety_review).toBe(true);

  const window = await openDiagnosis(page, seeded.conversationId);
  await expect(window.getByTestId("diagnosis-safety")).toBeVisible();
  await expect(window.getByRole("radio")).toHaveCount(0);
  const decide = window.getByTestId("diagnosis-decide");
  if ((await decide.count()) > 0) {
    for (const button of await decide.getByRole("button").all()) {
      await expect(button).toBeDisabled();
    }
  }
  await expect(window.getByTestId("diagnosis-receipt")).toHaveCount(0);
});

test("Diagnosis window fits a 390px viewport with touch-sized controls", async ({
  page,
  request,
}) => {
  test.setTimeout(150_000);
  await page.setViewportSize({ width: 390, height: 844 });
  const seeded = await seedDiagnosisRun(page, request);
  const window = await openDiagnosis(page, seeded.conversationId);
  const item = window.locator(".bc-diagnosis-proposal").first();
  await expect(item).toBeVisible();

  const overflow = await page.evaluate(() => ({
    doc: document.documentElement.scrollWidth,
    body: (
      document.querySelector(
        '[data-testid="diagnosis-window"] .bc-window-body',
      ) as HTMLElement
    ).scrollWidth,
    bodyClient: (
      document.querySelector(
        '[data-testid="diagnosis-window"] .bc-window-body',
      ) as HTMLElement
    ).clientWidth,
  }));
  expect(overflow.doc).toBeLessThanOrEqual(390);
  expect(overflow.body).toBeLessThanOrEqual(overflow.bodyClient);

  const box = await window.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(390);

  for (const label of await item.locator(".bc-diagnosis-choice label").all()) {
    const target = await label.boundingBox();
    expect(target!.height).toBeGreaterThanOrEqual(44);
    expect(target!.x + target!.width).toBeLessThanOrEqual(390);
  }
  const actions = window.getByTestId("diagnosis-decide").getByRole("button");
  await expect(actions).toHaveCount(2);
  for (const button of await actions.all()) {
    const target = await button.boundingBox();
    expect(target!.height).toBeGreaterThanOrEqual(44);
    expect(target!.x + target!.width).toBeLessThanOrEqual(390);
  }

  // Tapping a choice works at 390px and enables saving.
  await item.getByRole("radio", { name: "采纳", exact: true }).check();
  await expect(
    window.getByRole("button", { name: "保存所选的 1 条" }),
  ).toBeEnabled();
});
