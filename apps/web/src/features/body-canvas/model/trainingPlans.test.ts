import { describe, expect, it } from "vitest";
import {
  diffPlanContent,
  formatPrescription,
  projectGate,
  projectTrainingWindow,
  type ProjectTrainingWindowInput,
} from "./trainingPlans";
import {
  readSelectedPlanId,
  resolveSelectedPlanId,
  selectTrainingPlan,
  writeSelectedPlanId,
} from "./trainingPlanSelection";
import {
  CREATE_PROPOSAL_ID,
  NEXT_REVISION_ID,
  NOW,
  PLAN_ID,
  PROPOSAL_ID,
  REVISION_ID,
  exercise,
  makeContent,
  makePlan,
  makeProposal,
  updatedContent,
} from "./__fixtures__/trainingPlanFixtures";

function project(overrides: Partial<ProjectTrainingWindowInput> = {}) {
  return projectTrainingWindow({
    plans: [makePlan()],
    proposals: [],
    selectedPlanId: null,
    selectedTemplateId: null,
    focus: { kind: "plan" },
    loading: false,
    error: null,
    now: NOW,
    ...overrides,
  });
}

const receipt = {
  receiptId: "66666666-6666-4666-8666-666666666666",
  proposalId: PROPOSAL_ID,
  decisionId: "77777777-7777-4777-8777-777777777777",
  planId: PLAN_ID,
  revisionId: NEXT_REVISION_ID,
  idempotencyKey: "k",
  decidedAt: "2026-10-18T12:00:00+08:00",
  executionGate: makePlan().executionGate,
};

