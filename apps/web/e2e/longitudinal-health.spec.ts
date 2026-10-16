import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeStructuredSafetyIntake } from "./support/safety";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

test("register -> profile -> durable BodyState fact survives reload", async ({
  page,
  request,
}) => {
  test.setTimeout(120_000);
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
  await expect(page.getByTestId("body-canvas-workspace")).toBeVisible();
  // Body-first: no business window is open until the user opens one, and that
  // choice is mirrored into the route.
  await expect(page.getByTestId("diagnosis-window")).toBeHidden();

  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "对话", exact: true })
    .click();
  await expect(page).toHaveURL(/view=chat/);
  await page.reload();
  await expect(page).toHaveURL(/view=chat/);
  await expect(
    page.locator('.bc-business-window[data-window="chat"]'),
  ).toBeVisible();
  await page
    .locator('.bc-business-window[data-window="chat"]')
    .getByRole("button", { name: "关闭对话" })
    .click();

  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "综合诊断", exact: true })
    .click();
  await expect(page).toHaveURL(/view=diagnosis/);
  await expect(page.getByTestId("diagnosis-window")).toBeVisible();
  // Penpot V2 replaces the old heading with a toolbar selector. Verify the
  // actual labeled business window and the empty, non-clinical state.
  const diagnosisWindow = page.locator(
    '.bc-business-window[data-window="diagnosis"]',
  );
  await expect(
    diagnosisWindow.getByRole("region", { name: "当前诊断内容" }),
  ).toBeVisible();
  const diagnosisPicker = diagnosisWindow.getByRole("combobox", {
    name: "选择已确认病情",
  });
  await expect(diagnosisPicker).toBeDisabled();
  // Native <option> elements have no visible bounding box when the select is
  // closed. Assert their content, not an impossible visible-option state.
  await expect(diagnosisPicker.locator("option")).toHaveText("暂无已确认病情");
  await expect(diagnosisWindow.getByText("还没有已确认的病情")).toBeVisible();
  await expect(
    diagnosisWindow.getByRole("button", { name: "关闭综合诊断" }),
  ).toBeVisible();

  await page
    .locator(".bc-dock")
    .getByRole("button", { name: "个人档案", exact: true })
    .click();
  await expect(page).toHaveURL(/view=person/);
  // The tab is the only place the category name appears.
  await expect(page.getByRole("heading", { name: "身体记录" })).toBeHidden();

  await page
    .getByTestId("body-fact-records")
    .getByRole("button", { name: "补充记录" })
    .click();
  await page.getByLabel("你的描述").fill("久坐后颈肩酸胀");
  await page.getByRole("button", { name: "保存记录" }).click();

  await expect(page.getByText("久坐后颈肩酸胀")).toBeVisible();

  const bodyStateResponse = await request.get(
    `${apiBase}/api/v1/body-state/records`,
    {
      headers: { Authorization: `Bearer ${accessToken}` },
    },
  );
  expect(bodyStateResponse.ok()).toBeTruthy();
  const bodyState = (await bodyStateResponse.json()) as {
    current_revision: number;
    facts: Array<{ statement: string }>;
  };
  expect(bodyState.current_revision).toBe(1);
  expect(bodyState.facts).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ statement: "久坐后颈肩酸胀" }),
    ]),
  );

  await page.reload();
  // The deep link reopens the same business window after a full reload.
  await expect(page.getByTestId("person-window")).toBeVisible();
  await expect(page.getByText("久坐后颈肩酸胀")).toBeVisible();
});

