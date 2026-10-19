import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeStructuredSafetyIntake } from "./support/safety";
import {
  finishTrainerSessionWithFeedback,
  recordUserSymptom,
} from "./support/trainer";
import {
  adoptNativeDiagnosis,
  factStatementEvidenceId,
} from "./support/diagnosis";
import { profileReplacement } from "./support/profile";

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
  await page.getByRole("checkbox", { name: /同意/ }).check();
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);

  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: profileReplacement({
      gender: "male",
      birth_date: "1996-08-27",
    }),
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
    `${apiBase}/api/v1/conversation`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  expect(conversationResponse.ok()).toBeTruthy();
  const conversationData = (await conversationResponse.json()) as {
    conversation: { id: string };
  };

  await page.goto(`/consultation/${conversationData.conversation.id}`);
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
  await expect(page.locator('[data-window="chat"]')).toBeVisible();
  await page
    .locator('[data-window="chat"]')
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
  const diagnosisWindow = page.locator('[data-window="diagnosis"]');
  await expect(
    diagnosisWindow.getByRole("region", { name: "当前诊断内容" }),
  ).toBeVisible();
  // Penpot Diagnosis / Empty: title only (no empty picker) and the empty,
  // non-clinical state with a Chat entry. A pending safety reminder from the
  // seeded consultation only adds a note; it gates analyses, not Empty.
  await expect(
    diagnosisWindow.getByRole("combobox", { name: "选择已确认病情" }),
  ).toHaveCount(0);
  await expect(diagnosisWindow.getByTestId("diagnosis-empty")).toContainText(
    "还没有诊断",
  );
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
  await page.getByRole("checkbox", { name: /同意/ }).check();
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const token = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${token}` };

  expect(
    (
      await request.put(`${apiBase}/api/v1/profile`, {
        headers,
        data: profileReplacement({
          gender: "female",
          birth_date: "1994-08-27",
        }),
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
    `${apiBase}/api/v1/conversation`,
    { headers },
  );
  expect(conversationsResponse.ok()).toBeTruthy();
  const conversationsPayload = (await conversationsResponse.json()) as {
    conversation: { id: string };
  };
  const conversationId = conversationsPayload.conversation.id;
  expect(conversationId).toBeTruthy();

  // One original symptom and its own Go-bound checklist provide coverage.
  const symptom = (await completeStructuredSafetyIntake(
    request,
    apiBase,
    headers,
    String(conversationId),
  )) as { id: string };
  const requestEvidenceId = await factStatementEvidenceId(
    request,
    apiBase,
    headers,
    symptom.id,
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

  // Nothing is adopted yet: plan generation is gated on the native Diagnosis.
  const prematureGeneration = await request.post(
    `${apiBase}/api/v1/plan-proposals/generations`,
    {
      headers,
      data: { idempotencyKey: `premature-${conversationId}` },
    },
  );
  expect(prematureGeneration.status()).toBe(409);
  const prematureGenerationError = (await prematureGeneration.json()) as {
    error?: { code?: string };
  };
  expect(prematureGenerationError.error?.code).toBe("DIAGNOSIS_NOT_READY");

  const adopted = await adoptNativeDiagnosis(
    request,
    apiBase,
    headers,
    requestEvidenceId,
  );

  const staleGeneration = await request.post(
    `${apiBase}/api/v1/plan-proposals/generations`,
    {
      headers,
      data: {
        diagnosisRevision: adopted.revision + 1,
        idempotencyKey: `stale-${conversationId}`,
      },
    },
  );
  expect(staleGeneration.status()).toBe(409);
  expect(
    ((await staleGeneration.json()) as { error?: { code?: string } }).error
      ?.code,
  ).toBe("DIAGNOSIS_STALE");

  const firstProposalResponse = await request.post(
    `${apiBase}/api/v1/plan-proposals/generations`,
    {
      headers,
      data: {
        diagnosisRevision: adopted.revision,
        idempotencyKey: `first-${conversationId}`,
      },
    },
  );
  const firstProposalBody = await firstProposalResponse.text();
  expect(
    firstProposalResponse.ok(),
    `first plan proposal failed with ${firstProposalResponse.status()}: ${firstProposalBody}`,
  ).toBeTruthy();
  const firstProposal = JSON.parse(firstProposalBody) as {
    proposalId: string;
    contentFingerprint: string;
  };

  // Only a user-answered, Go-bound positive screen creates Safety authority.
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    headers,
    String(conversationId),
    ["麻木"],
    "我今天肩膀不舒服。",
  );

  // The Safety change after generation expires the proposal on accept.
  const blockedAccept = await request.post(
    `${apiBase}/api/v1/plan-proposals/${firstProposal.proposalId}/accept`,
    {
      headers,
      data: {
        decisionId: crypto.randomUUID(),
        presentedFingerprint: firstProposal.contentFingerprint,
        idempotencyKey: `accept-${firstProposal.proposalId}`,
      },
    },
  );
  expect(blockedAccept.status()).toBe(409);
  const blockedAcceptError = (await blockedAccept.json()) as {
    error?: { code?: string };
  };
  expect(blockedAcceptError.error?.code).toBe("CONTEXT_CHANGED");

  const restrictionResponse = await request.get(
    `${apiBase}/api/v1/body-state/safety/restrictions`,
    { headers },
  );
  expect(restrictionResponse.ok()).toBeTruthy();
  const safety = (await restrictionResponse.json()) as {
    current_revision: number;
    restrictions: Array<{
      id: string;
      active: boolean;
      concept: string;
      created_revision: number;
    }>;
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
    `${apiBase}/api/v1/conversation`,
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
        expected_restriction_version: active[0].created_revision,
        basis_kind: "professional_clearance",
        source_version: 1,
        idempotency_key: crypto.randomUUID(),
        source_message_id: source!.id,
        statement,
      },
    },
  );
  expect(resolveSafety.ok(), await resolveSafety.text()).toBeTruthy();
  expect(await resolveSafety.json()).toMatchObject({
    restriction_id: active[0].id,
    restriction_version: active[0].created_revision,
    basis_kind: "professional_clearance",
    source_message_id: source!.id,
    source_version: 1,
  });

  const proposalResponse = await request.post(
    `${apiBase}/api/v1/plan-proposals/generations`,
    {
      headers,
      data: {
        diagnosisRevision: adopted.revision,
        idempotencyKey: `second-${conversationId}`,
      },
    },
  );
  expect(proposalResponse.ok(), await proposalResponse.text()).toBeTruthy();
  const proposal = (await proposalResponse.json()) as {
    proposalId: string;
    contentFingerprint: string;
  };

  const acceptResponse = await request.post(
    `${apiBase}/api/v1/plan-proposals/${proposal.proposalId}/accept`,
    {
      headers,
      data: {
        decisionId: crypto.randomUUID(),
        presentedFingerprint: proposal.contentFingerprint,
        idempotencyKey: `accept-${proposal.proposalId}`,
      },
    },
  );
  expect(acceptResponse.ok(), await acceptResponse.text()).toBeTruthy();
  const accepted = (await acceptResponse.json()) as { planId: string };
  expect(accepted.planId).toBeTruthy();

  const workspaceResponse = await request.get(
    `${apiBase}/api/v1/health-workspace`,
    { headers },
  );
  expect(workspaceResponse.ok()).toBeTruthy();
  const workspace = (await workspaceResponse.json()) as {
    training_plan?: { id: string };
  };
  expect(workspace.training_plan?.id).toBe(accepted.planId);
  const beforeFacts = await request.get(
    `${apiBase}/api/v1/body-state/records`,
    { headers },
  );
  expect(beforeFacts.ok()).toBeTruthy();
  const revisionBeforeFeedback = (await beforeFacts.json()).current_revision;

  await page.goto(`/consultation/${conversationId}?view=training`);
  await expect(page).toHaveURL(/\/consultation\/[^?]+\?view=training$/);
  const trainingAction = page
    .getByTestId("training-window")
    .getByRole("button", { name: "开始训练", exact: true });
  await expect(trainingAction).toBeVisible();
  await page.reload();
  await expect(trainingAction).toBeVisible();
  await trainingAction.click();
  // The explicit Start runs one session; each set is one explicit receipt.
  const trainer = page.getByTestId("trainer-session-panel");
  await expect(trainer.getByTestId("trainer-in-progress")).toBeVisible();
  await expect(trainer.getByTestId("trainer-state")).toHaveText("进行中");
  await trainer.getByRole("button", { name: "完成本组" }).click();
  await expect(trainer.getByTestId("trainer-last-receipt")).toContainText(
    "第 1 组已记录",
  );

  // Session feedback is verbatim and never becomes a symptom; the worsening the
  // user reports is recorded through BodyState.
  await finishTrainerSessionWithFeedback(
    request,
    apiBase,
    headers,
    accepted.planId,
    "E2E worsening after session",
  );
  await recordUserSymptom(request, apiBase, headers, "肩膀不适加重");

  const finalWorkspaceResponse = await request.get(
    `${apiBase}/api/v1/health-workspace`,
    { headers },
  );
  const finalWorkspace = (await finalWorkspaceResponse.json()) as {
    capabilities: {
      requires_safety_review: boolean;
      can_start_training: boolean;
    };
  };
  const afterFacts = await request.get(`${apiBase}/api/v1/body-state/records`, {
    headers,
  });
  expect(afterFacts.ok()).toBeTruthy();
  expect((await afterFacts.json()).current_revision).toBeGreaterThan(
    revisionBeforeFeedback,
  );
  // The worsening symptom is unscreened: execution pauses, the plan is kept.
  expect(finalWorkspace.capabilities.requires_safety_review).toBe(true);
  expect(finalWorkspace.capabilities.can_start_training).toBe(false);
});
