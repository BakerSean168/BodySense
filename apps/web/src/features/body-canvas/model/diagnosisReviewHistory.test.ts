import { describe, expect, it } from "vitest";
import { projectConditionReview, projectRunReview } from "./diagnosisReview";
import { projectDiagnosisHistory } from "./diagnosisHistory";
import { projectSafetyRestrictions } from "./safetyRestrictions";
import { projectNativeDiagnosisWindow } from "./diagnosisDecisionViewModel";
import {
  CONDITION_ID,
  RUN_ID,
  makeCondition,
  makeDiagnosis,
  makeProposal,
  makeRun,
} from "./__fixtures__/nativeDiagnosisFixtures";

const FACT_1 = "00000000-0000-4000-8000-0000000000f1";
const FACT_2 = "00000000-0000-4000-8000-0000000000f2";

function freshness(
  state: "current" | "review_recommended" | "stale",
  reasons: ReturnType<typeof makeCondition>["freshness"]["reasons"] = [],
) {
  return {
    state,
    basedOnBodyStateRevision: 1,
    evaluatedBodyStateRevision: 4,
    evaluatedDiagnosisRevision: 2,
    reasons,
  };
}

describe("Diagnosis freshness review prompt", () => {
  it("counts new related records like the Penpot Current board", () => {
    const review = projectConditionReview(
      makeCondition({
        freshness: freshness("review_recommended", [
          {
            code: "related_fact_recorded",
            bodyStateRevision: 3,
            evidenceId: FACT_1,
          },
          {
            code: "related_fact_recorded",
            bodyStateRevision: 4,
            evidenceId: FACT_2,
          },
        ]),
      }),
    );
    expect(review).toMatchObject({
      state: "review_recommended",
      prompt: true,
      canDismiss: true,
      title: "2 条新记录可能相关",
      evaluatedBodyStateRevision: 4,
    });
  });

  it("hides a dismissed prompt but never a stale one", () => {
    const dismissed = projectConditionReview(
      makeCondition({
        freshness: freshness("review_recommended", [
          { code: "safety_changed", bodyStateRevision: 3 },
        ]),
        reviewPromptDismissal: {
          dismissalId: FACT_1,
          dismissedThroughBodyStateRevision: 4,
          dismissedAt: "2026-10-19T08:00:00+08:00",
        },
      }),
    );
    expect(dismissed).toMatchObject({
      prompt: false,
      dismissed: true,
      canDismiss: false,
    });

    const stale = projectConditionReview(
      makeCondition({
        freshness: freshness("stale", [
          {
            code: "cited_evidence_withdrawn",
            bodyStateRevision: 3,
            evidenceId: FACT_1,
          },
        ]),
      }),
    );
    expect(stale).toMatchObject({
      prompt: true,
      canDismiss: false,
      title: "依据有变化，需要重新分析",
    });
    expect(stale.details).toEqual(["引用的记录已被撤回"]);
    expect(projectConditionReview(makeCondition()).prompt).toBe(false);
  });

  it("makes a stale open run read-only and keeps the receipt rule", () => {
    const run = makeRun({
      freshness: freshness("stale", [
        {
          code: "condition_changed",
          conditionId: CONDITION_ID,
          diagnosisRevision: 2,
        },
      ]),
    });
    expect(projectRunReview(run)?.state).toBe("stale");
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run,
    });
    expect(view.run?.changed).toBe(true);
    expect(view.run?.canDecide).toBe(false);
    const recommended = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun({
        freshness: freshness("review_recommended", [
          { code: "safety_changed", bodyStateRevision: 3 },
        ]),
      }),
    });
    expect(recommended.run?.review?.state).toBe("review_recommended");
    expect(recommended.run?.canDecide).toBe(true);
  });

  it("keeps inactive entries out of the current list", () => {
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis({
        conditionEntries: [makeCondition({ disposition: "inactive" })],
      }),
      run: null,
    });
    expect(view.confirmed).toEqual([]);
    expect(view.inactive.map((item) => item.disposition)).toEqual(["inactive"]);
  });
});

