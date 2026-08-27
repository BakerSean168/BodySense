import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

test("register -> profile -> durable BodyState fact survives reload", async ({
  page,
  request,
}) => {
  const email = `e2e-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);

  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: {
      gender: "male",
      birth_date: "1996-08-27",
    },
  });
  expect(profile.ok()).toBeTruthy();

  const run = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    data: {
      conversationId: null,
      clientMessageId: `client-${Date.now()}`,
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: "创建长期健康对话。" }],
      },
    },
    timeout: 60_000,
  });
  expect(run.ok(), await run.text()).toBeTruthy();

  const conversationResponse = await request.get(
    `${apiBase}/api/v1/conversations?limit=20`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  expect(conversationResponse.ok()).toBeTruthy();
  const conversationData = (await conversationResponse.json()) as {
    conversations: Array<{ id: string }>;
  };
  expect(conversationData.conversations).toHaveLength(1);

  await page.goto(`/consultation/${conversationData.conversations[0].id}`);
  await expect(page.getByRole("tablist", { name: "健康工作区" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "状态" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  await page.getByRole("button", { name: "收起对话区" }).click();
  await expect(page.getByRole("button", { name: "展开对话区" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("button", { name: "展开对话区" })).toBeVisible();

  await page.getByRole("tab", { name: "分析" }).click();
  await expect(page).toHaveURL(/view=diagnosis/);
  await expect(page.getByRole("tabpanel", { name: "分析" })).toBeVisible();
  await expect(page.getByText("还没有分析结果")).toBeVisible();
  await page.getByRole("tab", { name: /状态/ }).click();
  await expect(page).toHaveURL(/view=state/);

  await page.getByRole("button", { name: "添加记录" }).click();
  await page.getByPlaceholder("身体区域，例如：颈肩").fill("颈肩");
  await page.getByPlaceholder("记录内容").fill("久坐后颈肩酸胀");
  await page.getByRole("button", { name: "保存记录" }).click();

  await expect(page.getByText("久坐后颈肩酸胀")).toBeVisible();

  const bodyStateResponse = await request.get(`${apiBase}/api/v1/body-state`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  expect(bodyStateResponse.ok()).toBeTruthy();
  const bodyState = (await bodyStateResponse.json()) as {
    current_revision: number;
    facts: Array<{ value: string }>;
  };
  expect(bodyState.current_revision).toBe(1);
  expect(bodyState.facts).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ value: "久坐后颈肩酸胀" }),
    ]),
  );

  await page.reload();
  await expect(page.getByText("久坐后颈肩酸胀")).toBeVisible();
});

test("full longitudinal loop enforces gates and remains discoverable after reload", async ({
  page,
  request,
}) => {
  const email = `loop-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const token = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${token}` };

  expect(
    (
      await request.put(`${apiBase}/api/v1/profile`, {
        headers,
        data: {
          gender: "female",
          birth_date: "1994-08-27",
        },
      })
    ).ok(),
  ).toBeTruthy();

  const run = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId: null,
      clientMessageId: `e2e-client-${Date.now()}`,
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: "E2E create consultation" }],
      },
    },
  });
  expect(run.ok()).toBeTruthy();
  await run.text();

  const conversationsResponse = await request.get(
    `${apiBase}/api/v1/conversations`,
    { headers },
  );
  expect(conversationsResponse.ok()).toBeTruthy();
  const conversationsPayload = (await conversationsResponse.json()) as {
    conversations: Array<{ id: string }>;
  };
  const conversationId = conversationsPayload.conversations[0]?.id;
  expect(conversationId).toBeTruthy();

  const factResponse = await request.post(
    `${apiBase}/api/v1/body-state/facts`,
    {
      headers,
      data: {
        expected_revision: 0,
        fact: {
          concern_key: "region:neck",
          kind: "discomfort",
          body_region: "颈肩",
          value: "久坐后颈肩酸胀",
          details: { trigger: "久坐", severity: "中度" },
          origin: "user_reported",
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: "stable",
          source_key: `e2e-fact-${Date.now()}`,
        },
      },
    },
  );
  expect(factResponse.ok()).toBeTruthy();

  const staleRevisionWrite = await request.post(
    `${apiBase}/api/v1/body-state/facts`,
    {
      headers,
      data: {
        expected_revision: 0,
        fact: {
          concern_key: "region:neck",
          kind: "discomfort",
          body_region: "颈肩",
          value: "stale concurrent write",
          origin: "user_reported",
          review_state: "confirmed",
          lifecycle_state: "active",
          source_key: `e2e-stale-${Date.now()}`,
        },
      },
    },
  );
  expect(staleRevisionWrite.status()).toBe(409);

  const concurrentRun = (text: string) =>
    request.post(`${apiBase}/api/v1/consultation-runs`, {
      headers,
      data: {
        conversationId,
        clientMessageId: `e2e-concurrent-${crypto.randomUUID()}`,
        requestId: crypto.randomUUID(),
        message: { role: "user", parts: [{ type: "text", text }] },
      },
    });
  const concurrentResponses = await Promise.all([
    concurrentRun("E2E_HOLD_RUN"),
    concurrentRun("E2E competing run"),
  ]);
  expect(
    concurrentResponses.map((response) => response.status()).sort(),
  ).toEqual([200, 409]);
  await Promise.all(concurrentResponses.map((response) => response.text()));

  const diagnosisResponse = await request.post(
    `${apiBase}/api/v1/consultations/${conversationId}/diagnosis`,
    { headers },
  );
  expect(diagnosisResponse.ok()).toBeTruthy();
  const diagnosis = (await diagnosisResponse.json()) as {
    analysis_id: string;
    candidates: Array<{ candidate_id: string }>;
  };
  expect(diagnosis.candidates).toHaveLength(1);

  const prematureTreatment = await request.post(
    `${apiBase}/api/v1/treatments/proposals`,
    {
      headers,
      data: { diagnosis_analysis_id: diagnosis.analysis_id },
    },
  );
  expect(prematureTreatment.status()).toBe(409);
  const prematureTreatmentError = (await prematureTreatment.json()) as {
    error?: { code?: string };
  };
  expect(prematureTreatmentError.error?.code).toBe(
    "DIAGNOSIS_ASSESSMENT_REQUIRED",
  );

  const assessment = await request.put(
    `${apiBase}/api/v1/diagnosis-analyses/${diagnosis.analysis_id}/assessment`,
    {
      headers,
      data: {
        candidates: [
          {
            candidate_id: diagnosis.candidates[0].candidate_id,
            state: "confirmed",
          },
        ],
      },
    },
  );
  expect(assessment.ok()).toBeTruthy();

  const firstProposalResponse = await request.post(
    `${apiBase}/api/v1/treatments/proposals`,
    {
      headers,
      data: { diagnosis_analysis_id: diagnosis.analysis_id },
    },
  );
  const firstProposalBody = await firstProposalResponse.text();
  expect(
    firstProposalResponse.ok(),
    `first Treatment proposal failed with ${firstProposalResponse.status()}: ${firstProposalBody}`,
  ).toBeTruthy();
  const firstProposal = (
    JSON.parse(firstProposalBody) as { proposal: { id: string } }
  ).proposal;

  const safetyRun = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId,
      clientMessageId: `e2e-safety-${Date.now()}`,
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: "E2E_TRIGGER_SAFETY" }],
      },
    },
  });
  expect(safetyRun.ok()).toBeTruthy();
  await safetyRun.text();

  const blockedAccept = await request.post(
    `${apiBase}/api/v1/treatments/revisions/${firstProposal.id}/accept`,
    { headers, data: { consultation_id: conversationId } },
  );
  expect(blockedAccept.status()).toBe(409);
  const blockedAcceptError = (await blockedAccept.json()) as {
    error?: { code?: string };
  };
  expect(blockedAcceptError.error?.code).toBe("TREATMENT_SAFETY_BLOCKED");

  const resolveSafety = await request.post(
    `${apiBase}/api/v1/body-state/safety/resolve`,
    {
      headers,
      data: { resolution: "cleared_by_review", note: "E2E reviewed" },
    },
  );
  expect(resolveSafety.ok()).toBeTruthy();

  const freshDiagnosisResponse = await request.post(
    `${apiBase}/api/v1/consultations/${conversationId}/diagnosis`,
    { headers },
  );
  expect(freshDiagnosisResponse.ok()).toBeTruthy();
  const freshDiagnosis = (await freshDiagnosisResponse.json()) as {
    analysis_id: string;
    candidates: Array<{ candidate_id: string }>;
  };
  expect(
    (
      await request.put(
        `${apiBase}/api/v1/diagnosis-analyses/${freshDiagnosis.analysis_id}/assessment`,
        {
          headers,
          data: {
            candidates: [
              {
                candidate_id: freshDiagnosis.candidates[0].candidate_id,
                state: "confirmed",
              },
            ],
          },
        },
      )
    ).ok(),
  ).toBeTruthy();

  const proposalResponse = await request.post(
    `${apiBase}/api/v1/treatments/proposals`,
    {
      headers,
      data: { diagnosis_analysis_id: freshDiagnosis.analysis_id },
    },
  );
  expect(proposalResponse.ok()).toBeTruthy();
  const proposal = (
    (await proposalResponse.json()) as { proposal: { id: string } }
  ).proposal;

  const acceptResponse = await request.post(
    `${apiBase}/api/v1/treatments/revisions/${proposal.id}/accept`,
    { headers, data: { consultation_id: conversationId } },
  );
  expect(acceptResponse.ok()).toBeTruthy();
  const accepted = (await acceptResponse.json()) as {
    training_plan: { id: string };
  };
  expect(accepted.training_plan.id).toBeTruthy();

  const workspaceResponse = await request.get(
    `${apiBase}/api/v1/health-workspace`,
    { headers },
  );
  expect(workspaceResponse.ok()).toBeTruthy();
  const workspace = (await workspaceResponse.json()) as {
    training_plan?: { id: string };
    body_state: { current_revision: number };
  };
  expect(workspace.training_plan?.id).toBe(accepted.training_plan.id);
  const revisionBeforeFeedback = workspace.body_state.current_revision;

  await page.goto(`/training/${accepted.training_plan.id}`);
  await expect(page).toHaveURL(/\/consultation(?:\/[^?]+)?\?view=treatment$/);
  const trainingAction = page.getByRole("button", { name: "展开训练执行" });
  await expect(trainingAction).toBeVisible();
  await page.reload();
  await expect(trainingAction).toBeVisible();
  await trainingAction.click();
  await expect(
    page.getByRole("heading", { name: "训练计划 (Training Plan)" }),
  ).toBeVisible();

  const feedback = await request.put(
    `${apiBase}/api/v1/training/${accepted.training_plan.id}/log`,
    {
      headers,
      data: {
        notes: "E2E worsening after session",
        exercises: [],
        symptom_changes: "颈肩酸胀加重",
        training_feeling: "uncomfortable",
        difficulties: "pain",
        body_region: "颈肩",
        concern_key: "region:neck",
        trend: "worsening",
      },
    },
  );
  expect(feedback.ok()).toBeTruthy();

  const finalWorkspaceResponse = await request.get(
    `${apiBase}/api/v1/health-workspace`,
    { headers },
  );
  const finalWorkspace = (await finalWorkspaceResponse.json()) as {
    body_state: { current_revision: number };
    recent_outcomes: unknown[];
    capabilities: { requires_treatment_review: boolean };
  };
  expect(finalWorkspace.body_state.current_revision).toBeGreaterThan(
    revisionBeforeFeedback,
  );
  expect(finalWorkspace.recent_outcomes.length).toBeGreaterThan(0);
  expect(finalWorkspace.capabilities.requires_treatment_review).toBe(true);
});
