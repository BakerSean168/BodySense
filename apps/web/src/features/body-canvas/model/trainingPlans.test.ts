import { describe, expect, it } from "vitest";
import {
  formatDosage,
  projectTrainingPlans,
  readSelectedPlanId,
  resolveSelectedPlanId,
  SELECTED_PLAN_STORAGE_KEY,
  trainingInterventions,
  writeSelectedPlanId,
} from "./trainingPlans";
import { makeWorkspace } from "./__fixtures__/diagnosisFixtures";
import type { TrainingPlan } from "@/features/training/services/trainingService";
import type {
  Intervention,
  TreatmentRevision,
} from "@/features/workspace/types/workspace";

function intervention(overrides: Partial<Intervention> = {}): Intervention {
  return {
    id: "int-1",
    treatment_id: "treatment-1",
    treatment_revision_id: "rev-1",
    kind: "exercise",
    title: "靠墙肩胛滑动",
    description: "缓慢控制",
    prescription: { sets: 3, reps: 10, rest: 60 },
    position: 0,
    status: "active",
    created_at: "2026-10-11T00:00:00Z",
    updated_at: "2026-10-11T00:00:00Z",
    ...overrides,
  };
}

function revision(overrides: Partial<TreatmentRevision> = {}): TreatmentRevision {
  return {
    id: "rev-1",
    treatment_id: "treatment-1",
    revision: 1,
    acceptance_state: "accepted",
    lifecycle_state: "active",
    source_body_state_revision: 1,
    source_diagnosis_analysis_id: "analysis-1",
    goal: "肩胛稳定训练",
    duration_weeks: 6,
    plan: {
      summary: "渐进式肩胛控制",
      goal: "肩胛稳定训练",
      duration_weeks: 6,
      interventions: [],
      daily_habits: ["每周 3 次"],
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
    interventions: [],
    ...overrides,
  };
}

function plan(overrides: Partial<TrainingPlan> = {}): TrainingPlan {
  return {
    id: "plan-1",
    status: "active",
    goal: "肩胛稳定训练",
    duration_weeks: 6,
    current_week: 1,
    phases: [],
    created_at: "2026-10-11T00:00:00Z",
    ...overrides,
  };
}

describe("training plan projection uses real plan identity", () => {
  it("has no plan when the user has no plans (empty, not a demo plan)", () => {
    const result = projectTrainingPlans(makeWorkspace(), [], null);
    expect(result.selected).toBeNull();
    expect(result.plans).toEqual([]);
  });

  it("honours a persisted planId that still exists", () => {
    const plans = [plan({ id: "plan-1" }), plan({ id: "plan-2" })];
    expect(resolveSelectedPlanId(plans, "plan-2")).toBe("plan-2");
  });

  it("falls back to an active plan when the persisted id is stale", () => {
    const plans = [
      plan({ id: "plan-1", status: "superseded" }),
      plan({ id: "plan-2", status: "active" }),
    ];
    expect(resolveSelectedPlanId(plans, "deleted-plan")).toBe("plan-2");
    expect(resolveSelectedPlanId(plans, null)).toBe("plan-2");
    expect(resolveSelectedPlanId(plans, undefined)).toBe("plan-2");
  });

  it("flags a missing selected plan instead of silently substituting content", () => {
    const result = projectTrainingPlans(makeWorkspace(), [], "ghost-plan");
    expect(result.selected).toBeNull();
    expect(result.selectedPlanMissing).toBe(false);
  });

  it("lists every real plan with its own title", () => {
    const workspace = makeWorkspace({
      treatment_revisions: [
        revision({ id: "rev-1", goal: "肩胛稳定训练" }),
        revision({ id: "rev-2", goal: "臀腿力量训练" }),
      ],
    });
    const plans = [
      plan({ id: "plan-1", treatment_revision_id: "rev-1" }),
      plan({ id: "plan-2", treatment_revision_id: "rev-2" }),
    ];
    const result = projectTrainingPlans(workspace, plans, "plan-2");
    expect(result.plans.map((item) => item.title)).toEqual([
      "肩胛稳定训练",
      "臀腿力量训练",
    ]);
    expect(result.selected?.planId).toBe("plan-2");
    expect(result.selected?.title).toBe("臀腿力量训练");
  });

  it("groups real intervention kinds into stages and keeps exercise identity", () => {
    const workspace = makeWorkspace({
      treatment_revisions: [
        revision({
          interventions: [
            intervention({ id: "warm", kind: "mobility", title: "胸椎活动", position: 0 }),
            intervention({ id: "main", kind: "exercise", title: "肩胛滑动", position: 1 }),
            intervention({ id: "check", kind: "self_test", title: "抬臂自测", position: 2 }),
            intervention({ id: "habit", kind: "habit", title: "日常姿势提醒", position: 3 }),
          ],
        }),
      ],
    });
    const plans = [plan({ treatment_revision_id: "rev-1" })];
    const result = projectTrainingPlans(workspace, plans, null)!;
    const selected = result.selected!;
    expect(selected.stages.map((stage) => stage.phase)).toEqual([
      "mobility",
      "main",
      "self_check",
    ]);
    expect(selected.stages.map((stage) => stage.exercises[0].exerciseId)).toEqual([
      "warm",
      "main",
      "check",
    ]);
    // Non-training interventions are not presented as plan exercises.
    expect(selected.exerciseCount).toBe(3);
    expect(selected.stageStructureUnavailable).toBe(true);
  });

  it("does not invent dosage when the prescription omits it", () => {
    const workspace = makeWorkspace({
      treatment_revisions: [
        revision({
          interventions: [intervention({ id: "x", prescription: {} })],
        }),
      ],
    });
    const selected = projectTrainingPlans(workspace, [plan({ treatment_revision_id: "rev-1" })], null)
      .selected!;
    const exercise = selected.stages[0].exercises[0];
    expect(exercise.sets).toBeNull();
    expect(exercise.reps).toBeNull();
    expect(formatDosage(exercise)).toBe("剂量未提供");
  });

  it("formats sets, reps and rest from real prescription values", () => {
    const workspace = makeWorkspace({
      treatment_revisions: [
        revision({ interventions: [intervention({ id: "x" })] }),
      ],
    });
    const selected = projectTrainingPlans(workspace, [plan({ treatment_revision_id: "rev-1" })], null)
      .selected!;
    expect(formatDosage(selected.stages[0].exercises[0])).toBe("3 组 · 10 次 · 休息 60 秒");
  });

  it("falls back to the execution projection when interventions are unavailable", () => {
    const workspace = makeWorkspace();
    const projectedExercises = [
      {
        intervention_id: "int-9",
        name: "靠墙肩胛滑动",
        description: "缓慢控制",
        sets: "3",
        reps: "10",
      },
    ];
    const plans = [
      plan({
        phases: [
          {
            week: 1,
            focus: "肩胛稳定训练",
            exercises: projectedExercises,
          },
        ] as TrainingPlan["phases"],
      }),
    ];
    const selected = projectTrainingPlans(workspace, plans, null).selected!;
    expect(selected.stages[0].exercises[0].exerciseId).toBe("int-9");
    expect(selected.frequency).toBeNull();
  });

  it("reports plan frequency from the accepted prescription when present", () => {
    const workspace = makeWorkspace({
      treatment_revisions: [
        revision({
          interventions: [
            intervention({ id: "x", prescription: { sets: 3, frequency: "每周 3 次" } }),
          ],
        }),
      ],
    });
    const selected = projectTrainingPlans(workspace, [plan({ treatment_revision_id: "rev-1" })], null)
      .selected!;
    expect(selected.frequency).toBe("每周 3 次");
  });

  it("marks only the plan matching the accepted revision or projection", () => {
    const workspace = makeWorkspace({
      training_plan: {
        id: "plan-1",
        status: "active",
        goal: "肩胛稳定训练",
        duration_weeks: 6,
        current_week: 1,
        phases: [],
        created_at: "2026-10-11T00:00:00Z",
      },
    });
    const result = projectTrainingPlans(
      workspace,
      [plan({ id: "plan-1" }), plan({ id: "plan-2" })],
      null,
    );
    expect(result.plans.filter((item) => item.isAccepted).map((item) => item.planId)).toEqual([
      "plan-1",
    ]);
  });

  it("filters non-training kinds out of the exercise list", () => {
    const interventions = trainingInterventions(
      revision({
        interventions: [
          intervention({ id: "a", kind: "education" }),
          intervention({ id: "b", kind: "monitoring" }),
          intervention({ id: "c", kind: "exercise" }),
        ],
      }),
    );
    expect(interventions.map((item) => item.id)).toEqual(["c"]);
  });

  it("sorts interventions by their real position", () => {
    const interventions = trainingInterventions(
      revision({
        interventions: [
          intervention({ id: "second", position: 5 }),
          intervention({ id: "first", position: 1 }),
        ],
      }),
    );
    expect(interventions.map((item) => item.id)).toEqual(["first", "second"]);
  });
});

describe("selected plan persistence", () => {
  function memoryStorage() {
    const store = new Map<string, string>();
    return {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key),
      has: (key: string) => store.has(key),
    };
  }

  it("round-trips the last opened plan", () => {
    const storage = memoryStorage();
    writeSelectedPlanId("plan-7", storage);
    expect(storage.has(SELECTED_PLAN_STORAGE_KEY)).toBe(true);
    expect(readSelectedPlanId(storage)).toBe("plan-7");
  });

  it("clears the stored plan when asked", () => {
    const storage = memoryStorage();
    writeSelectedPlanId("plan-7", storage);
    writeSelectedPlanId(null, storage);
    expect(readSelectedPlanId(storage)).toBeNull();
  });

  it("returns null for blank or throwing storage", () => {
    expect(readSelectedPlanId({ getItem: () => "   " })).toBeNull();
    expect(
      readSelectedPlanId({
        getItem: () => {
          throw new Error("blocked");
        },
      }),
    ).toBeNull();
  });
});
