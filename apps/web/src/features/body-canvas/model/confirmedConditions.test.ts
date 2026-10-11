import { describe, expect, it } from "vitest";
import { projectDiagnosisView, CERTAINTY_LABELS } from "./confirmedConditions";
import {
  candidate,
  makeDiagnosis,
  makeWorkspace,
} from "./__fixtures__/diagnosisFixtures";
import type { TreatmentRevision } from "@/features/workspace/types/workspace";

function acceptedRevision(
  overrides: Partial<TreatmentRevision> = {},
): TreatmentRevision {
  return {
    id: "rev-1",
    treatment_id: "treatment-1",
    revision: 1,
    acceptance_state: "accepted",
    lifecycle_state: "active",
    source_body_state_revision: 3,
    source_diagnosis_analysis_id: "analysis-1",
    goal: "缓解肩颈紧张",
    duration_weeks: 4,
    plan: {
      summary: "渐进式肩胛控制计划",
      goal: "缓解肩颈紧张",
      duration_weeks: 4,
      interventions: [],
      daily_habits: [],
      expected_timeline: "",
      warning_signs: [],
      review_triggers: [],
      safety_notes: [],
    },
    user_constraints: {},
    evidence_ids: [],
    governance: {},
    change_reason: "",
    created_at: "2026-10-11T00:00:00Z",
    interventions: [
      {
        id: "int-1",
        treatment_id: "treatment-1",
        treatment_revision_id: "rev-1",
        kind: "exercise",
        title: "靠墙肩胛滑动",
        description: "缓慢控制",
        prescription: { sets: 3, reps: 10 },
        position: 0,
        status: "active",
        created_at: "2026-10-11T00:00:00Z",
        updated_at: "2026-10-11T00:00:00Z",
      },
    ],
    ...overrides,
  };
}

