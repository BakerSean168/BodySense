import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeStructuredSafetyIntake } from "./support/safety";
import { profileReplacement } from "./support/profile";

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
    data: profileReplacement({ gender: "female", birth_date: "1994-03-12" }),
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

/**
 * Deep-links the Diagnosis window and waits for what it renders from.
 *
 * The window shows the run only after a chain of reads (workspace, then
 * Diagnosis and the latest run, then the run itself), and its lazy chunk loads
 * while the same cold page boots the software-rendered WebGL body scene. A
 * fixed 8 s assertion budget raced that chain under CI load (QA-001 #316), so
 * synchronize on the run read the window needs, then give the lazy window one
 * explicit render budget.
 */
async function openDiagnosis(page: Page, seeded: SeededRun) {
  const runRead = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname ===
        `/api/v1/diagnosis/analysis-runs/${seeded.runId}`,
    { timeout: 45_000 },
  );
  await page.goto(`/consultation/${seeded.conversationId}?view=diagnosis`);
  expect((await runRead).ok()).toBeTruthy();
  const window = page.getByTestId("diagnosis-window");
  await expect(window).toBeVisible({ timeout: 20_000 });
  return window;
}

test("Diagnosis proposal -> decide -> committed only after the settlement receipt", async ({
  page,
  request,
}) => {
  test.setTimeout(150_000);
  const seeded = await seedDiagnosisRun(page, request);
  const window = await openDiagnosis(page, seeded);

  // Penpot Confirm first: one proposal at a time, AI output, not yet written.
  const panel = window.getByTestId("diagnosis-proposals");
  await expect(panel).toHaveAttribute("data-phase", "deciding");
  await expect(page.getByTestId("diagnosis-mode")).toHaveText("正式分析");
  await expect(panel.getByTestId("diagnosis-step")).toContainText("尚未写入");
  const item = panel.locator(".bc-diagnosis-proposal").first();
  await expect(
    item.getByText(seeded.proposalTitle, { exact: true }),
  ).toBeVisible();
  await expect(item.getByTestId("proposal-state")).toHaveText("待你决定");
  // A proposal is never a condition before adoption.
  await expect(
    window.getByRole("heading", { name: seeded.proposalTitle }),
  ).toHaveCount(0);

  const decideResponse = page.waitForResponse(
    (response) =>
      response
        .url()
        .includes(`/diagnosis/analysis-runs/${seeded.runId}/decisions`) &&
      response.request().method() === "POST",
  );
  const settleResponse = page.waitForResponse(
    (response) =>
      response
        .url()
        .includes(`/diagnosis/analysis-runs/${seeded.runId}/settlement`) &&
      response.request().method() === "POST",
  );
  const footer = window.getByTestId("diagnosis-decide");
  await footer.getByRole("button", { name: /采纳并提交$/ }).click();
  expect((await decideResponse).status()).toBe(200);
  const settled = await settleResponse;
  expect(settled.status()).toBe(200);
  const receipt = (await settled.json()) as {
    outcome: string;
    receiptId: string;
  };
  expect(receipt.outcome).toBe("committed");

  // Receipt scene: committed is shown with the server's receipt id.
  const banner = window.getByTestId("diagnosis-receipt");
  await expect(banner).toContainText("诊断已更新");
  await expect(banner).toContainText(
    receipt.receiptId.slice(0, 8).toUpperCase(),
  );
  await expect(banner.getByTestId("proposal-state")).toHaveText("已采纳");
  await expect(page.getByTestId("diagnosis-mode")).toHaveText("提交回执");
  await expect(window.getByTestId("diagnosis-decide")).toHaveCount(0);

  // 查看当前 → Current: the adopted condition comes from the fresh query.
  await window.getByRole("button", { name: "查看当前" }).click();
  await expect(
    window.getByRole("heading", { name: seeded.proposalTitle }),
  ).toBeAttached();
  await expect(window.getByText("已采纳")).toBeVisible();

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

test("rejecting the only proposal closes the run with No changes", async ({
  page,
  request,
}) => {
  test.setTimeout(150_000);
  const seeded = await seedDiagnosisRun(page, request);
  const window = await openDiagnosis(page, seeded);
  const footer = window.getByTestId("diagnosis-decide");
  const settleResponse = page.waitForResponse(
    (response) =>
      response
        .url()
        .includes(`/diagnosis/analysis-runs/${seeded.runId}/settlement`) &&
      response.request().method() === "POST",
  );
  await footer.getByRole("button", { name: "不更改" }).click();
  const settled = await settleResponse;
  expect(settled.status()).toBe(200);
  expect(((await settled.json()) as { outcome: string }).outcome).toBe(
    "closed_without_change",
  );
  await expect(window.getByTestId("diagnosis-no-change")).toContainText(
    "诊断未更改",
  );
  await expect(window.getByTestId("diagnosis-receipt")).toHaveCount(0);
  await window.getByRole("button", { name: "返回当前" }).click();
  await expect(window.getByTestId("diagnosis-empty")).toContainText(
    "还没有诊断",
  );
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

  const window = await openDiagnosis(page, seeded);
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
  const window = await openDiagnosis(page, seeded);
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

  const actions = window.getByTestId("diagnosis-decide").getByRole("button");
  await expect(actions).toHaveCount(2);
  for (const button of await actions.all()) {
    const target = await button.boundingBox();
    expect(target!.height).toBeGreaterThanOrEqual(44);
    expect(target!.x).toBeGreaterThanOrEqual(0);
    expect(target!.x + target!.width).toBeLessThanOrEqual(390);
  }
  await expect(
    window.getByRole("button", { name: /采纳并提交$/ }),
  ).toBeEnabled();
});
