import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
} from "@playwright/test";

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

  const accessToken = await page.evaluate(() => {
    const raw = localStorage.getItem("auth-storage");
    if (!raw) return "";
    const parsed = JSON.parse(raw) as { state?: { accessToken?: string } };
    return parsed.state?.accessToken || "";
  });
  expect(accessToken).not.toBe("");

  const profileResponse = await request.put(`${apiBase}/api/v1/profile`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: {
      gender: "male",
      age: 30,
      height_cm: 175,
      weight_kg: 70,
      occupation: "software engineer",
      exercise_frequency: "2-3/week",
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

  const conversations = await expectJson<{
    conversations: Array<{ id: string }>;
  }>(
    await request.get(`${apiBase}/api/v1/conversations?limit=20`, {
      headers: authHeaders,
    }),
    "list conversations",
  );
  expect(conversations.conversations).toHaveLength(1);
  const conversationId = conversations.conversations[0].id;

  const factResponse = await expectJson<{
    fact: { id: string; concern_key: string; trend: string };
    revision: { revision: number };
  }>(
    await request.post(`${apiBase}/api/v1/body-state/facts`, {
      headers: authHeaders,
      data: {
        expected_revision: 0,
        fact: {
          concern_key: "region:颈肩",
          kind: "discomfort",
          body_region: "颈肩",
          value: "久坐后颈肩酸胀",
          details: {
            duration: "2周",
            trigger: "久坐",
            severity: "轻度",
          },
          origin: "user_reported",
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: "stable",
          source_key: `e2e-fact-${crypto.randomUUID()}`,
          provenance: { source_type: "playwright-e2e" },
          observed_at: new Date().toISOString(),
        },
      },
    }),
    "create BodyState fact",
  );
  expect(factResponse.revision.revision).toBe(1);
  expect(factResponse.fact.concern_key).toBe("region:颈肩");

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
  expect(diagnosis.candidates[0].concern_key).toBe("region:颈肩");
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

  const treatmentV1 = "treat-config-85718f8e90ac9d80";
  const treatmentV2 = "treat-config-f68eec9846664596";
  const counterfactualTarget =
    proposalResponse.proposal.agent_configuration_id === treatmentV1
      ? treatmentV2
      : treatmentV1;
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
    "counterfactual Treatment replay",
  );
  expect(counterfactualReplay.mode).toBe("counterfactual");
  expect(counterfactualReplay.target_configuration_id).toBe(
    counterfactualTarget,
  );
  expect(counterfactualReplay.comparison.hard.match).toBe(true);

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
      target: { route: `/training/${acceptance.training_plan.id}` },
    }),
  );

  await page.goto("/dashboard");
  const trainingAction = page.getByRole("button", { name: "继续执行训练" });
  await expect(trainingAction).toBeVisible();
  await page.reload();
  await expect(trainingAction).toBeVisible();
  await trainingAction.click();
  await expect(page).toHaveURL(
    new RegExp(`/training/${acceptance.training_plan.id}$`),
  );
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
          notes: "完成温和训练后，颈肩感觉更轻松。",
          exercises: [{ name: "下巴微收", completed: true }],
          symptom_changes: "久坐后的颈肩酸胀有所改善",
          training_feeling: "轻松",
          difficulties: "",
          body_region: "颈肩",
          concern_key: "region:颈肩",
          trend: "improving",
          fact_id: factResponse.fact.id,
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
    body_state: {
      current_revision: number;
      facts: Array<{ id: string; trend: string }>;
    };
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
  expect(finalWorkspace.body_state.current_revision).toBeGreaterThan(
    diagnosis.body_state_revision,
  );
  expect(
    finalWorkspace.body_state.facts.find(
      (fact) => fact.id === factResponse.fact.id,
    )?.trend,
  ).toBe("improving");
  expect(finalWorkspace.treatment.status).toBe("review_recommended");
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
