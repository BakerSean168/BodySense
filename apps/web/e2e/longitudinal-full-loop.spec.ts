import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeStructuredSafetyIntake } from "./support/safety";

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
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);

  const profileResponse = await request.put(`${apiBase}/api/v1/profile`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: {
      gender: "male",
      birth_date: "1996-08-27",
    },
  });
  expect(profileResponse.ok()).toBeTruthy();
  return { accessToken, email };
}

test("BodyState -> Diagnosis -> Treatment -> Training -> Outcome closes the longitudinal loop", async ({
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

  const diagnosis = await expectJson<{
    analysis_id: string;
    body_state_revision: number;
    candidates: Array<{ candidate_id: string; concern_key: string }>;
    freshness: { state: string };
  }>(
    await request.post(
      `${apiBase}/api/v1/consultations/${conversationId}/diagnosis`,
      { headers: authHeaders, timeout: 60_000 },
    ),
    "generate DiagnosisAnalysis",
  );
  expect(diagnosis.candidates).toHaveLength(1);
  // Chat intake preserves the statement without guessing a canonical region.
  expect(diagnosis.candidates[0].concern_key).toBe("general");
  expect(diagnosis.freshness.state).toBe("fresh");

  await expectJson(
    await request.put(
      `${apiBase}/api/v1/diagnosis-analyses/${diagnosis.analysis_id}/assessment`,
      {
        headers: authHeaders,
        data: {
          candidates: [
            {
              candidate_id: diagnosis.candidates[0].candidate_id,
              state: "confirmed",
            },
          ],
        },
      },
    ),
    "assess Diagnosis candidate",
  );

  const proposalResponse = await expectJson<{
    proposal: {
      id: string;
      acceptance_state: string;
      source_diagnosis_analysis_id: string;
      source_body_state_revision: number;
      agent_configuration_id: string;
    };
  }>(
    await request.post(`${apiBase}/api/v1/treatments/proposals`, {
      headers: authHeaders,
      data: {
        diagnosis_analysis_id: diagnosis.analysis_id,
        user_constraints: { available_minutes_per_day: 10 },
        change_reason: "initial accepted strategy",
      },
      timeout: 60_000,
    }),
    "generate Treatment proposal",
  );
  expect(proposalResponse.proposal.acceptance_state).toBe("proposed");
  expect(proposalResponse.proposal.source_diagnosis_analysis_id).toBe(
    diagnosis.analysis_id,
  );

  const historicalReplay = await expectJson<{
    mode: string;
    artifact_integrity: { match: boolean };
    comparison: {
      hard: { match: boolean };
      semantic: { match: boolean };
      presentation: { match: boolean };
    };
  }>(
    await request.post(
      `${apiBase}/api/v1/treatments/revisions/${proposalResponse.proposal.id}/replay`,
      { headers: authHeaders, data: { mode: "historical" } },
    ),
    "historical Treatment replay",
  );
  expect(historicalReplay.mode).toBe("historical");
  expect(historicalReplay.artifact_integrity.match).toBe(true);
  expect(historicalReplay.comparison.hard.match).toBe(true);
  expect(historicalReplay.comparison.semantic.match).toBe(true);
  expect(historicalReplay.comparison.presentation.match).toBe(true);

  const counterfactualTarget = proposalResponse.proposal.agent_configuration_id;
  const counterfactualReplay = await expectJson<{
    mode: string;
    target_configuration_id: string;
    comparison: { hard: { match: boolean } };
  }>(
    await request.post(
      `${apiBase}/api/v1/treatments/revisions/${proposalResponse.proposal.id}/replay`,
      {
        headers: authHeaders,
        data: {
          mode: "counterfactual",
          configuration_id: counterfactualTarget,
        },
        timeout: 60_000,
      },
    ),
    "current-config Treatment counterfactual replay",
  );
  expect(counterfactualReplay.mode).toBe("counterfactual");
  expect(counterfactualReplay.target_configuration_id).toBe(
    counterfactualTarget,
  );
  expect(counterfactualReplay.comparison.hard.match).toBe(true);

  const retiredReplay = await request.post(
    `${apiBase}/api/v1/treatments/revisions/${proposalResponse.proposal.id}/replay`,
    {
      headers: authHeaders,
      data: {
        mode: "counterfactual",
        configuration_id: "treat-config-85718f8e90ac9d80",
      },
    },
  );
  expect(retiredReplay.status()).toBe(422);
  expect(await retiredReplay.json()).toMatchObject({
    error: { code: "UNKNOWN_AGENT_CONFIGURATION" },
  });

  const regressionExport = await expectJson<{
    schema_target: string;
    case: { inputs: { body_state_revision: number } };
  }>(
    await request.get(
      `${apiBase}/api/v1/treatments/revisions/${proposalResponse.proposal.id}/regression-export`,
      { headers: authHeaders },
    ),
    "Treatment regression export",
  );
  expect(regressionExport.schema_target).toBe("treatment_qualification_v1");
  expect(regressionExport.case.inputs.body_state_revision).toBe(
    proposalResponse.proposal.source_body_state_revision,
  );

  const proposalAfterReplay = await expectJson<{ acceptance_state: string }>(
    await request.get(
      `${apiBase}/api/v1/treatments/revisions/${proposalResponse.proposal.id}`,
      { headers: authHeaders },
    ),
    "Treatment proposal after replay",
  );
  expect(proposalAfterReplay.acceptance_state).toBe("proposed");

  const acceptance = await expectJson<{
    treatment: { status: string; current: { id: string } };
    training_plan: {
      id: string;
      status: string;
      treatment_revision_id: string;
    };
  }>(
    await request.post(
      `${apiBase}/api/v1/treatments/revisions/${proposalResponse.proposal.id}/accept`,
      {
        headers: authHeaders,
        data: { consultation_id: conversationId },
      },
    ),
    "accept Treatment and create TrainingPlan",
  );
  expect(acceptance.treatment.status).toBe("active");
  expect(acceptance.training_plan.status).toBe("active");
  expect(acceptance.training_plan.treatment_revision_id).toBe(
    proposalResponse.proposal.id,
  );

  const activeWorkspace = await expectJson<{
    training_plan: { id: string };
    capabilities: { can_execute_treatment: boolean };
    actions: Array<{ kind: string; target?: { route?: string } }>;
  }>(
    await request.get(`${apiBase}/api/v1/health-workspace`, {
      headers: authHeaders,
    }),
    "load active HealthWorkspace",
  );
  expect(activeWorkspace.training_plan.id).toBe(acceptance.training_plan.id);
  expect(activeWorkspace.capabilities.can_execute_treatment).toBe(true);
  expect(activeWorkspace.actions).toContainEqual(
    expect.objectContaining({
      kind: "open_training",
      target: { route: `/consultation/${conversationId}?view=treatment` },
    }),
  );

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

  const checkIn = await request.post(
    `${apiBase}/api/v1/training/${acceptance.training_plan.id}/checkin`,
    { headers: authHeaders },
  );
  expect(
    checkIn.ok(),
    `training check-in failed with ${checkIn.status()}: ${await checkIn.text()}`,
  ).toBeTruthy();

  const feedback = await expectJson<{
    result: {
      outcome: { id: string; body_state_revision: number };
      treatment_status: string;
      review_recommended: boolean;
    };
  }>(
    await request.put(
      `${apiBase}/api/v1/training/${acceptance.training_plan.id}/log`,
      {
        headers: authHeaders,
        data: {
          notes: "完成温和训练后，肩膀感觉更轻松。",
          exercises: [{ name: "下巴微收", completed: true }],
          symptom_changes: "肩膀不适有所改善",
          training_feeling: "轻松",
          difficulties: "",
          body_region: "肩部",
          concern_key: "general",
          trend: "improving",
        },
      },
    ),
    "record symptom feedback Outcome",
  );
  expect(feedback.result.outcome.body_state_revision).toBeGreaterThan(
    diagnosis.body_state_revision,
  );
  expect(feedback.result.review_recommended).toBe(true);
  expect(feedback.result.treatment_status).toBe("review_recommended");

  const finalWorkspace = await expectJson<{
    treatment: { status: string };
    capabilities: {
      can_execute_treatment: boolean;
      requires_treatment_review: boolean;
    };
    recent_outcomes: Array<{
      id: string;
      body_state_revision: number | null;
      causality_level: string;
    }>;
  }>(
    await request.get(`${apiBase}/api/v1/health-workspace`, {
      headers: authHeaders,
    }),
    "load reviewed HealthWorkspace",
  );
  // The user's own symptom wording became a new V2 symptom statement at the
  // Outcome's revision; the original statement is untouched (no legacy trend).
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
    created_revision: feedback.result.outcome.body_state_revision,
  });
  expect(records.facts.some((fact) => fact.id === symptom.id)).toBe(true);
  // The new Outcome symptom has not been screened: fail closed until its own checklist.
  expect(finalWorkspace.treatment.status).toBe("paused");
  expect(finalWorkspace.capabilities.requires_treatment_review).toBe(true);
  expect(finalWorkspace.capabilities.can_execute_treatment).toBe(false);
  expect(
    finalWorkspace.recent_outcomes.some(
      (outcome) =>
        outcome.id === feedback.result.outcome.id &&
        outcome.body_state_revision != null &&
        outcome.causality_level === "association_only",
    ),
  ).toBe(true);
});