describe("Diagnosis History", () => {
  it("lists only decided proposals, committed adoption only with its receipt", () => {
    const decided = (
      id: string,
      decision: "accept" | "reject",
      at: string,
    ) => ({
      decisionId: id,
      decision,
      contentFingerprint: "a".repeat(64),
      receiptId: id,
      decidedAt: at,
    });
    const committed = makeRun({
      state: "committed",
      outcome: "committed",
      settlement: {
        settlementId: FACT_1,
        outcome: "committed",
        receiptId: FACT_1,
        settledAt: "2026-10-13T10:00:00+08:00",
      } as never,
      proposals: [
        makeProposal({
          state: "decided",
          decision: decided(FACT_1, "accept", "2026-10-13T09:00:00+08:00"),
        }),
        makeProposal({
          proposalId: FACT_2,
          state: "decided",
          content: { proposedEntry: { medicalName: "颈椎病" } },
          decision: decided(FACT_2, "reject", "2026-10-13T09:01:00+08:00"),
        }),
      ],
    });
    const open = makeRun({ runId: CONDITION_ID, state: "open" });
    const entries = projectDiagnosisHistory(
      makeDiagnosis({
        conditionEntries: [
          makeCondition({
            conditionId: FACT_2,
            medicalName: "紧张型头痛",
            disposition: "inactive",
            updatedAt: "2026-09-20T10:00:00+08:00",
          }),
        ],
      }),
      [committed, open],
    );
    expect(
      entries.map((entry) => [
        entry.title,
        entry.outcomeLabel,
        entry.levelLabel,
        entry.note,
      ]),
    ).toEqual([
      ["颈椎病", "未采纳", null, "提案已拒绝"],
      ["上交叉综合征", "采纳", "AI 疑似", null],
      ["紧张型头痛", "停用", null, null],
    ]);
    expect(entries[0].dateLabel).toBe("10月13日");
    expect(entries[2].dateLabel).toBe("9月20日");
  });

  it("never lists an accepted choice of a run that did not commit", () => {
    const run = makeRun({
      runId: RUN_ID,
      state: "closed_without_change",
      outcome: "safety_blocked",
      proposals: [
        makeProposal({
          state: "decided",
          decision: {
            decisionId: FACT_1,
            decision: "accept",
            contentFingerprint: "a".repeat(64),
            receiptId: FACT_1,
            decidedAt: "2026-10-13T09:00:00+08:00",
          },
        }),
      ],
    });
    const entries = projectDiagnosisHistory(makeDiagnosis(), [run]);
    expect(entries.map((entry) => entry.outcome)).toEqual(["stopped"]);
  });
});

describe("Safety / Restricted", () => {
  it("shows only active Go restrictions, urgent first, without lifting anything", () => {
    const base = {
      user_id: FACT_1,
      safety_screen_id: FACT_1,
      policy_revision: "safety-screen-restriction-v1" as const,
      plan_id: null,
      exercise_ids: [],
      lift_conditions: [],
      created_revision: 1,
      valid_until: null,
      resolution: null,
      clearance_available: false,
    };
    const views = projectSafetyRestrictions([
      {
        ...base,
        id: FACT_1,
        concept: "numbness",
        risk_level: "restricted",
        scope_kind: "all_training",
        lift_policy: "professional_clearance",
        reason: "安全筛查确认“麻木”",
        valid_from: "2026-10-13T09:00:00+08:00",
        active: true,
      },
      {
        ...base,
        id: FACT_2,
        concept: "dizziness",
        risk_level: "urgent",
        scope_kind: "all_training",
        lift_policy: "professional_clearance",
        reason: "突发剧烈头痛",
        valid_from: "2026-10-12T09:00:00+08:00",
        active: true,
      },
      {
        ...base,
        id: CONDITION_ID,
        concept: "trauma",
        risk_level: "restricted",
        scope_kind: "all_training",
        lift_policy: "on_expiry",
        reason: "已解除",
        valid_from: "2026-10-01T09:00:00+08:00",
        active: false,
      },
    ]);
    expect(views.map((item) => [item.id, item.statusLabel])).toEqual([
      [FACT_2, "建议立即就医"],
      [FACT_1, "暂不能执行"],
    ]);
    expect(views[1].meta).toBe("麻木 · 10月13日 · 需要复核");
    expect(views[1].liftPolicyLabel).toBe("需要医生或治疗师确认可以恢复训练");
  });
});