describe("diagnosis view only lists confirmed conditions", () => {
  it("returns an empty view when no analysis exists", () => {
    expect(projectDiagnosisView(undefined, undefined).confirmed).toEqual([]);
    expect(projectDiagnosisView(undefined, null).confirmed).toEqual([]);
  });

  it("excludes candidates the user never confirmed (DX-06)", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" }), candidate({ candidate_id: "b" })],
      candidate_assessments: [],
    });
    const view = projectDiagnosisView(makeWorkspace(), diagnosis);
    expect(view.confirmed).toEqual([]);
    expect(view.pendingProposals.map((p) => p.conditionId)).toEqual(["a", "b"]);
  });

  it("excludes explicitly not-applicable candidates entirely", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "not_applicable" }],
    });
    const view = projectDiagnosisView(makeWorkspace(), diagnosis);
    expect(view.confirmed).toEqual([]);
    expect(view.pendingProposals).toEqual([]);
  });

  it("keeps unsure candidates as pending proposals, not confirmed conditions", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "unsure" }],
    });
    const view = projectDiagnosisView(makeWorkspace(), diagnosis);
    expect(view.confirmed).toEqual([]);
    expect(view.pendingProposals).toHaveLength(1);
  });

  it("lists a confirmed candidate with cause, basis and evidence boundary", () => {
    const diagnosis = makeDiagnosis({
      candidates: [
        candidate({
          candidate_id: "a",
          reasoning_summary: "长期久坐使肩胛稳定肌群持续紧张",
          differential: "需排除肩袖损伤",
          evidence_strength: "中",
          counterevidence_ids: ["neg-1", "neg-2"],
          missing_information: ["夜间痛是否加重"],
          basis_fact_ids: ["fact-1"],
          safety_notes: ["出现麻木时停止训练"],
        }),
      ],
      candidate_assessments: [{ candidate_id: "a", state: "confirmed" }],
    });
    const [condition] = projectDiagnosisView(makeWorkspace(), diagnosis).confirmed;
    expect(condition.name).toBe("上交叉综合征倾向");
    expect(condition.cause.summary).toBe("长期久坐使肩胛稳定肌群持续紧张");
    expect(condition.cause.basis).toBe("久坐后肩颈酸胀");
    expect(condition.cause.differential).toBe("需排除肩袖损伤");
    expect(condition.cause.counterEvidenceCount).toBe(2);
    expect(condition.cause.informationGaps).toEqual(["夜间痛是否加重"]);
    expect(condition.cause.basisFactIds).toEqual(["fact-1"]);
    expect(condition.safetyNotes).toEqual(["出现麻木时停止训练"]);
  });

  it("never claims clinical documentation for a user-confirmed analysis (DX-07)", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "confirmed" }],
    });
    const [condition] = projectDiagnosisView(makeWorkspace(), diagnosis).confirmed;
    expect(condition.certainty).toBe("user_confirmed_agent_analysis");
    expect(CERTAINTY_LABELS[condition.certainty]).toBe("你已确认的分析结果");
  });

  it("leaves handling unconfirmed when no accepted plan covers this analysis", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "confirmed" }],
    });
    const [condition] = projectDiagnosisView(makeWorkspace(), diagnosis).confirmed;
    expect(condition.handling.state).toBe("not_confirmed");
  });

  it("attaches plan-level handling only for a matching accepted revision", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "confirmed" }],
    });
    const workspace = makeWorkspace({
      treatment: {
        id: "treatment-1",
        current_revision: 1,
        status: "active",
        status_reasons: [],
        created_at: "2026-10-11T00:00:00Z",
        updated_at: "2026-10-11T00:00:00Z",
        current: acceptedRevision(),
      },
    });
    const [condition] = projectDiagnosisView(workspace, diagnosis).confirmed;
    expect(condition.handling.state).toBe("plan_level");
    expect(condition.handling.planGoal).toBe("缓解肩颈紧张");
    expect(condition.handling.actions).toEqual(["靠墙肩胛滑动"]);
  });

  it("does not attach a proposal (unaccepted) revision as handling", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "confirmed" }],
    });
    const workspace = makeWorkspace({
      treatment: {
        id: "treatment-1",
        current_revision: 1,
        status: "active",
        status_reasons: [],
        created_at: "2026-10-11T00:00:00Z",
        updated_at: "2026-10-11T00:00:00Z",
        current: acceptedRevision({ acceptance_state: "proposed" }),
      },
    });
    const [condition] = projectDiagnosisView(workspace, diagnosis).confirmed;
    expect(condition.handling.state).toBe("not_confirmed");
  });

  it("does not attach handling generated from a different analysis", () => {
    const diagnosis = makeDiagnosis({
      candidates: [candidate({ candidate_id: "a" })],
      candidate_assessments: [{ candidate_id: "a", state: "confirmed" }],
    });
    const workspace = makeWorkspace({
      treatment: {
        id: "treatment-1",
        current_revision: 1,
        status: "active",
        status_reasons: [],
        created_at: "2026-10-11T00:00:00Z",
        updated_at: "2026-10-11T00:00:00Z",
        current: acceptedRevision({ source_diagnosis_analysis_id: "other" }),
      },
    });
    const [condition] = projectDiagnosisView(workspace, diagnosis).confirmed;
    expect(condition.handling.state).toBe("not_confirmed");
  });

  it("surfaces review need and safety block instead of hiding them", () => {
    const diagnosis = makeDiagnosis({
      status: "safety_blocked",
      freshness: {
        analysis_id: "analysis-1",
        state: "stale",
        evaluated_against_revision: 2,
        reasons: [{ code: "new_fact", message: "出现了新的安全相关记录" }],
        checked_at: "2026-10-11T00:00:00Z",
      },
    });
    const view = projectDiagnosisView(makeWorkspace(), diagnosis);
    expect(view.needsReview).toBe(true);
    expect(view.reviewReasons).toEqual(["出现了新的安全相关记录"]);
    expect(view.safetyBlocked).toBe(true);
  });

  it("treats a fresh analysis as not needing review", () => {
    const view = projectDiagnosisView(makeWorkspace(), makeDiagnosis());
    expect(view.needsReview).toBe(false);
    expect(view.reviewReasons).toEqual([]);
  });
});
