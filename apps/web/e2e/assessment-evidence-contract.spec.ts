import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

test("assessment v2 persists only deterministic source-grounded observations", async ({
  page,
  request,
}) => {
  const email = `assessment-evidence-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };

  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: { gender: "male", birth_date: "1996-08-27" },
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  const lifestyle = await request.put(`${apiBase}/api/v1/lifestyle`, {
    headers,
    data: {
      expected_revision: 0,
      activity: {
        summary: "日常活动以久坐为主",
        details: { source: "assessment-evidence-e2e" },
      },
    },
  });
  expect(lifestyle.ok(), await lifestyle.text()).toBeTruthy();

  const beforeState = await request.get(`${apiBase}/api/v1/body-state`, {
    headers,
  });
  expect(beforeState.ok(), await beforeState.text()).toBeTruthy();
  const before = (await beforeState.json()) as {
    current_revision: number;
    facts: Array<{ id: string; kind: string; value: unknown }>;
  };
  const activityFact = before.facts.find(
    (fact) => fact.kind === "lifestyle.activity",
  );
  expect(activityFact).toBeTruthy();
  expect(before.current_revision).toBe(1);

  const generated = await request.post(
    `${apiBase}/api/v1/assessment/generate`,
    { headers },
  );
  const generatedText = await generated.text();
  expect(
    generated.status(),
    `assessment generation failed: ${generatedText}`,
  ).toBe(201);

  const report = JSON.parse(generatedText) as {
    contract_revision: string;
    status: string;
    observations: Array<{
      kind: string;
      label: string;
      description: string;
      evidence_refs: string[];
    }>;
    summary: string;
    evidence_coverage: {
      status: string;
      domains: Record<string, { status: string; evidence_refs: string[] }>;
    };
    evidence_gaps: Array<{ dimension: string; required: boolean }>;
    execution_provenance: { status: string; usage: { requests: number } };
    generation_decision_trace: {
      status: string;
      phase: string;
      model_executed: boolean;
      execution_status: string;
    };
    agent_configuration_id: string;
    agent_configuration: { evidence_policy_revision: string };
    health_grade?: unknown;
    dimension_scores?: unknown;
  };

  expect(report.contract_revision).toBe("assessment-output-v2");
  expect(report.agent_configuration_id).toBe("assess-config-e579030c2b8b540c");
  expect(report.agent_configuration.evidence_policy_revision).toBe(
    "assessment-evidence-contract-v3",
  );
  expect(report.status).toBe("completed");
  expect(report.health_grade).toBeUndefined();
  expect(report.dimension_scores).toBeUndefined();
  expect(report.evidence_coverage.status).toBe("partial");
  expect(report.evidence_coverage.domains.lifestyle.status).toBe("available");
  expect(report.evidence_coverage.domains.posture.status).toBe("missing");
  expect(report.evidence_coverage.domains.injury_symptoms.status).toBe(
    "missing",
  );
  expect(report.evidence_coverage.domains.health_report.status).toBe("missing");
  expect(
    report.evidence_gaps.every((gap) => gap.required === false),
  ).toBeTruthy();
  expect(report.summary).toContain("1/6 个证据领域已有资料");
  expect(report.execution_provenance.status).toBe("executed");
  expect(report.execution_provenance.usage.requests).toBeGreaterThanOrEqual(1);
  expect(report.generation_decision_trace.status).toBe("generated");
  expect(report.generation_decision_trace.phase).toBe("generation");
  expect(report.generation_decision_trace.model_executed).toBe(true);
  expect(report.generation_decision_trace.execution_status).toBe("executed");

  const expectedRef = `body_state:fact:${activityFact?.id}`;
  for (const observation of report.observations) {
    expect(observation.kind).toBe("lifestyle_pattern");
    expect(observation.evidence_refs).toEqual([expectedRef]);
    expect(observation.label).toBe("日常活动记录");
    expect(observation.description).toBe(
      `来源记录：${String(activityFact?.value)}。`,
    );
    expect(observation.description).not.toContain("可能");
    expect(observation.description).not.toContain("建议");
  }
  expect(
    report.observations.some((observation) =>
      observation.kind.startsWith("posture"),
    ),
  ).toBeFalsy();

  // GET /body-state is the confirmed reasoning projection and intentionally hides
  // unverified Assessment candidates. The health workspace is the product read
  // model that exposes pending observations for explicit user review.
  const workspaceResponse = await request.get(
    `${apiBase}/api/v1/health-workspace`,
    {
      headers,
    },
  );
  expect(workspaceResponse.ok(), await workspaceResponse.text()).toBeTruthy();
  const workspace = (await workspaceResponse.json()) as {
    body_state: {
      current_revision: number;
      pending_observations: Array<{
        kind: string;
        value?: { label?: string; description?: string };
        provenance?: { source_type?: string; evidence_selection?: unknown };
      }>;
    };
  };
  expect(workspace.body_state.current_revision).toBe(
    before.current_revision + report.observations.length,
  );
  const assessmentObservations =
    workspace.body_state.pending_observations.filter(
      (observation) => observation.provenance?.source_type === "assessment",
    );
  expect(assessmentObservations).toHaveLength(report.observations.length);
  for (const observation of assessmentObservations) {
    expect(observation.kind).toBe("lifestyle_pattern");
    expect(observation.value?.description).toBe(
      `来源记录：${String(activityFact?.value)}。`,
    );
  }
});

test("assessment v2 derives insufficient information without any model request", async ({
  page,
  request,
}) => {
  const email = `assessment-empty-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";

  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };

  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: { gender: "female", birth_date: "1996-08-27" },
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  const beforeState = await request.get(`${apiBase}/api/v1/body-state`, {
    headers,
  });
  expect(beforeState.ok(), await beforeState.text()).toBeTruthy();
  const before = (await beforeState.json()) as { current_revision: number };

  const generated = await request.post(
    `${apiBase}/api/v1/assessment/generate`,
    { headers },
  );
  const generatedText = await generated.text();
  expect(generated.status(), generatedText).toBe(201);

  const report = JSON.parse(generatedText) as {
    contract_revision: string;
    status: string;
    observations: unknown[];
    evidence_coverage: {
      status: string;
      available_sources: string[];
      domains: Record<string, { status: string; evidence_refs: string[] }>;
    };
    evidence_gaps: Array<{ required: boolean }>;
    summary: string;
    execution_provenance: {
      status: string;
      runtime: string;
      usage: { requests: number };
    };
    generation_decision_trace: {
      status: string;
      phase: string;
      model_executed: boolean;
      execution_status: string;
    };
    agent_configuration_id: string;
    agent_configuration: { evidence_policy_revision: string };
  };

  expect(report.contract_revision).toBe("assessment-output-v2");
  expect(report.agent_configuration_id).toBe("assess-config-e579030c2b8b540c");
  expect(report.agent_configuration.evidence_policy_revision).toBe(
    "assessment-evidence-contract-v3",
  );
  expect(report.status).toBe("insufficient_information");
  expect(report.observations).toEqual([]);
  expect(report.evidence_coverage.status).toBe("insufficient");
  expect(report.evidence_coverage.available_sources).toEqual([]);
  expect(
    Object.values(report.evidence_coverage.domains).every(
      (domain) =>
        domain.status === "missing" && domain.evidence_refs.length === 0,
    ),
  ).toBeTruthy();
  expect(report.evidence_gaps).toHaveLength(6);
  expect(
    report.evidence_gaps.every((gap) => gap.required === false),
  ).toBeTruthy();
  expect(report.summary).toContain("0/6 个证据领域已有资料");
  expect(report.execution_provenance.status).toBe("skipped_no_evidence");
  expect(report.execution_provenance.runtime).toBe("deterministic");
  expect(report.execution_provenance.usage.requests).toBe(0);
  expect(report.generation_decision_trace.status).toBe("derived_without_model");
  expect(report.generation_decision_trace.phase).toBe(
    "deterministic_derivation",
  );
  expect(report.generation_decision_trace.model_executed).toBe(false);
  expect(report.generation_decision_trace.execution_status).toBe(
    "skipped_no_evidence",
  );

  const afterState = await request.get(`${apiBase}/api/v1/body-state`, {
    headers,
  });
  expect(afterState.ok(), await afterState.text()).toBeTruthy();
  const after = (await afterState.json()) as {
    current_revision: number;
    observations: unknown[];
  };
  expect(after.current_revision).toBe(before.current_revision);
  expect(after.observations).toEqual([]);
});