describe("TrainingPlan window view model", () => {
  it("formats prescriptions exactly, never guessing a missing target", () => {
    expect(
      formatPrescription(exercise({ planExerciseId: "a", nameSnapshot: "a" })),
    ).toBe("3 组 × 12 次");
    expect(
      formatPrescription(
        exercise({
          planExerciseId: "a",
          nameSnapshot: "a",
          target: { kind: "repetitions", min: 8, max: 12 },
        }),
      ),
    ).toBe("3 组 × 8–12 次");
    expect(
      formatPrescription(
        exercise({
          planExerciseId: "a",
          nameSnapshot: "a",
          sets: 1,
          target: { kind: "duration", seconds: 30 },
        }),
      ),
    ).toBe("1 组 × 30 秒");
    expect(
      formatPrescription(
        exercise({
          planExerciseId: "a",
          nameSnapshot: "a",
          target: { kind: "distance" },
        }),
      ),
    ).toBe("3 组");
  });

  it("projects the Accepted board: status chips, real stages and the selected template", () => {
    const view = project();
    expect(view.scene).toBe("plan");
    const plan = view.selected!;
    expect(plan).toMatchObject({
      planId: PLAN_ID,
      statusLabel: "启用中",
      frequencyLabel: "每周 3 次",
      scheduleLabel: "仅频率",
      canStart: true,
    });
    expect(plan.selectedTemplate!.title).toBe("A · 基础课");
    expect(plan.selectedTemplate!.stages.map((stage) => stage.label)).toEqual([
      "热身",
      "主训练",
      "放松",
    ]);
    expect(plan.selectedTemplate!.stages[1].exercises[2]).toMatchObject({
      name: "弹力带侧走",
      prescription: "2 组 × 12 次",
      rest: "休息 45 秒",
      side: "左右交替",
      change: null,
    });
    expect(
      project({ selectedTemplateId: "b" }).selected!.selectedTemplate!.title,
    ).toBe("B · 进阶课");
    expect(
      project({ selectedTemplateId: "ghost" }).selected!.selectedTemplate!
        .templateId,
    ).toBe("a");
  });

  it("shows the real empty state and never a demo plan", () => {
    expect(project({ plans: [] }).scene).toBe("empty");
    expect(project({ plans: undefined, loading: true }).scene).toBe("loading");
    expect(project({ error: "x" }).scene).toBe("error");
  });

  it("honours a persisted plan only when it still exists for this user", () => {
    const plans = [
      makePlan({ planId: "p1", status: "paused" }),
      makePlan({ planId: "p2" }),
    ];
    expect(resolveSelectedPlanId(plans, "p1")).toBe("p1");
    expect(resolveSelectedPlanId(plans, "ghost")).toBe("p2");
    expect(resolveSelectedPlanId([], "p1")).toBeNull();
    expect(project({ selectedPlanId: "ghost" }).selectedPlanMissing).toBe(true);
    expect(selectTrainingPlan(plans, null).selected).toMatchObject({
      planId: "p2",
      title: "日常活动计划",
      status: "active",
      gatePermits: true,
    });
  });

  it("only lets an active plan with a permitting gate start", () => {
    expect(
      project({ plans: [makePlan({ status: "paused" })] }).selected!.canStart,
    ).toBe(false);
    const blocked = project({
      plans: [
        makePlan({
          executionGate: {
            ...makePlan().executionGate,
            result: "block",
            permitsExecution: false,
            reasons: ["restricted_restriction"],
          },
        }),
      ],
    }).selected!;
    expect(blocked.canStart).toBe(false);
    expect(blocked.gate).toMatchObject({
      tone: "bad",
      label: "暂不能开始",
      reasons: ["有限制训练的安全提醒"],
    });
  });

  it("maps every TrainerGate result to an explicit, fail-closed view", () => {
    const base = makePlan().executionGate;
    expect(projectGate(base)).toMatchObject({
      permits: true,
      tone: "ok",
      label: "可以开始",
      reasons: [],
    });
    expect(
      projectGate({
        ...base,
        result: "warn",
        reasons: ["advisory_restriction"],
      }),
    ).toMatchObject({
      permits: true,
      tone: "warn",
      reasons: ["有需要留意的安全提醒"],
    });
    expect(
      projectGate({
        ...base,
        result: "unavailable",
        permitsExecution: false,
        reasons: [],
      }),
    ).toMatchObject({
      permits: false,
      reasons: ["安全检查未通过"],
    });
    // A contradictory projection never permits.
    expect(
      projectGate({ ...base, result: "block", permitsExecution: true }).permits,
    ).toBe(false);
    expect(
      projectGate({
        ...base,
        result: "block",
        permitsExecution: false,
        urgentCare: true,
        reasons: ["urgent_restriction"],
      }).reasons,
    ).toEqual(["有需要尽快就医的安全提醒"]);
  });

  it("diffs a proposal against its base revision by stable planExerciseId", () => {
    expect(diffPlanContent(makeContent(), updatedContent())).toEqual([
      {
        kind: "modified",
        planExerciseId: "bridge",
        name: "臀桥",
        before: "3 组 × 12 次 · 休息 45 秒",
        after: "3 组 × 15 次 · 休息 45 秒",
      },
      {
        kind: "added",
        planExerciseId: "dead-bug",
        name: "死虫式",
        before: null,
        after: "2 组 × 10 次 · 休息 30 秒",
      },
    ]);
    const removed = makeContent([
      exercise({
        planExerciseId: "bridge",
        nameSnapshot: "臀桥",
        restSeconds: 45,
      }),
    ]);
    expect(
      diffPlanContent(makeContent(), removed).map((change) => [
        change.kind,
        change.planExerciseId,
      ]),
    ).toEqual([
      ["removed", "wall-sit"],
      ["removed", "band-walk"],
    ]);
  });

  it("surfaces a pending update with its TTL and opens the Proposal board", () => {
    const plan = project({ proposals: [makeProposal()] });
    expect(plan.selected!.pendingProposals).toHaveLength(1);
    expect(plan.selected!.pendingProposals[0]).toMatchObject({
      expiresInDays: 6,
      byAgent: true,
    });
    const opened = project({
      proposals: [makeProposal()],
      focus: { kind: "proposal", proposalId: PROPOSAL_ID },
    });
    expect(opened.scene).toBe("proposal");
    expect(opened.proposal!.changes).toHaveLength(2);
  });

  it("treats a timed-out, re-based or refused proposal as Expired without touching the plan", () => {
    const focus = { kind: "proposal" as const, proposalId: PROPOSAL_ID };
    const timedOut = project({
      proposals: [makeProposal({ expiresAt: "2026-10-17T12:00:00+08:00" })],
      focus,
    });
    expect(timedOut.scene).toBe("expired");
    expect(timedOut.proposal!.expiredReason).toBe("timeout");
    expect(timedOut.selected!.pendingProposals).toHaveLength(0);

    const rebased = project({
      proposals: [makeProposal({ basedOnRevisionId: NEXT_REVISION_ID })],
      focus,
    });
    expect(rebased.proposal).toMatchObject({
      phase: "expired",
      expiredReason: "base_revision_changed",
    });

    const refused = project({
      proposals: [makeProposal()],
      focus,
      refused: { [PROPOSAL_ID]: "relevant_context_changed" },
    });
    expect(refused.proposal).toMatchObject({
      phase: "expired",
      expiredReason: "relevant_context_changed",
    });

    const server = project({
      proposals: [makeProposal({ state: "expired", expiredReason: "timeout" })],
      focus,
    });
    expect(server.scene).toBe("expired");
    expect(server.selected!.revisionId).toBe(REVISION_ID);
  });

  it("shows a pending create proposal as the window content when there is no plan yet", () => {
    const create = makeProposal({
      proposalId: CREATE_PROPOSAL_ID,
      kind: "create",
      planId: undefined,
      basedOnRevisionId: undefined,
    });
    const view = project({ plans: [], proposals: [create] });
    expect(view.scene).toBe("proposal");
    expect(
      view.proposal!.changes.every((change) => change.kind === "added"),
    ).toBe(true);
  });

  it("updates only after the record carries the receipt's revision", () => {
    const focus = {
      kind: "updated" as const,
      receipt,
      baseContent: makeContent(),
      acceptedContent: updatedContent(),
      created: false,
    };
    const waiting = project({ focus });
    expect(waiting.scene).toBe("updating");
    expect(waiting.selected).toBeNull();

    const updated = project({
      plans: [
        makePlan({
          currentAcceptedRevisionId: NEXT_REVISION_ID,
          currentContent: updatedContent(),
        }),
      ],
      focus,
    });
    expect(updated.scene).toBe("updated");
    const main = updated.selected!.selectedTemplate!.stages[1].exercises;
    expect(main.map((item) => [item.name, item.change])).toEqual([
      ["臀桥", "modified"],
      ["死虫式", "added"],
      ["靠墙静蹲", null],
      ["弹力带侧走", null],
    ]);
  });
});

describe("selected plan persistence", () => {
  it("round-trips through storage and tolerates blocked storage", () => {
    const store = new Map<string, string>();
    const storage = {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key),
    };
    writeSelectedPlanId("plan-1", storage);
    expect(readSelectedPlanId(storage)).toBe("plan-1");
    writeSelectedPlanId(null, storage);
    expect(readSelectedPlanId(storage)).toBeNull();
    const blocked = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
    };
    expect(readSelectedPlanId(blocked)).toBeNull();
    expect(() => writeSelectedPlanId("x", blocked)).not.toThrow();
  });
});
