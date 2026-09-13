import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { assessmentApi } from "./assessmentService";

const jsonResponse = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });

const reportID = "11111111-1111-4111-8111-111111111111";
const userID = "22222222-2222-4222-8222-222222222222";
const observationID = "33333333-3333-4333-8333-333333333333";

function coverage() {
  return {
    status: "partial",
    available_sources: ["body_state"],
    domains: {
      posture: { status: "missing", evidence_refs: [] },
      exercise: { status: "available", evidence_refs: ["body_state:fact:0"] },
      lifestyle: { status: "missing", evidence_refs: [] },
      anthropometry: { status: "missing", evidence_refs: [] },
      health_report: { status: "missing", evidence_refs: [] },
      injury_symptoms: { status: "missing", evidence_refs: [] },
    },
  };
}

function v2Report() {
  return {
    id: reportID,
    user_id: userID,
    status: "completed",
    contract_revision: "assessment-output-v2",
    evidence_coverage: coverage(),
    evidence_gaps: [
      {
        dimension: "posture",
        required: false,
        description: "当前未提供已完成的体态分析。",
        needed_sources: ["posture_analysis"],
      },
    ],
    observations: [
      {
        observation_id: observationID,
        review_state: "unverified",
        kind: "exercise_pattern",
        body_region: "",
        label: "运动记录",
        description: "每周训练。",
        method: "assessment_evidence",
        evidence_refs: ["body_state:fact:0"],
      },
    ],
    summary: "当前资料支持 1 项待审核观察。",
    information_gaps: [],
    safety_notes: ["不构成医疗诊断。"],
    body_state_revision: 4,
    agent_configuration_id: "assessment-v5",
    agent_configuration: { id: "assessment-v5", role: "assessment" },
    execution_provenance: { status: "executed" },
    generation_decision_trace: { status: "generated" },
    created_at: "2026-09-13T09:00:00Z",
  };
}

function v1Report() {
  return {
    id: "44444444-4444-4444-8444-444444444444",
    user_id: userID,
    status: "completed",
    contract_revision: "assessment-output-v1",
    health_grade: "B",
    dimension_scores: {
      posture: 70,
      exercise: 80,
      lifestyle: 65,
      injury_risk: 60,
      overall: 69,
    },
    evidence_coverage: {},
    evidence_gaps: [],
    observations: [
      {
        kind: "posture",
        label: "历史观察",
        description: "历史 v1 观察。",
      },
    ],
    summary: "历史报告。",
    information_gaps: [],
    safety_notes: [],
    agent_configuration_id: "assessment-v2",
    agent_configuration: { id: "assessment-v2", role: "assessment" },
    execution_provenance: { status: "executed" },
    generation_decision_trace: {},
    created_at: "2026-08-01T09:00:00Z",
  };
}

describe("assessment OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("generates only an evidence-grounded v2 report through the generated boundary", async () => {
    authFetchMock.mockResolvedValue(jsonResponse(v2Report(), 201));

    const report = await assessmentApi.generate();

    expect(report.contract_revision).toBe("assessment-output-v2");
    expect(report.observations[0]?.evidence_refs).toEqual([
      "body_state:fact:0",
    ]);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/assessment/generate",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("fails closed when v2 evidence coverage omits a canonical domain", async () => {
    const invalid = v2Report();
    delete (
      invalid.evidence_coverage.domains as Partial<
        typeof invalid.evidence_coverage.domains
      >
    ).injury_symptoms;
    authFetchMock.mockResolvedValue(jsonResponse(invalid));

    await expect(assessmentApi.getReport(reportID)).rejects.toThrow();
  });

  it("validates both historical v1 and current v2 reports in list responses", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({
        reports: [v2Report(), v1Report()],
        total: 2,
        limit: 20,
        offset: 0,
      }),
    );

    const result = await assessmentApi.listReports();

    expect(result.total).toBe(2);
    expect(result.reports.map((report) => report.contract_revision)).toEqual([
      "assessment-output-v2",
      "assessment-output-v1",
    ]);
  });

  it("uses generated query serialization for pagination", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({ reports: [], total: 0, limit: 10, offset: 20 }),
    );

    await assessmentApi.listReports({ limit: 10, offset: 20 });

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/assessment?limit=10&offset=20",
      expect.objectContaining({ method: "GET" }),
    );
  });
});
