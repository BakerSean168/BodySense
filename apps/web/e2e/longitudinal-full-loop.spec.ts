import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeStructuredSafetyIntake } from "./support/safety";
import {
  finishTrainerSessionWithFeedback,
  getTrainerPlan,
  recordUserSymptom,
} from "./support/trainer";
import {
  adoptNativeDiagnosis,
  factStatementEvidenceId,
} from "./support/diagnosis";
import { profileReplacement } from "./support/profile";

// The genuine consultation stream explicitly permits a 60s API request.
// Playwright's default 30s test deadline would abort that allowed request
// on a cold shared CI browser. Match the suite's bounded 90s full-run budget;
// do not relax individual application assertions or mask failed responses.
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

async function registerAndProfile(
  page: Page,
  request: APIRequestContext,
): Promise<{ accessToken: string; email: string }> {
  const email = `full-loop-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("checkbox", { name: /同意/ }).check();
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);

  const profileResponse = await request.put(`${apiBase}/api/v1/profile`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: profileReplacement({
      gender: "male",
      birth_date: "1996-08-27",
    }),
  });
  expect(profileResponse.ok()).toBeTruthy();
  return { accessToken, email };
}

test("BodyState -> Diagnosis -> Treatment -> Training -> feedback review closes the longitudinal loop", async ({
  page,
  request,
}) => {
  const { accessToken } = await registerAndProfile(page, request);
  const authHeaders = {
    Authorization: `Bearer ${accessToken}`,
    "Content-Type": "application/json",
  };

  const runResponse = await request.post(
    `${apiBase}/api/v1/consultation-runs`,
    {
      headers: authHeaders,
      data: {
        conversationId: null,
        clientMessageId: `client-${Date.now()}`,
        requestId: crypto.randomUUID(),
        message: {
          role: "user",
          parts: [
            { type: "text", text: "我久坐后颈肩酸胀，想先记录当前情况。" },
          ],
        },
      },
      timeout: 60_000,
    },
  );
  const runBody = await runResponse.text();
  expect(
    runResponse.ok(),
    `consultation run failed with ${runResponse.status()}: ${runBody}`,
  ).toBeTruthy();
  expect(runBody).toContain("event: stream.done");

  const singleton = await expectJson<{
    conversation: { id: string };
  }>(
    await request.get(`${apiBase}/api/v1/conversation`, {
      headers: authHeaders,
    }),
    "get the single conversation",
  );
  const conversationId = singleton.conversation.id;

  // Capture and screen the same symptom. Screening another symptom must not
  // authorize a separately inserted, unscreened fact.
  const symptom = await completeStructuredSafetyIntake(
    request,
    apiBase,
    authHeaders,
    conversationId,
  );
  expect(symptom.created_revision).toBe(1);

  // Diagnosis: one formal native analysis run, the user's explicit accept
  // and the whole-run settlement adopt the Condition plan generation needs.
  const requestEvidenceId = await factStatementEvidenceId(
    request,
    apiBase,
    authHeaders,
    symptom.id,
  );
  const adopted = await adoptNativeDiagnosis(
    request,
    apiBase,
    authHeaders,
    requestEvidenceId,
  );
  const diagnosis = await expectJson<{
    currentRevision: number;
    conditionEntries: Array<{ conditionId: string; disposition: string }>;
  }>(
    await request.get(`${apiBase}/api/v1/diagnosis`, { headers: authHeaders }),
    "read the adopted native Diagnosis",
  );
  expect(diagnosis.currentRevision).toBe(adopted.revision);
  expect(
    diagnosis.conditionEntries.filter((row) => row.disposition === "adopted"),
  ).not.toHaveLength(0);

  // Generation only proposes: the Treatment Agent's draft becomes a pending
  // agent PlanProposal(create). Nothing is accepted or executable yet.
  const generationCommand = {
    diagnosisRevision: adopted.revision,
    userConstraints: { available_minutes_per_day: 10 },
    changeReason: "initial accepted strategy",
    idempotencyKey: `plan-generation-${conversationId}`,
  };
  const generated = await expectJson<{
    proposalId: string;
    kind: string;
    state: string;
    initiatedBy: string;
    contentFingerprint: string;
    draft: { content: { sessions: unknown[] } };
  }>(
    await request.post(`${apiBase}/api/v1/plan-proposals/generations`, {
      headers: authHeaders,
      data: generationCommand,
      timeout: 60_000,
    }),
    "generate agent PlanProposal",
  );
  expect(generated.kind).toBe("create");
  expect(generated.state).toBe("pending");
  expect(generated.initiatedBy).toBe("agent");
  expect(generated.draft.content.sessions.length).toBeGreaterThan(0);

  // Retrying the same command returns the stored proposal.
  const replayedGeneration = await expectJson<{ proposalId: string }>(
    await request.post(`${apiBase}/api/v1/plan-proposals/generations`, {
      headers: authHeaders,
      data: generationCommand,
      timeout: 60_000,
    }),
    "replay agent PlanProposal generation",
  );
  expect(replayedGeneration.proposalId).toBe(generated.proposalId);

  const pendingWorkspace = await expectJson<{
    training_plan?: { id: string };
    capabilities: { can_start_training: boolean };
  }>(
    await request.get(`${apiBase}/api/v1/health-workspace`, {
      headers: authHeaders,
    }),
    "load HealthWorkspace with a pending proposal",
  );
  expect(pendingWorkspace.training_plan).toBeUndefined();
  expect(pendingWorkspace.capabilities.can_start_training).toBe(false);

  // Only the user's explicit accept writes the plan.
  const acceptance = await expectJson<{
    planId: string;
    revisionId: string;
    proposalId: string;
  }>(
    await request.post(
      `${apiBase}/api/v1/plan-proposals/${generated.proposalId}/accept`,
      {
        headers: authHeaders,
        data: {
          decisionId: crypto.randomUUID(),
          presentedFingerprint: generated.contentFingerprint,
          idempotencyKey: `plan-accept-${generated.proposalId}`,
        },
      },
    ),
    "accept agent PlanProposal",
  );
  expect(acceptance.proposalId).toBe(generated.proposalId);

  const activeWorkspace = await expectJson<{
    training_plan: { id: string; status: string };
    capabilities: { can_start_training: boolean };
    actions: Array<{ kind: string; target?: { route?: string } }>;
  }>(
    await request.get(`${apiBase}/api/v1/health-workspace`, {
      headers: authHeaders,
    }),
    "load active HealthWorkspace",
  );
  expect(activeWorkspace.training_plan.id).toBe(acceptance.planId);
  expect(activeWorkspace.training_plan.status).toBe("active");
  expect(activeWorkspace.capabilities.can_start_training).toBe(true);
  expect(activeWorkspace.actions).toContainEqual(
    expect.objectContaining({
      kind: "open_training",
      target: { route: `/consultation/${conversationId}?view=training` },
    }),
  );

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
  // 390px: the same Trainer in the mobile sheet with 44px touch targets.
  await page.setViewportSize({ width: 390, height: 844 });
  for (const name of ["完成本组", "暂停训练"]) {
    const box = await trainer.getByRole("button", { name }).boundingBox();
    expect(box, name).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  }
  await page.setViewportSize({ width: 1280, height: 720 });

  // The Trainer runs through the Go runtime by explicit commands; the user's
  // feedback is stored verbatim and only surfaces as the plan review hint.
  const feedbackText = "完成温和训练后，肩膀感觉更轻松。";
  const ended = await finishTrainerSessionWithFeedback(
    request,
    apiBase,
    authHeaders,
    acceptance.planId,
    feedbackText,
  );
  expect(ended.feedback?.text).toBe(feedbackText);
  // A body change the user reports is recorded through BodyState, never
  // derived from the session feedback.
  const symptomChange = await recordUserSymptom(
    request,
    apiBase,
    authHeaders,
    "肩膀不适有所改善",
  );
  expect(symptomChange.revision).toBeGreaterThan(symptom.created_revision);

  const finalWorkspace = await expectJson<{
    training_plan: { id: string; status: string };
    capabilities: {
      can_start_training: boolean;
      requires_safety_review: boolean;
    };
  }>(
    await request.get(`${apiBase}/api/v1/health-workspace`, {
      headers: authHeaders,
    }),
    "load reviewed HealthWorkspace",
  );
  // The user's own symptom wording is a new V2 symptom statement at its own
  // revision; the original statement is untouched.
  const records = await expectJson<{
    facts: Array<{
      id: string;
      kind: string;
      statement: string;
      created_revision: number;
    }>;
  }>(
    await request.get(`${apiBase}/api/v1/body-state/records`, {
      headers: authHeaders,
    }),
    "list V2 body facts",
  );
  expect(
    records.facts.find((fact) => fact.statement === "肩膀不适有所改善"),
  ).toMatchObject({
    kind: "symptom",
    created_revision: symptomChange.revision,
  });
  expect(records.facts.some((fact) => fact.id === symptom.id)).toBe(true);
  // The new symptom has not been screened: execution fails closed
  // until its own checklist, without changing the user's plan status.
  expect(finalWorkspace.capabilities.requires_safety_review).toBe(true);
  expect(finalWorkspace.capabilities.can_start_training).toBe(false);
  expect(finalWorkspace.training_plan.id).toBe(acceptance.planId);
  expect(finalWorkspace.training_plan.status).toBe("active");
  // Feedback never changed the plan; it only recommends review.
  const reviewed = await getTrainerPlan(
    request,
    apiBase,
    authHeaders,
    acceptance.planId,
  );
  expect(reviewed.status).toBe("active");
  expect(reviewed.currentAcceptedRevisionId).toBe(acceptance.revisionId);
  expect(reviewed.review.recommended).toBe(true);
  expect(reviewed.review.sessionFeedbackCount).toBe(1);
});