test("full longitudinal loop enforces gates and remains discoverable after reload", async ({
  page,
  request,
}) => {
  test.setTimeout(120_000);
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

  // One original symptom and its own Go-bound checklist provide coverage.
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    headers,
    String(conversationId),
  );

  const staleRevisionWrite = await request.post(
    `${apiBase}/api/v1/body-state/records`,
    {
      headers,
      data: {
        idempotency_key: `e2e-stale-${Date.now()}`,
        expected_revision: 0,
        facts: [
          {
            kind: "symptom",
            statement: "stale concurrent write",
            body_region_ids: ["neck"],
            temporal: { precision: "unknown" },
            details: {},
          },
        ],
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
  const diagnosisBody = await diagnosisResponse.text();
  expect(diagnosisResponse.ok(), diagnosisBody).toBeTruthy();
  const diagnosis = JSON.parse(diagnosisBody) as {
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

  // Only a user-answered, Go-bound positive screen creates Safety authority.
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    headers,
    String(conversationId),
    ["麻木"],
    "我今天肩膀不舒服。",
  );

  const blockedAccept = await request.post(
    `${apiBase}/api/v1/treatments/revisions/${firstProposal.id}/accept`,
    { headers, data: { consultation_id: conversationId } },
  );
  expect(blockedAccept.status()).toBe(409);
  const blockedAcceptError = (await blockedAccept.json()) as {
    error?: { code?: string };
  };
  expect(blockedAcceptError.error?.code).toBe("TREATMENT_SAFETY_BLOCKED");

  const restrictionResponse = await request.get(
    `${apiBase}/api/v1/body-state/safety/restrictions`,
    { headers },
  );
  expect(restrictionResponse.ok()).toBeTruthy();
  const safety = (await restrictionResponse.json()) as {
    current_revision: number;
    restrictions: Array<{ id: string; active: boolean; concept: string }>;
  };
  const active = safety.restrictions.filter((row) => row.active);
  expect(active).toHaveLength(1);
  expect(active[0].concept).toBe("numbness");

  const statement = "专业人员已针对这条限制及全部训练放行";
  const clearanceRun = await request.post(
    `${apiBase}/api/v1/consultation-runs`,
    {
      headers,
      data: {
        conversationId,
        clientMessageId: crypto.randomUUID(),
        requestId: crypto.randomUUID(),
        message: { role: "user", parts: [{ type: "text", text: statement }] },
      },
    },
  );
  expect(clearanceRun.ok()).toBeTruthy();
  await clearanceRun.text();
  const conversationResponse = await request.get(
    `${apiBase}/api/v1/conversations/${conversationId}`,
    { headers },
  );
  expect(conversationResponse.ok()).toBeTruthy();
  const conversation = (await conversationResponse.json()) as {
    messages: Array<{
      id: string;
      role: string;
      parts: Array<{ type: string; text?: string }>;
    }>;
  };
  const source = conversation.messages.find(
    (message) =>
      message.role === "user" &&
      message.parts.some(
        (part) => part.type === "text" && part.text === statement,
      ),
  );
  expect(source).toBeTruthy();
  const resolveSafety = await request.post(
    `${apiBase}/api/v1/body-state/safety/restrictions/${active[0].id}/resolve`,
    {
      headers,
      data: {
        expected_revision: safety.current_revision,
        idempotency_key: crypto.randomUUID(),
        source_message_id: source!.id,
        statement,
      },
    },
  );
  expect(resolveSafety.ok(), await resolveSafety.text()).toBeTruthy();

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
  };
  expect(workspace.training_plan?.id).toBe(accepted.training_plan.id);
  const beforeFacts = await request.get(
    `${apiBase}/api/v1/body-state/records`,
    { headers },
  );
  expect(beforeFacts.ok()).toBeTruthy();
  const revisionBeforeFeedback = (await beforeFacts.json()).current_revision;

  await page.goto(`/consultation/${conversationId}?view=treatment`);
  await expect(page).toHaveURL(/\/consultation\/[^?]+\?view=treatment$/);
  const trainingAction = page
    .getByTestId("training-window")
    .getByRole("button", { name: "开始训练", exact: true });
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
        symptom_changes: "肩膀不适加重",
        training_feeling: "uncomfortable",
        difficulties: "pain",
        body_region: "肩部",
        concern_key: "general",
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
    recent_outcomes: unknown[];
    capabilities: { requires_treatment_review: boolean };
  };
  const afterFacts = await request.get(`${apiBase}/api/v1/body-state/records`, {
    headers,
  });
  expect(afterFacts.ok()).toBeTruthy();
  expect((await afterFacts.json()).current_revision).toBeGreaterThan(
    revisionBeforeFeedback,
  );
  expect(finalWorkspace.recent_outcomes.length).toBeGreaterThan(0);
  expect(finalWorkspace.capabilities.requires_treatment_review).toBe(true);
});
